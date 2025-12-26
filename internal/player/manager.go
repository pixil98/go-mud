package player

import (
	"context"
	"io"
	"strings"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/storage"
)

type PlayerManager struct {
	players    map[string]*Player
	cmdHandler *commands.Handler

	loginFlow        *loginFlow
	charCreationFlow *characterCreationFlow

	chars storage.Storer[*Character]
	//pronouns storage.Storer[*Pronoun]
	//races    storage.Storer[*Race]
}

func NewPlayerManager(cmd *commands.Handler, cs storage.Storer[*Character], ps storage.Storer[*Pronoun], rs storage.Storer[*Race]) *PlayerManager {
	pm := &PlayerManager{
		players:   map[string]*Player{},
		chars:     cs,
		loginFlow: &loginFlow{cStore: cs},
		charCreationFlow: &characterCreationFlow{
			pSelector: NewSelector(ps.GetAll()),
			rSelector: NewSelector(rs.GetAll()),
		},
		cmdHandler: cmd,
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

	err = m.charCreationFlow.Run(conn, char)
	if err != nil {
		return nil, err
	}

	// Save the character back to preserve changes
	if m.chars == nil {
		conn.Write([]byte("chars is nil"))
	}
	m.chars.Save(strings.ToLower(char.Name), char)

	return &Player{
		conn: conn,
		state: &State{
			char: char,
		},
	}, nil
}
