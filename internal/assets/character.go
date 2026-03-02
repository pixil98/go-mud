package assets

import (
	"strings"

	"github.com/pixil98/go-mud/internal/storage"
)

// DefaultBaseStats returns base stats initialized to 10 for all abilities.
func DefaultBaseStats() map[StatKey]int {
	return map[StatKey]int{
		StatSTR: 10, StatDEX: 10, StatCON: 10,
		StatINT: 10, StatWIS: 10, StatCHA: 10,
	}
}

// Character is the persistent spec for a player character.
// It is loaded from and saved to the character store.
type Character struct {
	Name         string `json:"name"`
	Password     string `json:"password"`
	Title        string `json:"title,omitempty"`
	DetailedDesc string `json:"detailed_desc"`

	// Last known location, restored on login
	LastZone string `json:"last_zone,omitempty"`
	LastRoom string `json:"last_room,omitempty"`

	Pronoun storage.SmartIdentifier[*Pronoun] `json:"pronoun,omitempty"`
	Race    storage.SmartIdentifier[*Race]    `json:"race,omitempty"`

	Level      int            `json:"level,omitempty"`
	BaseStats  map[StatKey]int `json:"base_stats,omitempty"`
	Experience int            `json:"experience,omitempty"`

	// Persisted runtime state
	MaxHP     int `json:"max_hp,omitempty"`
	CurrentHP int `json:"current_hp,omitempty"`

	// Inventory and equipment stored as spawn specs so objects are re-materialized on login
	Inventory []ObjectSpawn          `json:"inventory,omitempty"`
	Equipment map[string]ObjectSpawn `json:"equipment,omitempty"`
}

// NewCharacter creates a new level-0 character with default values.
// Level and HP are set by the caller after character creation (via Gain).
func NewCharacter(name, password string) *Character {
	return &Character{
		Name:         name,
		Password:     password,
		Title:        "the Newbie",
		DetailedDesc: "A plain, unremarkable adventurer.",
	}
}

// MatchName returns true if name matches this character's name (case-insensitive).
func (c *Character) MatchName(name string) bool {
	return strings.EqualFold(c.Name, name)
}

// Selector returns the character's name for use in selectable lists.
func (c *Character) Selector() string {
	return c.Name
}

// Validate returns an error if the character definition is invalid.
func (c *Character) Validate() error {
	return nil
}

// Resolve resolves all foreign key references on the character.
func (c *Character) Resolve(pronouns storage.Storer[*Pronoun], races storage.Storer[*Race], objs storage.Storer[*Object]) error {
	if c.Race.Id() != "" {
		if err := c.Race.Resolve(races); err != nil {
			return err
		}
	}
	if c.Pronoun.Id() != "" {
		if err := c.Pronoun.Resolve(pronouns); err != nil {
			return err
		}
	}
	for i := range c.Inventory {
		if err := c.Inventory[i].Resolve(objs); err != nil {
			return err
		}
	}
	for slot, spawn := range c.Equipment {
		s := spawn
		if err := s.Resolve(objs); err != nil {
			return err
		}
		c.Equipment[slot] = s
	}
	return nil
}
