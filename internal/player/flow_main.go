package player

import (
	"context"
	"encoding/csv"
	"io"
	"strings"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/storage"
)

const (
	mainStepLogin int = iota
)

type MainFlow struct {
	cmdHandler *commands.Handler
	pStore     storage.Storer[*Pronoun]
	rStore     storage.Storer[*Race]
	cStore     storage.Storer[*Character]
	step       int
}

func NewMainFlow(cmd *commands.Handler, p storage.Storer[*Pronoun], r storage.Storer[*Race], c storage.Storer[*Character]) *MainFlow {
	f := &MainFlow{
		cmdHandler: cmd,
		pStore:     p,
		rStore:     r,
		cStore:     c,
		step:       mainStepLogin,
	}

	return f
}

func (f *MainFlow) Name() string {
	return "main"
}

func (f *MainFlow) Run(str string, state *State, w io.Writer) (bool, error) {
	switch f.step {
	case mainStepLogin:
		cmd := f.parseInput(str)
		if len(cmd) > 0 {
			err := f.cmdHandler.Exec(context.TODO(), cmd[0], cmd[1:]...)
			if err != nil {
				return false, err
			}
		}

		return false, nil
	}

	return true, nil
}

// TODO: I'd like to support single quotes as well
func (f *MainFlow) parseInput(str string) []string {
	r := csv.NewReader(strings.NewReader(str))
	r.Comma = ' '
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	rec, err := r.Read()
	if err != nil {
		return []string{}
	}

	return rec
}
