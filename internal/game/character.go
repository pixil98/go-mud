package game

import (
	"encoding/json"
	"strings"

	"github.com/pixil98/go-mud/internal/storage"
)

// Character represents a player character in the game.
type Character struct {
	// Name is the character's display name
	Name string `json:"name"`

	// Password is the bcrypt-hashed login credential
	Password string `json:"password"`

	// Title is displayed after the character's name (e.g., "Bob the Brave")
	Title string `json:"title,omitempty"`

	// DetailedDesc is shown when a player looks at this character
	DetailedDesc string `json:"detailed_desc"`

	// Last known location, saved on quit/save for restoring on login
	LastZone storage.Identifier `json:"last_zone,omitempty"`
	LastRoom storage.Identifier `json:"last_room,omitempty"`

	Actor
	ActorInstance
}

func (c *Character) UnmarshalJSON(b []byte) error {
	type Alias Character
	if err := json.Unmarshal(b, (*Alias)(c)); err != nil {
		return err
	}
	if c.Inventory == nil {
		c.Inventory = NewInventory()
	}
	if c.Equipment == nil {
		c.Equipment = NewEquipment()
	}
	return nil
}

func NewCharacter(name string, pass string) *Character {
	return &Character{
		Name:         name,
		Password:     pass,
		Title:        "the Newbie",
		DetailedDesc: "A plain, unremarkable adventurer.",
		ActorInstance: ActorInstance{
			Inventory: NewInventory(),
			Equipment: NewEquipment(),
		},
	}
}

// StatSections returns the character's stat display sections.
func (c *Character) StatSections() []StatSection {
	sections := c.Actor.statSections()

	name := c.Name
	if c.Title != "" {
		name = c.Name + " " + c.Title
	}
	sections[0].Lines = append([]StatLine{{Value: name, Center: true}}, sections[0].Lines...)
	return sections
}

// MatchName returns true if name matches this character's name (case-insensitive).
func (c *Character) MatchName(name string) bool {
	return strings.EqualFold(c.Name, name)
}

// Resolve resolves all foreign keys on the character from the dictionary.
func (c *Character) Resolve(dict *Dictionary) error {
	if err := c.Actor.Resolve(dict); err != nil {
		return err
	}

	if c.Inventory != nil {
		for _, oi := range c.Inventory.Objs {
			if err := resolveObj(oi, dict.Objects); err != nil {
				return err
			}
		}
	}
	if c.Equipment != nil {
		for _, slot := range c.Equipment.Objs {
			if slot.Obj != nil {
				if err := resolveObj(slot.Obj, dict.Objects); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// resolveObj resolves an ObjectInstance's SmartIdentifier and
// ensures containers have a non-nil Contents inventory.
func resolveObj(oi *ObjectInstance, objDefs storage.Storer[*Object]) error {
	if err := oi.Object.Resolve(objDefs); err != nil {
		return err
	}
	if oi.Object.Id().HasFlag(ObjectFlagContainer) && oi.Contents == nil {
		oi.Contents = NewInventory()
	}
	if oi.Contents != nil {
		for _, ci := range oi.Contents.Objs {
			if err := resolveObj(ci, objDefs); err != nil {
				return err
			}
		}
	}
	return nil
}

// Validate a character definition
// TODO: We should validate some things here
func (c *Character) Validate() error {
	return nil
}
