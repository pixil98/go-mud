package game

import "github.com/pixil98/go-mud/internal/storage"

// Character represents a player character in the game.
type Character struct {
	CharName string `json:"name"`
	Password string `json:"password"` //TODO make this okay to save
	Title    string `json:"title,omitempty"`

	storage.ExtensionState `json:"ext,omitempty"`
}

// Name returns the character's display name
func (c *Character) Name() string {
	return c.CharName
}

// Validate satisfies storage.ValidatingSpec
func (c *Character) Validate() error {
	return nil
}
