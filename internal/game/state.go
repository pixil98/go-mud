package game

import (
	"sync"
	"time"

	"github.com/pixil98/go-mud/internal/storage"
)

// WorldState is the single source of truth for all mutable game state.
// All access must go through its methods to ensure thread-safety.
type WorldState struct {
	mu      sync.RWMutex
	players map[storage.Identifier]*PlayerState

	// Stores for looking up entities
	chars storage.Storer[*Character]
	zones storage.Storer[*Zone]
}

// PlayerState holds all mutable state for an active player.
type PlayerState struct {
	// Location
	ZoneId storage.Identifier
	RoomId storage.Identifier

	// Session state
	Quit         bool
	LastActivity time.Time
}
