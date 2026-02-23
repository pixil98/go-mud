package player

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

const (
	DefaultLinklessTimeout = 5 * time.Minute
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

type PlayerManager struct {
	cmdHandler *commands.Handler
	world      *game.WorldState
	dict       *game.Dictionary

	loginFlow   *loginFlow
	defaultZone storage.Identifier
	defaultRoom storage.Identifier

	pronouns *storage.SelectableStorer[*game.Pronoun]
	races    *storage.SelectableStorer[*game.Race]

	linklessTimeout time.Duration
	idleTimeout     time.Duration
}

func NewPlayerManager(cmd *commands.Handler, world *game.WorldState, dict *game.Dictionary, defaultZone, defaultRoom string, opts ...PlayerManagerOpt) *PlayerManager {
	pm := &PlayerManager{
		cmdHandler:      cmd,
		world:           world,
		dict:            dict,
		loginFlow:       &loginFlow{chars: dict.Characters},
		defaultZone:     storage.Identifier(defaultZone),
		defaultRoom:     storage.Identifier(defaultRoom),
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
func (m *PlayerManager) Tick(ctx context.Context) error {
	now := time.Now()
	var linklessExpired []storage.Identifier
	var idleExpired []storage.Identifier

	m.world.ForEachPlayer(func(charId storage.Identifier, ps game.PlayerState) {
		if ps.Linkless {
			if now.Sub(ps.LinklessAt) >= m.linklessTimeout {
				linklessExpired = append(linklessExpired, charId)
			}
		} else if now.Sub(ps.LastActivity) >= m.idleTimeout {
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
		_ = m.world.RemovePlayer(charId)
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
	if err := char.Resolve(m.dict); err != nil {
		return nil, fmt.Errorf("resolving character references: %w", err)
	}

	// Save the character back to preserve changes
	err = m.dict.Characters.Save(strings.ToLower(char.Name), char)
	if err != nil {
		return nil, fmt.Errorf("saving character: %w", err)
	}

	charId := storage.Identifier(strings.ToLower(char.Name))
	msgs := make(chan []byte, 100)
	var zoneId, roomId storage.Identifier

	if ps := m.world.GetPlayer(charId); ps != nil {
		// Player already in world — kick old connection and reattach
		slog.Info("player reconnecting", "charId", charId)
		ps.Kick()
		time.Sleep(10 * time.Millisecond)
		ps.Reattach(msgs)
		zoneId, roomId = ps.Location()
		_, _ = conn.Write([]byte("Reconnecting...\n"))
	} else {
		// Fresh login
		zoneId, roomId = m.startingLocation(char)
		err = m.world.AddPlayer(charId, m.dict.Characters.Get(charId.String()), msgs, zoneId, roomId)
		if err != nil {
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

	if err := m.subscribePlayer(ps, charId, zoneId, roomId); err != nil {
		_ = m.world.RemovePlayer(charId)
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

	id := storage.Identifier(charId)
	ps := m.world.GetPlayer(id)
	if ps == nil {
		return
	}

	if err := ps.SaveCharacter(m.dict.Characters); err != nil {
		slog.Warn("failed to save player on session end", "charId", charId, "error", err)
	}

	if ps.Quit {
		ps.UnsubscribeAll()
		_ = m.world.RemovePlayer(id)
		slog.Info("player quit", "charId", charId)
	} else {
		ps.MarkLinkless()
		slog.Info("player went linkless", "charId", charId)
	}
}

// subscribePlayer subscribes the player to the standard NATS channels.
func (m *PlayerManager) subscribePlayer(ps *game.PlayerState, charId storage.Identifier, zoneId, roomId storage.Identifier) error {
	if err := ps.Subscribe(fmt.Sprintf("player-%s", charId)); err != nil {
		return fmt.Errorf("subscribing to player channel: %w", err)
	}
	if err := ps.Subscribe("world"); err != nil {
		return fmt.Errorf("subscribing to world channel: %w", err)
	}
	if err := ps.Subscribe(fmt.Sprintf("zone-%s", zoneId)); err != nil {
		return fmt.Errorf("subscribing to zone channel: %w", err)
	}
	if err := ps.Subscribe(fmt.Sprintf("zone-%s-room-%s", zoneId, roomId)); err != nil {
		return fmt.Errorf("subscribing to room channel: %w", err)
	}
	return nil
}

// initCharacter prompts for any missing traits on a character.
func (m *PlayerManager) initCharacter(rw io.ReadWriter, char *game.Character) error {
	for char.Pronoun.Id() == "" {
		sel, err := m.pronouns.Prompt(rw, "What are your pronouns?")
		if err != nil {
			return fmt.Errorf("selecting pronouns: %w", err)
		}
		char.Pronoun = storage.NewSmartIdentifier[*game.Pronoun](string(sel))
	}

	for char.Race.Id() == "" {
		sel, err := m.races.Prompt(rw, "What is your race?")
		if err != nil {
			return fmt.Errorf("selecting race: %w", err)
		}
		char.Race = storage.NewSmartIdentifier[*game.Race](string(sel))
	}

	if char.Level == 0 {
		char.Level = 1
	}

	return nil
}

// startingLocation returns the zone and room a character should start in.
// Uses saved location if the zone and room are still valid, otherwise falls back to defaults.
func (m *PlayerManager) startingLocation(char *game.Character) (storage.Identifier, storage.Identifier) {
	if char.LastZone == "" || char.LastRoom == "" {
		return m.defaultZone, m.defaultRoom
	}

	// Verify zone exists
	if m.dict.Zones.Get(string(char.LastZone)) == nil {
		slog.Warn("saved zone not found, using default", "char", char.Name, "zone", char.LastZone)
		return m.defaultZone, m.defaultRoom
	}

	// Verify room exists and is in the saved zone
	room := m.dict.Rooms.Get(string(char.LastRoom))
	if room == nil {
		slog.Warn("saved room not found, using default", "char", char.Name, "room", char.LastRoom)
		return m.defaultZone, m.defaultRoom
	}
	if room.Zone.Id() != string(char.LastZone) {
		slog.Warn("saved room not in saved zone, using default", "char", char.Name, "zone", char.LastZone, "room", char.LastRoom, "room_zone", room.Zone.Id())
		return m.defaultZone, m.defaultRoom
	}

	return char.LastZone, char.LastRoom
}
