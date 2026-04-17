package player

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

const (
	// DefaultLinklessTimeout is how long a disconnected player remains in the world before being saved and removed.
	DefaultLinklessTimeout = 5 * time.Minute
	// DefaultIdleTimeout is how long a connected but inactive player is allowed before being kicked.
	DefaultIdleTimeout     = 15 * time.Minute
)

// PlayerManagerOpt configures optional PlayerManager settings.
type PlayerManagerOpt func(*PlayerManager)

// WithLinklessTimeout sets how long a linkless player remains in the world before being removed.
func WithLinklessTimeout(d time.Duration) PlayerManagerOpt {
	return func(pm *PlayerManager) { pm.linklessTimeout = d }
}

// WithIdleTimeout sets how long an idle connected player can remain before being kicked.
func WithIdleTimeout(d time.Duration) PlayerManagerOpt {
	return func(pm *PlayerManager) { pm.idleTimeout = d }
}

// PlayerManager manages player sessions, lifecycle, and timeouts.
type PlayerManager struct {
	cmdHandler *commands.Handler
	world      *game.WorldState
	dict       *game.Dictionary

	loginFlow   *loginFlow
	defaultZone string
	defaultRoom string

	pronouns *storage.SelectableStorer[*assets.Pronoun]
	races    *storage.SelectableStorer[*assets.Race]

	linklessTimeout time.Duration
	idleTimeout     time.Duration
}

// NewPlayerManager creates a PlayerManager with the given command handler, world state, and asset dictionary.
func NewPlayerManager(cmd *commands.Handler, world *game.WorldState, dict *game.Dictionary, defaultZone, defaultRoom string, opts ...PlayerManagerOpt) *PlayerManager {
	pm := &PlayerManager{
		cmdHandler:      cmd,
		world:           world,
		dict:            dict,
		loginFlow:       &loginFlow{chars: dict.Characters},
		defaultZone:     defaultZone,
		defaultRoom:     defaultRoom,
		linklessTimeout: DefaultLinklessTimeout,
		idleTimeout:     DefaultIdleTimeout,
	}

	pm.races = storage.NewSelectableStorer(dict.Races)
	pm.pronouns = storage.NewSelectableStorer(dict.Pronouns)

	for _, opt := range opts {
		opt(pm)
	}

	return pm
}

// Tick checks all players for linkless and idle timeouts.
// Linkless players past the timeout are saved and removed from the world.
// Idle connected players are marked linkless and kicked, dropping their connection.
func (m *PlayerManager) Tick(_ context.Context) error {
	now := time.Now()
	var linklessExpired []string
	var idleExpired []string

	m.world.ForEachPlayer(func(charId string, ps *game.CharacterInstance) {
		if ps.IsLinkless() {
			if now.Sub(ps.LinklessAt()) >= m.linklessTimeout {
				linklessExpired = append(linklessExpired, charId)
			}
		} else if now.Sub(ps.LastActivity()) >= m.idleTimeout {
			idleExpired = append(idleExpired, charId)
		}
	})

	for _, charId := range linklessExpired {
		ps := m.world.GetPlayer(charId)
		if ps == nil {
			continue
		}
		if err := ps.SaveCharacter(m.dict.Characters); err != nil {
			slog.Warn("failed to save linkless player", "charId", charId, "error", err)
		}
		ps.UnsubscribeAll()
		if err := m.world.RemovePlayer(charId); err != nil {
			slog.Warn("failed to remove linkless player", "charId", charId, "error", err)
		}
		slog.Info("linkless player removed", "charId", charId)
	}

	for _, charId := range idleExpired {
		ps := m.world.GetPlayer(charId)
		if ps == nil {
			continue
		}
		// Save happens in handleSessionEnd after the kick triggers Play() to exit.
		ps.MarkLinkless()
		ps.Kick()
		slog.Info("idle player kicked", "charId", charId)
	}

	return nil
}

// RunSession handles the full player lifecycle: login, play, and cleanup.
func (m *PlayerManager) RunSession(ctx context.Context, conn io.ReadWriter) error {
	p, err := m.newPlayer(conn)
	if err != nil {
		_, _ = conn.Write([]byte("Failed to setup player session.\n"))
		return err
	}

	playErr := p.Play(ctx)
	m.handleSessionEnd(p.Id(), playErr)
	return playErr
}

