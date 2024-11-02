package player

import (
	"io"

	"github.com/pixil98/go-mud/internal/storage"
)

const (
	mainStepLogin int = iota
)

type MainFlow struct {
	pStore storage.Storer[*Pronoun]
	rStore storage.Storer[*Race]
	cStore storage.Storer[*Character]
	step   int
}

func NewMainFlow(p storage.Storer[*Pronoun], r storage.Storer[*Race], c storage.Storer[*Character]) *MainFlow {
	f := &MainFlow{
		pStore: p,
		rStore: r,
		cStore: c,
		step:   mainStepLogin,
	}

	return f
}

func (f *MainFlow) Name() string {
	return "main"
}

func (f *MainFlow) Run(str string, state *State, w io.Writer) (bool, error) {
	w.Write([]byte("Welcome to the main flow\n"))

	return true, nil
}
