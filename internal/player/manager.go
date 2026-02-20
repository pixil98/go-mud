package player

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

type PlayerManager struct {
	cmdHandler *commands.Handler
	world      *game.WorldState
	dict       *game.Dictionary

	loginFlow   *loginFlow
	defaultZone storage.Identifier
	defaultRoom storage.Identifier

	pronouns *storage.SelectableStorer[*game.Pronoun]
	races    *storage.SelectableStorer[*game.Race]
}

func NewPlayerManager(cmd *commands.Handler, world *game.WorldState, dict *game.Dictionary, defaultZone, defaultRoom string) *PlayerManager {
	pm := &PlayerManager{
		cmdHandler:  cmd,
		world:       world,
		dict:        dict,
		loginFlow:   &loginFlow{chars: dict.Characters},
		defaultZone: storage.Identifier(defaultZone),
		defaultRoom: storage.Identifier(defaultRoom),
	}

	pm.races = storage.NewSelectableStorer(dict.Races)
	pm.pronouns = storage.NewSelectableStorer(dict.Pronouns)

	return pm
}

// RemovePlayer removes a player from the world state
func (m *PlayerManager) RemovePlayer(charId string) {
	_ = m.world.RemovePlayer(storage.Identifier(charId))
}

func (m *PlayerManager) NewPlayer(conn io.ReadWriter) (*Player, error) {
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

	p := &Player{
		conn:       conn,
		charId:     charId,
		world:      m.world,
		cmdHandler: m.cmdHandler,
		msgs:       make(chan []byte, 100),
	}

	// Determine starting location: use saved location if valid, otherwise defaults
	startZone, startRoom := m.startingLocation(char)

	// Register player in world state
	err = m.world.AddPlayer(charId, m.dict.Characters.Get(charId.String()), p.msgs, startZone, startRoom)
	if err != nil {
		return nil, fmt.Errorf("registering player in world: %w", err)
	}

	// Subscribe to player-specific channel
	err = p.world.GetPlayer(charId).Subscribe(fmt.Sprintf("player-%s", p.Id()))
	if err != nil {
		// Clean up world state on failure
		_ = m.world.RemovePlayer(charId)
		return nil, fmt.Errorf("subscribing to player channel: %w", err)
	}

	// Subscribe to world channel (for gossip, etc.)
	err = p.world.GetPlayer(charId).Subscribe("world")
	if err != nil {
		_ = m.world.RemovePlayer(charId)
		return nil, fmt.Errorf("subscribing to world channel: %w", err)
	}

	// Subscribe to zone channel
	err = p.world.GetPlayer(charId).Subscribe(fmt.Sprintf("zone-%s", startZone))
	if err != nil {
		_ = m.world.RemovePlayer(charId)
		return nil, fmt.Errorf("subscribing to zone channel: %w", err)
	}

	// Subscribe to room channel
	err = p.world.GetPlayer(charId).Subscribe(fmt.Sprintf("zone-%s-room-%s", startZone, startRoom))
	if err != nil {
		_ = m.world.RemovePlayer(charId)
		return nil, fmt.Errorf("subscribing to room channel: %w", err)
	}

	return p, nil
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
