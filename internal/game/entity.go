package game

import "github.com/pixil98/go-mud/internal/storage"

// Entity represents the shared properties of any named game entity
// (player characters, mobiles, and future objects).
type Entity struct {
	EntityName string `json:"name"`
	Title      string `json:"title,omitempty"`

	storage.ExtensionState `json:"ext,omitempty"`
}

// Name returns the entity's display name.
func (e *Entity) Name() string {
	return e.EntityName
}
