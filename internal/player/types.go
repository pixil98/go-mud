package player

import (
	"fmt"

	"github.com/pixil98/go-errors/errors"
	"github.com/pixil98/go-mud/internal/storage"
)

type Character struct {
	Name     string             `json:"name"`
	Password string             `json:"password"` //TODO make this okay to save
	Pronoun  storage.Identifier `json:"pronoun"`
	Race     storage.Identifier `json:"race"`
}

func (c *Character) Validate() error {
	return nil
}

// Selectable
type pronounPossessive struct {
	Adjective string `json:"adjective"`
	Pronoun   string `json:"pronoun"`
}

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

type Race struct {
	Name  string         `json:"name"`
	Stats map[string]int `json:"stats"`
	Perks []string       `json:"perks"`
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
