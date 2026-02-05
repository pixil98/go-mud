package player

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/plugins"
	"github.com/pixil98/go-mud/internal/storage"
)

type PlayerManager struct {
	cmdHandler    *commands.Handler
	pluginManager *plugins.PluginManager
	world         *game.WorldState

	loginFlow   *loginFlow
	defaultZone storage.Identifier
	defaultRoom storage.Identifier
}

func NewPlayerManager(cmd *commands.Handler, plugins *plugins.PluginManager, world *game.WorldState, defaultZone, defaultRoom string) *PlayerManager {
	pm := &PlayerManager{
		pluginManager: plugins,
		cmdHandler:    cmd,
		world:         world,
		loginFlow:     &loginFlow{world: world},
		defaultZone:   storage.Identifier(defaultZone),
		defaultRoom:   storage.Identifier(defaultRoom),
	}

	return pm
}

func (m *PlayerManager) Start(ctx context.Context) error {
	<-ctx.Done()
	// Player connections are stopped via context cancellation from the telnet listener
	return nil
}

func (m *PlayerManager) Tick(ctx context.Context) error {
	// Iterate over all players in the world state
	// Tick logic (regen, effects, etc.) can be added to PlayerState later
	m.world.ForEachPlayer(func(id storage.Identifier, p game.PlayerState) {
		// Future: p.Tick() or similar
		_ = p // Placeholder until tick logic is implemented
	})
	return nil
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

	err = m.pluginManager.InitCharacter(conn, char)
	if err != nil {
		return nil, fmt.Errorf("initializing character: %w", err)
	}
	// Save the character back to preserve changes
	err = m.world.Characters().Save(strings.ToLower(char.Name()), char)
	if err != nil {
		return nil, fmt.Errorf("saving character: %w", err)
	}

	charId := storage.Identifier(strings.ToLower(char.Name()))

	p := &Player{
		conn:       conn,
		charId:     charId,
		world:      m.world,
		cmdHandler: m.cmdHandler,
		msgs:       make(chan []byte, 100),
	}

	// Register player in world state
	// TODO: Get starting zone/room from saved character data instead of config defaults
	err = m.world.AddPlayer(charId, p.msgs, m.defaultZone, m.defaultRoom)
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
	err = p.world.GetPlayer(charId).Subscribe(fmt.Sprintf("zone-%s", m.defaultZone))
	if err != nil {
		_ = m.world.RemovePlayer(charId)
		return nil, fmt.Errorf("subscribing to zone channel: %w", err)
	}

	// Subscribe to room channel
	err = p.world.GetPlayer(charId).Subscribe(fmt.Sprintf("zone-%s-room-%s", m.defaultZone, m.defaultRoom))
	if err != nil {
		_ = m.world.RemovePlayer(charId)
		return nil, fmt.Errorf("subscribing to room channel: %w", err)
	}

	return p, nil
}
