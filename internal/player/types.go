package player

import (
	"github.com/pixil98/go-mud/internal/storage"
)

type Character struct {
	Name     string `json:"name"`
	Password string `json:"password"` //TODO make this okay to save

	storage.ExtensionState `json:"ext,omitempty"`
}

func (c *Character) Validate() error {
	return nil
}
