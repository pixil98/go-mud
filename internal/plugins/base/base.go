package base

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/plugins"
	"github.com/pixil98/go-mud/internal/storage"
)

const (
	baseExtKey = "base"
)

type baseChar struct {
	Pronoun storage.Identifier `json:"pronoun"`
	Race    storage.Identifier `json:"race"`
	Level   int                `json:"level"`
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

func (p *BasePlugin) InitCharacter(rw io.ReadWriter, c *game.Character) error {
	bc := &baseChar{}
	_, err := c.Get(baseExtKey, bc)
	if err != nil {
		return err
	}

	for bc.Pronoun == "" {
		sel, err := p.pronouns.Prompt(rw, "What are your pronouns?")
		if err != nil {
			return fmt.Errorf("selecting pronouns: %w", err)
		}

		bc.Pronoun = sel
	}

	for bc.Race == "" {
		sel, err := p.races.Prompt(rw, "What is your race?")
		if err != nil {
			return fmt.Errorf("selecting race: %w", err)
		}

		bc.Race = sel
	}

	// Set default level for new characters
	if bc.Level == 0 {
		bc.Level = 1
	}

	return c.Set(baseExtKey, bc)
}

func (p *BasePlugin) GetCharacterInfo(c *game.Character, style plugins.InfoStyle) map[string]string {
	result := make(map[string]string)

	bc := &baseChar{}
	found, err := c.Get(baseExtKey, bc)
	if err != nil || !found {
		return result
	}

	// Look up race
	if race := p.races.Get(string(bc.Race)); race != nil {
		switch style {
		case plugins.InfoStyleShort:
			result["race"] = race.Abbreviation
		case plugins.InfoStyleFull:
			result["race"] = race.Name
		}
	}

	result["level"] = strconv.Itoa(bc.Level)

	return result
}
