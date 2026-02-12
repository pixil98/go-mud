package game

import (
	"context"
	"fmt"
	"log/slog"
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
	objects storage.Storer[*Object]

	// Mobile instances indexed by zone -> room -> instanceId -> instance
	mobileInstances map[storage.Identifier]map[storage.Identifier]map[string]*MobileInstance

	// Object instances indexed by zone -> room -> instanceId -> instance
	objectInstances map[storage.Identifier]map[storage.Identifier]map[string]*ObjectInstance

	// Index of rooms by zone for efficient zone resets
	roomsByZone map[storage.Identifier][]storage.Identifier
}

// NewWorldState creates a new WorldState.
func NewWorldState(sub Subscriber, chars storage.Storer[*Character], zones storage.Storer[*Zone], rooms storage.Storer[*Room], mobiles storage.Storer[*Mobile], objects storage.Storer[*Object]) *WorldState {
	// Build index of rooms by zone
	roomsByZone := make(map[storage.Identifier][]storage.Identifier)
	for roomId, room := range rooms.GetAll() {
		zoneId := storage.Identifier(room.ZoneId)
		roomsByZone[zoneId] = append(roomsByZone[zoneId], roomId)
	}

	return &WorldState{
		subscriber:      sub,
		players:         make(map[storage.Identifier]*PlayerState),
		chars:           chars,
		zones:           zones,
		rooms:           rooms,
		mobiles:         mobiles,
		objects:         objects,
		mobileInstances: make(map[storage.Identifier]map[storage.Identifier]map[string]*MobileInstance),
		objectInstances: make(map[storage.Identifier]map[storage.Identifier]map[string]*ObjectInstance),
		roomsByZone:     roomsByZone,
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

// Objects returns the object store.
func (w *WorldState) Objects() storage.Storer[*Object] {
	return w.objects
}

// spawnMobile creates a new mobile instance in the specified location.
func (w *WorldState) SpawnMobileInstance(mobileId, zoneId, roomId storage.Identifier) *MobileInstance {
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
	}
	w.mobileInstances[zoneId][roomId][instance.InstanceId] = instance

	slog.Debug("spawned mobile", "mobile", mobileId, "zone", zoneId, "room", roomId)
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

// spawnObjectInstance creates an ObjectInstance from an ObjectSpawn,
// recursively spawning any contents for containers.
func spawnObjectInstance(spawn ObjectSpawn) *ObjectInstance {
	instance := &ObjectInstance{
		InstanceId: uuid.New().String(),
		ObjectId:   storage.Identifier(spawn.ObjectId),
	}
	if len(spawn.Contents) > 0 {
		instance.Contents = NewInventory()
		for _, contentSpawn := range spawn.Contents {
			instance.Contents.Add(spawnObjectInstance(contentSpawn))
		}
	}
	return instance
}

// GetObjectsInRoom returns all object instances in a room.
func (w *WorldState) GetObjectsInRoom(zoneId, roomId storage.Identifier) []*ObjectInstance {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.objectInstances[zoneId] == nil || w.objectInstances[zoneId][roomId] == nil {
		return nil
	}

	result := make([]*ObjectInstance, 0, len(w.objectInstances[zoneId][roomId]))
	for _, oi := range w.objectInstances[zoneId][roomId] {
		result = append(result, oi)
	}
	return result
}

func (w *WorldState) removeObjectFromRoom(zoneId, roomId storage.Identifier, instanceId string) *ObjectInstance {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.objectInstances[zoneId] == nil || w.objectInstances[zoneId][roomId] == nil {
		return nil
	}

	oi, ok := w.objectInstances[zoneId][roomId][instanceId]
	if !ok {
		return nil
	}

	delete(w.objectInstances[zoneId][roomId], instanceId)
	return oi
}

func (w *WorldState) addObjectToRoom(zoneId, roomId storage.Identifier, obj *ObjectInstance) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.objectInstances[zoneId] == nil {
		w.objectInstances[zoneId] = make(map[storage.Identifier]map[string]*ObjectInstance)
	}
	if w.objectInstances[zoneId][roomId] == nil {
		w.objectInstances[zoneId][roomId] = make(map[string]*ObjectInstance)
	}

	w.objectInstances[zoneId][roomId][obj.InstanceId] = obj
}

// RoomObjectHolder wraps a specific room's object storage so it satisfies
// the ObjectHolder interface defined in the commands package.
type RoomObjectHolder struct {
	world  *WorldState
	zoneId storage.Identifier
	roomId storage.Identifier
}

// RoomHolder creates a RoomObjectHolder for the given room.
func (w *WorldState) RoomHolder(zoneId, roomId storage.Identifier) *RoomObjectHolder {
	return &RoomObjectHolder{world: w, zoneId: zoneId, roomId: roomId}
}

// Add places an object instance in this room.
func (r *RoomObjectHolder) Add(obj *ObjectInstance) {
	r.world.addObjectToRoom(r.zoneId, r.roomId, obj)
}

// Remove removes an object instance from this room by instance ID.
func (r *RoomObjectHolder) Remove(instanceId string) *ObjectInstance {
	return r.world.removeObjectFromRoom(r.zoneId, r.roomId, instanceId)
}

// ResetZone despawns all mobiles in a zone and respawns them per room definitions.
// If force is true, bypasses time/occupancy checks and resets immediately.
func (w *WorldState) ResetZone(zoneId storage.Identifier, force bool) {
	zone := w.zones.Get(string(zoneId))
	if zone == nil {
		slog.Warn("zone not found during reset", "zone", zoneId)
		return
	}

	now := time.Now()

	// Unless forced, check if reset conditions are met
	if !force {
		if zone.ResetMode == ZoneResetNever {
			return
		}
		if now.Before(zone.NextReset) {
			return
		}
		if zone.ResetMode == ZoneResetEmpty && w.isZoneOccupied(zoneId) {
			return
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Clear all mobile and object instances in this zone
	delete(w.mobileInstances, zoneId)
	delete(w.objectInstances, zoneId)

	slog.Info("resetting zone", "zone", zoneId, "rooms", len(w.roomsByZone[zoneId]))

	// Respawn mobiles and objects for each room in the zone
	for _, roomId := range w.roomsByZone[zoneId] {
		room := w.rooms.Get(string(roomId))
		if room == nil {
			slog.Warn("room not found during zone reset", "zone", zoneId, "room", roomId)
			continue
		}

		// Spawn mobiles
		for _, mobileId := range room.MobSpawns {
			w.SpawnMobileInstance(storage.Identifier(mobileId), zoneId, roomId)
		}

		// Spawn objects
		for _, spawn := range room.ObjSpawns {
			if w.objectInstances[zoneId] == nil {
				w.objectInstances[zoneId] = make(map[storage.Identifier]map[string]*ObjectInstance)
			}
			if w.objectInstances[zoneId][roomId] == nil {
				w.objectInstances[zoneId][roomId] = make(map[string]*ObjectInstance)
			}
			instance := spawnObjectInstance(spawn)
			w.objectInstances[zoneId][roomId][instance.InstanceId] = instance
			slog.Debug("spawned object", "object", spawn.ObjectId, "zone", zoneId, "room", roomId)
		}
	}

	// Schedule next reset
	if zone.LifespanDuration() > 0 {
		zone.NextReset = now.Add(zone.LifespanDuration())
	}
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
		CharId:       charId,
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

// Subscriber provides the ability to subscribe to message subjects
type Subscriber interface {
	Subscribe(subject string, handler func(data []byte)) (unsubscribe func(), err error)
}

// Tick processes zone resets based on their reset mode and lifespan.
func (w *WorldState) Tick(ctx context.Context) error {
	//

	// Reset pending zones
	for zoneId := range w.zones.GetAll() {
		w.ResetZone(zoneId, false)
	}

	return nil
}

// isZoneOccupied returns true if any players are in the zone.
func (w *WorldState) isZoneOccupied(zoneId storage.Identifier) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for _, p := range w.players {
		if p.ZoneId == zoneId {
			return true
		}
	}
	return false
}

// PlayerState holds all mutable state for an active player.
type PlayerState struct {
	subscriber Subscriber
	msgs       chan []byte

	CharId storage.Identifier

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
