package base

import (
	"context"
	"fmt"
	"io"

	"github.com/pixil98/go-mud/internal/plugins"
	"github.com/pixil98/go-mud/internal/storage"
)

const (
	baseExtKey = "base"
)

type baseChar struct {
	Pronoun storage.Identifier `json:"pronoun"`
	Race    storage.Identifier `json:"race"`
}

type BasePlugin struct {
	pronouns *storage.SelectableStorer[*Pronoun]
	races    *storage.SelectableStorer[*Race]
}

func (p *BasePlugin) Key() string {
	return baseExtKey
}

func (p *BasePlugin) Init() error {
	//TODO load these paths from config?
	s, err := storage.NewFileStore[*Pronoun]("./assets/pronouns")
	if err != nil {
		return fmt.Errorf("loading pronouns: %w", err)
	}
	p.pronouns = storage.NewSelectableStorer(s)

	r, err := storage.NewFileStore[*Race]("./assets/races")
	if err != nil {
		return fmt.Errorf("loading races: %w", err)
	}
	p.races = storage.NewSelectableStorer(r)

	return nil
}

func (p *BasePlugin) Tick(ctx context.Context) error {
	return nil
}

func (p *BasePlugin) OnInitCharacter(rw io.ReadWriter, e plugins.Extensible) error {
	char := &baseChar{}
	_, err := e.Get(baseExtKey, char)
	if err != nil {
		return err
	}

	for char.Pronoun == "" {
		sel, err := p.pronouns.Prompt(rw, "What are your pronouns?")
		if err != nil {
			return fmt.Errorf("selecting pronouns: %w", err)
		}

		char.Pronoun = sel
	}

	for char.Race == "" {
		sel, err := p.races.Prompt(rw, "What is your race?")
		if err != nil {
			return fmt.Errorf("selecting race: %w", err)
		}

		char.Race = sel
	}

	return e.Set(baseExtKey, char)
}
