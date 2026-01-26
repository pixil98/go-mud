package player

import (
	"github.com/pixil98/go-mud/internal/storage"
)

type Character struct {
	CharName string `json:"name"`
	Password string `json:"password"` //TODO make this okay to save

	storage.ExtensionState `json:"ext,omitempty"`
}

// Name returns the character's display name
func (c *Character) Name() string {
	return c.CharName
}

func (c *Character) Validate() error {
	return nil
}
