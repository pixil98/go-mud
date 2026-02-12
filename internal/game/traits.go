package game

import (
	"fmt"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// Traits holds properties shared between characters and mobiles.
type Traits struct {
	Pronoun storage.Identifier `json:"pronoun,omitempty"`
	Race    storage.Identifier `json:"race,omitempty"`
	Level   int                `json:"level,omitempty"`
}

type pronounPossessive struct {
	Adjective string `json:"adjective"`
	Pronoun   string `json:"pronoun"`
}

// Pronoun defines a set of pronouns loaded from asset files.
type Pronoun struct {
	Subjective string            `json:"subjective"`
	Objective  string            `json:"objective"`
	Possessive pronounPossessive `json:"possessive"`
	Reflexive  string            `json:"reflexive"`
}

func (p *Pronoun) Validate() error {
	return nil
}

func (p *Pronoun) Selector() string {
	return fmt.Sprintf("%s/%s", p.Subjective, p.Objective)
}

// Race defines a playable race loaded from asset files.
type Race struct {
	Name         string         `json:"name"`
	Abbreviation string         `json:"abbreviation"`
	Stats        map[string]int `json:"stats"`
	Perks        []string       `json:"perks"`
}

func (r *Race) Validate() error {
	el := errors.NewErrorList()

	for _, p := range r.Perks {
		el.Add(func() error {
			switch p {
			case "darkvision":
				return nil
			default:
				return fmt.Errorf("unknown perk: %s", p)
			}
		}())
	}

	return el.Err()
}

func (r *Race) Selector() string {
	return r.Name
}
