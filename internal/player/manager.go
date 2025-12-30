package player

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/plugins"
	"github.com/pixil98/go-mud/internal/storage"
)

type PlayerManager struct {
	players       map[string]*Player
	cmdHandler    *commands.Handler
	pluginManager *plugins.PluginManager

	loginFlow *loginFlow

	chars storage.Storer[*Character]
	//pronouns storage.Storer[*Pronoun]
	//races    storage.Storer[*Race]
}

func NewPlayerManager(cmd *commands.Handler, plugins *plugins.PluginManager, cs storage.Storer[*Character]) *PlayerManager {
	pm := &PlayerManager{
		players:       map[string]*Player{},
		pluginManager: plugins,
		cmdHandler:    cmd,
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
		return nil, fmt.Errorf("initilizing character: %w", err)
	}
	// Save the character back to preserve changes
	err = m.chars.Save(strings.ToLower(char.Name), char)
	if err != nil {
		return nil, fmt.Errorf("saving character: %w", err)
	}

	return &Player{
		conn: conn,
		state: &State{
			mu:   sync.Mutex{},
			char: char,
		},
	}, nil
}
