package game

import (
	"time"

	"github.com/pixil98/go-mud/internal/storage"
)

// NewWorldState creates a new WorldState.
func NewWorldState(chars storage.Storer[*Character], zones storage.Storer[*Zone], rooms storage.Storer[*Room]) *WorldState {
	return &WorldState{
		players: make(map[storage.Identifier]*PlayerState),
		chars:   chars,
		zones:   zones,
		rooms:   rooms,
	}
}

// --- WorldState Methods (manage the collection) ---

// Characters returns the character store.
func (w *WorldState) Characters() storage.Storer[*Character] {
	return w.chars
}

// Zones returns the zone store.
func (w *WorldState) Zones() storage.Storer[*Zone] {
	return w.zones
}

// Rooms returns the room store.
func (w *WorldState) Rooms() storage.Storer[*Room] {
	return w.rooms
}

// GetPlayer returns the player state. Returns nil if player not found.
func (w *WorldState) GetPlayer(charId storage.Identifier) *PlayerState {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.players[charId]
}

// AddPlayer registers a new player in the world state.
func (w *WorldState) AddPlayer(charId, zoneId, roomId storage.Identifier) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.players[charId]; exists {
		return ErrPlayerExists
	}

	w.players[charId] = &PlayerState{
		ZoneId:       zoneId,
		RoomId:       roomId,
		Quit:         false,
		LastActivity: time.Now(),
	}
	return nil
}

// RemovePlayer removes a player from the world state.
func (w *WorldState) RemovePlayer(charId storage.Identifier) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.players[charId]; !exists {
		return ErrPlayerNotFound
	}

	delete(w.players, charId)
	return nil
}

// MovePlayer updates a player's location.
func (w *WorldState) MovePlayer(charId, toZoneId, toRoomId storage.Identifier) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, exists := w.players[charId]
	if !exists {
		return ErrPlayerNotFound
	}

	p.ZoneId = toZoneId
	p.RoomId = toRoomId
	p.LastActivity = time.Now()
	return nil
}

// SetPlayerQuit sets the quit flag for a player.
func (w *WorldState) SetPlayerQuit(charId storage.Identifier, quit bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, exists := w.players[charId]
	if !exists {
		return ErrPlayerNotFound
	}

	p.Quit = quit
	return nil
}

// UpdatePlayer applies a mutation function to player state.
// The function receives a pointer to the player state and can modify it directly.
func (w *WorldState) UpdatePlayer(charId storage.Identifier, fn func(*PlayerState)) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, exists := w.players[charId]
	if !exists {
		return ErrPlayerNotFound
	}

	fn(p)
	p.LastActivity = time.Now()
	return nil
}

// ForEachPlayer calls the given function for each player.
// The function receives a copy of the player state.
func (w *WorldState) ForEachPlayer(fn func(PlayerState)) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for _, p := range w.players {
		fn(*p)
	}
}

// --- PlayerState Methods (access properties) ---

// Location returns the player's current zone and room.
func (p *PlayerState) Location() (zoneId, roomId storage.Identifier) {
	return p.ZoneId, p.RoomId
}

// IsQuitting returns whether the player has requested to quit.
func (p *PlayerState) IsQuitting() bool {
	return p.Quit
}