func (m *PlayerManager) newPlayer(conn io.ReadWriter) (*Player, error) {
	char, err := m.loginFlow.Run(conn)
	if err != nil {
		return nil, err
	}

	err = m.initCharacter(conn, char)
	if err != nil {
		return nil, fmt.Errorf("initializing character: %w", err)
	}

	// Resolve foreign keys on the character
	if err := char.Resolve(m.dict.Pronouns, m.dict.Races, m.dict.Objects); err != nil {
		return nil, fmt.Errorf("resolving character references: %w", err)
	}

	// Save the character back to preserve changes
	err = m.dict.Characters.Save(strings.ToLower(char.Name), char)
	if err != nil {
		return nil, fmt.Errorf("saving character: %w", err)
	}

	charId := strings.ToLower(char.Name)
	msgs := make(chan []byte, 100)
	if ps := m.world.GetPlayer(charId); ps != nil && !ps.IsQuit() {
		// Player already in world with an active session — kick old connection and reattach.
		slog.Info("player reconnecting", "charId", charId)
		ps.Kick()
		time.Sleep(10 * time.Millisecond)
		ps.Reattach(msgs)
		_, _ = conn.Write([]byte("Reconnecting...\n"))
	} else {
		// Fresh login
		charRef := storage.NewSmartIdentifier[*assets.Character](charId)
		if err := charRef.Resolve(m.dict.Characters); err != nil {
			return nil, fmt.Errorf("resolving character %q: %w", charId, err)
		}
		room := m.startingRoom(charRef.Get())
		ci, err := game.NewCharacterInstance(charRef, msgs, room)
		if err != nil {
			return nil, fmt.Errorf("creating character instance: %w", err)
		}
		if err = m.world.AddPlayer(ci); err != nil {
			return nil, fmt.Errorf("registering player in world: %w", err)
		}
	}

	ps := m.world.GetPlayer(charId)

	p := &Player{
		conn:       conn,
		charId:     charId,
		world:      m.world,
		cmdHandler: m.cmdHandler,
		msgs:       msgs,
		done:       ps.Done(),
	}

	if err := m.subscribePlayer(ps, charId); err != nil {
		if removeErr := m.world.RemovePlayer(charId); removeErr != nil {
			slog.Warn("failed to remove player after subscribe failure", "charId", charId, "error", removeErr)
		}
		return nil, fmt.Errorf("subscribing player: %w", err)
	}

	return p, nil
}

// HandleSessionEnd handles a player's Play() loop ending. It examines the error and
// player state to determine whether to remove the player, mark them linkless, or do nothing.
func (m *PlayerManager) handleSessionEnd(charId string, playErr error) {
	if errors.Is(playErr, game.ErrPlayerReconnected) {
		return
	}

	ps := m.world.GetPlayer(charId)
	if ps == nil {
		return
	}

	if err := ps.SaveCharacter(m.dict.Characters); err != nil {
		slog.Warn("failed to save player on session end", "charId", charId, "error", err)
	}

	if ps.IsQuit() {
		ps.UnsubscribeAll()
		if err := m.world.RemovePlayer(charId); err != nil {
			slog.Warn("failed to remove player on quit", "charId", charId, "error", err)
		}
		slog.Info("player quit", "charId", charId)
	} else {
		ps.MarkLinkless()
		slog.Info("player went linkless", "charId", charId)
	}
}

// subscribePlayer subscribes the player to their individual NATS channel.
func (m *PlayerManager) subscribePlayer(ps *game.CharacterInstance, charId string) error {
	if err := ps.Subscribe(fmt.Sprintf("player-%s", charId)); err != nil {
		return fmt.Errorf("subscribing to player channel: %w", err)
	}
	return nil
}

// initCharacter prompts for any missing traits on a character and initializes HP for new characters.
func (m *PlayerManager) initCharacter(rw io.ReadWriter, char *assets.Character) error {
	for char.Pronoun.Id() == "" {
		sel, err := m.pronouns.Prompt(rw, "What are your pronouns?")
		if err != nil {
			return fmt.Errorf("selecting pronouns: %w", err)
		}
		char.Pronoun = storage.NewSmartIdentifier[*assets.Pronoun](sel)
	}

	for char.Race.Id() == "" {
		sel, err := m.races.Prompt(rw, "What is your race?")
		if err != nil {
			return fmt.Errorf("selecting race: %w", err)
		}
		char.Race = storage.NewSmartIdentifier[*assets.Race](sel)
	}

	if char.BaseStats == nil {
		char.BaseStats = assets.DefaultBaseStats()
	}

	// Level 0 means brand new character — set to level 1. Resource pools
	// (HP, etc.) are computed from perks when NewCharacterInstance runs.
	if char.Level == 0 {
		char.Level = 1
	}

	return nil
}

// startingRoom returns the room instance a character should start in.
// Uses saved location if still valid, otherwise falls back to defaults.
func (m *PlayerManager) startingRoom(char *assets.Character) *game.RoomInstance {
	defaultRoom := m.world.GetZone(m.defaultZone).GetRoom(m.defaultRoom)

	if char.LastZone == "" || char.LastRoom == "" {
		return defaultRoom
	}

	zi := m.world.GetZone(char.LastZone)
	if zi == nil {
		slog.Warn("saved zone not found, using default", "char", char.Name, "zone", char.LastZone)
		return defaultRoom
	}

	ri := zi.GetRoom(char.LastRoom)
	if ri == nil {
		slog.Warn("saved room not found, using default", "char", char.Name, "room", char.LastRoom)
		return defaultRoom
	}

	return ri
}
