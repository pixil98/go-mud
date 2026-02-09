package game

import "github.com/pixil98/go-mud/internal/storage"

// Character represents a player character in the game.
type Character struct {
	// Name is the character's display name
	Name string `json:"name"`

	// Password is the character's login credential
	// TODO: make this okay to save (hash it)
	Password string `json:"password"`

	// Title is displayed after the character's name (e.g., "Bob the Brave")
	Title string `json:"title,omitempty"`

	// DetailedDesc is shown when a player looks at this character
	DetailedDesc string `json:"detailed_desc"`

	// Inventory holds items carried by this character
	Inventory *Inventory `json:"inventory,omitempty"`

	// Last known location, saved on quit/save for restoring on login
	LastZone storage.Identifier `json:"last_zone,omitempty"`
	LastRoom storage.Identifier `json:"last_room,omitempty"`

	// ExtensionState holds plugin-specific data for this character
	storage.ExtensionState `json:"ext,omitempty"`
}

// Validate satisfies storage.ValidatingSpec
// TODO: We should validate some things here
func (c *Character) Validate() error {
	return nil
}
