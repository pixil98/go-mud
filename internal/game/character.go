package game

import (
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
		for _, oi := range c.Inventory.Items {
			populateObjDefinition(oi, dict.Objects)
		}
	}
	if c.Equipment != nil {
		for _, slot := range c.Equipment.Items {
			if slot.Obj != nil {
				populateObjDefinition(slot.Obj, dict.Objects)
			}
		}
	}
	return nil
}

// populateObjDefinition links an ObjectInstance to its definition and
// ensures containers have a non-nil Contents inventory.
func populateObjDefinition(oi *ObjectInstance, objDefs storage.Storer[*Object]) {
	oi.Definition = objDefs.Get(string(oi.ObjectId))
	if oi.Definition != nil && oi.Definition.HasFlag(ObjectFlagContainer) && oi.Contents == nil {
		oi.Contents = NewInventory()
	}
	if oi.Contents != nil {
		for _, ci := range oi.Contents.Items {
			populateObjDefinition(ci, objDefs)
		}
	}
}

// Validate satisfies storage.ValidatingSpec
// TODO: We should validate some things here
func (c *Character) Validate() error {
	return nil
}
