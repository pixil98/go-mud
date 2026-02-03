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

// Subscriber provides the ability to subscribe to message subjects
type Subscriber interface {
	Subscribe(subject string, handler func(data []byte)) (unsubscribe func(), err error)
}

type PlayerManager struct {
	cmdHandler    *commands.Handler
	pluginManager *plugins.PluginManager
	subscriber    Subscriber
	world         *game.WorldState

	loginFlow *loginFlow
}

func NewPlayerManager(cmd *commands.Handler, plugins *plugins.PluginManager, subscriber Subscriber, world *game.WorldState) *PlayerManager {
	pm := &PlayerManager{
		pluginManager: plugins,
		cmdHandler:    cmd,
		subscriber:    subscriber,
		world:         world,
		loginFlow:     &loginFlow{world: world},
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
	m.world.ForEachPlayer(func(p game.PlayerState) {
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

	// Register player in world state
	// TODO: Get starting zone/room from config or character data
	startZone := storage.Identifier("default")
	startRoom := storage.Identifier("default")
	err = m.world.AddPlayer(charId, startZone, startRoom)
	if err != nil {
		return nil, fmt.Errorf("registering player in world: %w", err)
	}

	p := &Player{
		conn:       conn,
		charId:     charId,
		world:      m.world,
		cmdHandler: m.cmdHandler,
		subscriber: m.subscriber,
		subs:       make(map[string]func()),
		msgs:       make(chan []byte, 100),
	}

	// Subscribe to player-specific channel
	subject := fmt.Sprintf("player-%s", p.Id())
	err = p.Subscribe("player", subject)
	if err != nil {
		// Clean up world state on failure
		_ = m.world.RemovePlayer(charId)
		return nil, fmt.Errorf("subscribing to player channel: %w", err)
	}

	return p, nil
}
