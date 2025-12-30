package player

import (
	"context"
	"io"
	"sync"
)

type State struct {
	mu   sync.Mutex
	char *Character
}

type Player struct {
	conn  io.ReadWriter
	state *State
}

func (p *Player) Tick(ctx context.Context) {
	// do something like regen here
	//p.character.Regen()
}

func (p *Player) Play(ctx context.Context) error {
	return nil
}
