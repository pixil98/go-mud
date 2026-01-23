package game

import "github.com/pixil98/go-mud/internal/storage"

// EntityState holds location and other shared state for a single entity (player, mob, etc).
type EntityState struct {
	Zone storage.Identifier
	Room storage.Identifier

	// Quit signals the entity wants to disconnect
	Quit bool
}
