package game

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pixil98/go-mud/internal/storage"
)

// WorldState is the single source of truth for all mutable game state.
// All access must go through its methods to ensure thread-safety.
type WorldState struct {
	mu         sync.RWMutex
	subscriber Subscriber
	players    map[storage.Identifier]*PlayerState

	instances map[storage.Identifier]*ZoneInstance
}

// NewWorldState creates a new WorldState with zone and room instances initialized.
func NewWorldState(sub Subscriber, zones storage.Storer[*Zone], rooms storage.Storer[*Room]) *WorldState {
	// Build zone instances
	instances := make(map[storage.Identifier]*ZoneInstance)
	for zoneId, zone := range zones.GetAll() {
		instances[zoneId] = NewZoneInstance(zoneId, zone)
	}

	// Build room instances and add to their zones
	for roomId, room := range rooms.GetAll() {
		zoneId := storage.Identifier(room.Zone.Get())
		if zi, ok := instances[zoneId]; ok {
			zi.AddRoom(roomId, NewRoomInstance(roomId, room))
		}
	}

	return &WorldState{
		subscriber: sub,
		players:    make(map[storage.Identifier]*PlayerState),
		instances:  instances,
	}
}

// Instances returns all zone instances.
func (w *WorldState) Instances() map[storage.Identifier]*ZoneInstance {
	return w.instances
}

// GetPlayer returns the player state. Returns nil if player not found.
func (w *WorldState) GetPlayer(charId storage.Identifier) *PlayerState {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.players[charId]
}

// AddPlayer registers a new player in the world state and adds them to the room instance.
func (w *WorldState) AddPlayer(charId storage.Identifier, char *Character, msgs chan []byte, zoneId storage.Identifier, roomId storage.Identifier) error {
	w.mu.Lock()
	if _, exists := w.players[charId]; exists {
		w.mu.Unlock()
		return ErrPlayerExists
	}

	ps := &PlayerState{
		subscriber:   w.subscriber,
		subs:         make(map[string]func()),
		msgs:         msgs,
		CharId:       charId,
		Character:    char,
		ZoneId:       zoneId,
		RoomId:       roomId,
		Quit:         false,
		LastActivity: time.Now(),
	}
	w.players[charId] = ps
	room := w.instances[zoneId].GetRoom(roomId)
	w.mu.Unlock()

	room.AddPlayer(charId, ps)
	return nil
}

// RemovePlayer removes a player from the world state and from the room instance.
func (w *WorldState) RemovePlayer(charId storage.Identifier) error {
	w.mu.Lock()
	ps, exists := w.players[charId]
	if !exists {
		w.mu.Unlock()
		return ErrPlayerNotFound
	}

	room := w.instances[ps.ZoneId].GetRoom(ps.RoomId)
	delete(w.players, charId)
	w.mu.Unlock()

	room.RemovePlayer(charId)
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
func (w *WorldState) ForEachPlayer(fn func(storage.Identifier, PlayerState)) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for id, p := range w.players {
		fn(id, *p)
	}
}

// Subscriber provides the ability to subscribe to message subjects
type Subscriber interface {
	Subscribe(subject string, handler func(data []byte)) (unsubscribe func(), err error)
}

// Tick processes zone resets based on their reset mode and lifespan.
func (w *WorldState) Tick(ctx context.Context) error {
	for _, zi := range w.instances {
		zi.Reset(false)
	}
	return nil
}

// PlayerState holds all mutable state for an active player.
type PlayerState struct {
	subscriber Subscriber
	msgs       chan []byte

	CharId    storage.Identifier
	Character *Character

	// Location
	ZoneId storage.Identifier
	RoomId storage.Identifier

	// Subscriptions
	subs map[string]func()

	// Session state
	Quit         bool
	LastActivity time.Time
}

// Location returns the player's current zone and room.
func (p *PlayerState) Location() (zoneId, roomId storage.Identifier) {
	return p.ZoneId, p.RoomId
}

// Move updates the player's location, manages zone/room subscriptions,
// and updates room instance player lists.
func (p *PlayerState) Move(fromRoom, toRoom *RoomInstance) {
	prevZone, prevRoom := p.ZoneId, p.RoomId
	toZoneId := storage.Identifier(toRoom.Definition.Zone.Get())
	toRoomId := toRoom.RoomId

	// Update room player lists
	fromRoom.RemovePlayer(p.CharId)
	toRoom.AddPlayer(p.CharId, p)

	// Update location
	p.ZoneId = toZoneId
	p.RoomId = toRoomId

	// Update zone subscription if zone changed
	if toZoneId != prevZone {
		p.Unsubscribe(fmt.Sprintf("zone-%s", prevZone))
		_ = p.Subscribe(fmt.Sprintf("zone-%s", toZoneId))
	}

	// Update room subscription if zone or room changed
	if toZoneId != prevZone || toRoomId != prevRoom {
		p.Unsubscribe(fmt.Sprintf("zone-%s-room-%s", prevZone, prevRoom))
		_ = p.Subscribe(fmt.Sprintf("zone-%s-room-%s", toZoneId, toRoomId))
	}
}

// Subscribe adds a new subscription
func (p *PlayerState) Subscribe(subject string) error {
	if p.subscriber == nil {
		return fmt.Errorf("subscriber is nil")
	}

	unsub, err := p.subscriber.Subscribe(subject, func(data []byte) {
		p.msgs <- data
	})

	// If we some how are subscribing to a channel we already think we have
	// unsubscribe from the existing one.
	if unsub, ok := p.subs[subject]; ok {
		unsub()
	}

	if err != nil {
		return fmt.Errorf("subscribing to channel '%s': %w", subject, err)
	}
	p.subs[subject] = unsub
	return nil

}

// Unsubscribe removes a subscription by name
func (p *PlayerState) Unsubscribe(subject string) {
	if unsub, ok := p.subs[subject]; ok {
		unsub()
		delete(p.subs, subject)
	}
}

// UnsubscribeAll removes all subscriptions
func (p *PlayerState) UnsubscribeAll() {
	for name, unsub := range p.subs {
		unsub()
		delete(p.subs, name)
	}
}
