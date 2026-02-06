package game

import (
	"github.com/pixil98/go-mud/internal/storage"
)

// Entity represents the shared properties of any named game entity
// (player characters, mobiles, and future objects).
type Entity struct {
	// DetailedDesc is shown when a player looks at the target
	DetailedDesc string `json:"detailed_desc"`

	storage.ExtensionState `json:"ext,omitempty"`
}

func (e *Entity) Validate() error {
	return nil
}
