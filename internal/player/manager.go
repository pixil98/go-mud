package player

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/plugins"
	"github.com/pixil98/go-mud/internal/storage"
)

// Subscriber provides the ability to subscribe to message subjects
type Subscriber interface {
	Subscribe(subject string, handler func(data []byte)) (unsubscribe func(), err error)
}

type PlayerManager struct {
	players       map[string]*Player
	cmdHandler    *commands.Handler
	pluginManager *plugins.PluginManager
	subscriber    Subscriber

	loginFlow *loginFlow

	chars storage.Storer[*Character]
}

func NewPlayerManager(cmd *commands.Handler, plugins *plugins.PluginManager, cs storage.Storer[*Character], subscriber Subscriber) *PlayerManager {
	pm := &PlayerManager{
		players:       map[string]*Player{},
		pluginManager: plugins,
		cmdHandler:    cmd,
		subscriber:    subscriber,
		loginFlow:     &loginFlow{cStore: cs},
		chars:         cs,
	}

	return pm
}

func (m *PlayerManager) Start(ctx context.Context) error {
	<-ctx.Done()

	//TODO stop all player connections
	return nil
}

func (m *PlayerManager) Tick(ctx context.Context) error {
	for _, p := range m.players {
		p.Tick(ctx)
	}

	return nil
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
	err = m.chars.Save(strings.ToLower(char.Name), char)
	if err != nil {
		return nil, fmt.Errorf("saving character: %w", err)
	}

	p := &Player{
		conn:       conn,
		char:       char,
		cmdHandler: m.cmdHandler,
		subscriber: m.subscriber,
		subs:       make(map[string]func()),
		msgs:       make(chan []byte, 100),
	}

	// Subscribe to player-specific channel
	subject := fmt.Sprintf("player-%s", strings.ToLower(char.Name))
	err = p.Subscribe("player", subject)
	if err != nil {
		return nil, fmt.Errorf("subscribing to player channel: %w", err)
	}

	return p, nil
}
