package player

import (
	"context"
	"io"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/storage"
)

type PlayerManager struct {
	players    map[string]*Player
	cmdHandler *commands.Handler

	chars    storage.Storer[*Character]
	pronouns storage.Storer[*Pronoun]
	races    storage.Storer[*Race]
}

func NewPlayerManager(cmd *commands.Handler, cs storage.Storer[*Character], ps storage.Storer[*Pronoun], rs storage.Storer[*Race]) *PlayerManager {
	pm := &PlayerManager{
		players:    map[string]*Player{},
		cmdHandler: cmd,
		chars:      cs,
		pronouns:   ps,
		races:      rs,
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

func (m *PlayerManager) NewPlayer(conn io.ReadWriter) *Player {
	lf := NewLoginFlow(m.chars, NewCharacterCreationFlow(m.chars, m.pronouns, m.races))

	p := &Player{
		conn:      conn,
		state:     &State{},
		flow:      lf,
		loginFlow: lf,
		mainFlow:  NewMainFlow(m.cmdHandler, m.pronouns, m.races, m.chars),
	}

	return p
}
