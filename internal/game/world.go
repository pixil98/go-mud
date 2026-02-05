package game

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pixil98/go-mud/internal/storage"
)

// WorldState is the single source of truth for all mutable game state.
// All access must go through its methods to ensure thread-safety.
type WorldState struct {
	mu         sync.RWMutex
	subscriber Subscriber
	players    map[storage.Identifier]*PlayerState

	// Stores for looking up entities
	chars   storage.Storer[*Character]
	zones   storage.Storer[*Zone]
	rooms   storage.Storer[*Room]
	mobiles storage.Storer[*Mobile]

	// Mobile instances indexed by zone -> room -> instanceId -> instance
	mobileInstances map[storage.Identifier]map[storage.Identifier]map[string]*MobileInstance
}

// NewWorldState creates a new WorldState.
func NewWorldState(sub Subscriber, chars storage.Storer[*Character], zones storage.Storer[*Zone], rooms storage.Storer[*Room], mobiles storage.Storer[*Mobile]) *WorldState {
	return &WorldState{
		subscriber:      sub,
		players:         make(map[storage.Identifier]*PlayerState),
		chars:           chars,
		zones:           zones,
		rooms:           rooms,
		mobiles:         mobiles,
		mobileInstances: make(map[storage.Identifier]map[storage.Identifier]map[string]*MobileInstance),
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

// Mobiles returns the mobile store.
func (w *WorldState) Mobiles() storage.Storer[*Mobile] {
	return w.mobiles
}

// SpawnMobile creates a new mobile instance in the specified location.
func (w *WorldState) SpawnMobile(mobileId, zoneId, roomId storage.Identifier) *MobileInstance {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Ensure nested maps exist
	if w.mobileInstances[zoneId] == nil {
		w.mobileInstances[zoneId] = make(map[storage.Identifier]map[string]*MobileInstance)
	}
	if w.mobileInstances[zoneId][roomId] == nil {
		w.mobileInstances[zoneId][roomId] = make(map[string]*MobileInstance)
	}

	instance := &MobileInstance{
		InstanceId: uuid.New().String(),
		MobileId:   mobileId,
		ZoneId:     zoneId,
		RoomId:     roomId,
	}
	w.mobileInstances[zoneId][roomId][instance.InstanceId] = instance
	return instance
}

// GetMobilesInRoom returns all mobile instances in a room.
func (w *WorldState) GetMobilesInRoom(zoneId, roomId storage.Identifier) []*MobileInstance {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.mobileInstances[zoneId] == nil || w.mobileInstances[zoneId][roomId] == nil {
		return nil
	}

	result := make([]*MobileInstance, 0, len(w.mobileInstances[zoneId][roomId]))
	for _, mi := range w.mobileInstances[zoneId][roomId] {
		result = append(result, mi)
	}
	return result
}

// GetPlayer returns the player state. Returns nil if player not found.
func (w *WorldState) GetPlayer(charId storage.Identifier) *PlayerState {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.players[charId]
}

// AddPlayer registers a new player in the world state.
func (w *WorldState) AddPlayer(charId storage.Identifier, msgs chan []byte, zoneId storage.Identifier, roomId storage.Identifier) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.players[charId]; exists {
		return ErrPlayerExists
	}

	w.players[charId] = &PlayerState{
		subscriber:   w.subscriber,
		subs:         make(map[string]func()),
		msgs:         msgs,
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

// --- PlayerState Methods (access properties) ---

// Location returns the player's current zone and room.
func (p *PlayerState) Location() (zoneId, roomId storage.Identifier) {
	return p.ZoneId, p.RoomId
}

// Move updates the player's location and manages zone/room subscriptions.
func (p *PlayerState) Move(toZoneId, toRoomId storage.Identifier) {
	prevZone, prevRoom := p.ZoneId, p.RoomId

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

// Subscriber provides the ability to subscribe to message subjects
type Subscriber interface {
	Subscribe(subject string, handler func(data []byte)) (unsubscribe func(), err error)
}

// PlayerState holds all mutable state for an active player.
type PlayerState struct {
	subscriber Subscriber
	msgs       chan []byte

	// Location
	ZoneId storage.Identifier
	RoomId storage.Identifier

	// Subscriptions
	subs map[string]func()

	// Session state
	Quit         bool
	LastActivity time.Time
}

func (p *PlayerState) Subscribe(subject string) error {
	if p.subscriber == nil {
		return fmt.Errorf("subscriber is nil")
	}

	unsub, err := p.subscriber.Subscribe(subject, func(data []byte) {
		p.msgs <- data
	})
	if err != nil {
		return fmt.Errorf("subscribing to channel '%s': %w", subject, err)
	}
	p.subs[subject] = unsub
	return nil

}

// Unsubscribe removes a subscription by name
func (p *PlayerState) Unsubscribe(name string) {
	if unsub, ok := p.subs[name]; ok {
		unsub()
		delete(p.subs, name)
	}
}

// UnsubscribeAll removes all subscriptions
func (p *PlayerState) UnsubscribeAll() {
	for name, unsub := range p.subs {
		unsub()
		delete(p.subs, name)
	}
}
