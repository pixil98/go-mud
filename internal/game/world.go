package game

import (
	"context"
	"sync"
	"time"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// WorldState is the single source of truth for all mutable game state.
// All access must go through its methods to ensure thread-safety.
type WorldState struct {
	mu         sync.RWMutex
	subscriber Subscriber
	players    map[string]*CharacterInstance

	instances map[string]*ZoneInstance
}

// NewWorldState creates a new WorldState with zone and room instances initialized.
func NewWorldState(sub Subscriber, zones storage.Storer[*assets.Zone], rooms storage.Storer[*assets.Room]) (*WorldState, error) {
	// Build zone instances
	instances := make(map[string]*ZoneInstance)
	for zoneId, zone := range zones.GetAll() {
		zi, err := NewZoneInstance(storage.NewResolvedSmartIdentifier(string(zoneId), zone))
		if err != nil {
			return nil, err
		}
		instances[zoneId] = zi
	}

	// Build room instances and add to their zones
	for roomId, room := range rooms.GetAll() {
		zoneId := room.Zone.Id()
		if zi, ok := instances[zoneId]; ok {
			ri, err := NewRoomInstance(storage.NewResolvedSmartIdentifier(string(roomId), room))
			if err != nil {
				return nil, err
			}
			zi.AddRoom(ri)
		}
	}

	return &WorldState{
		subscriber: sub,
		players:    make(map[string]*CharacterInstance),
		instances:  instances,
	}, nil
}

// Instances returns all zone instances.
func (w *WorldState) Instances() map[string]*ZoneInstance {
	return w.instances
}

// GetZone returns the zone instance for the given zone ID.
// Returns nil if the zone is not found.
func (w *WorldState) GetZone(zoneId string) *ZoneInstance {
	return w.instances[zoneId]
}

// GetRoom returns the room instance for the given zone and room IDs.
// Returns nil if the zone or room is not found.
func (w *WorldState) GetRoom(zoneId, roomId string) *RoomInstance {
	zi := w.instances[zoneId]
	if zi == nil {
		return nil
	}
	return zi.GetRoom(roomId)
}

// GetPlayer returns the player state. Returns nil if player not found.
func (w *WorldState) GetPlayer(charId string) *CharacterInstance {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.players[charId]
}

// AddPlayer registers a CharacterInstance in the world state and places them in their room.
func (w *WorldState) AddPlayer(ci *CharacterInstance) error {
	w.mu.Lock()
	charId := ci.Character.Id()
	if _, exists := w.players[charId]; exists {
		w.mu.Unlock()
		return ErrPlayerExists
	}
	ci.subscriber = w.subscriber
	w.players[charId] = ci
	room := w.instances[ci.ZoneId].GetRoom(ci.RoomId)
	w.mu.Unlock()

	room.AddPlayer(charId, ci)
	return nil
}

// RemovePlayer removes a player from the world state and from the room instance.
func (w *WorldState) RemovePlayer(charId string) error {
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
func (w *WorldState) SetPlayerQuit(charId string, quit bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, exists := w.players[charId]
	if !exists {
		return ErrPlayerNotFound
	}

	p.Quit = quit
	return nil
}

// MarkPlayerActive resets the player's idle timer.
func (w *WorldState) MarkPlayerActive(charId string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if p, ok := w.players[charId]; ok {
		p.LastActivity = time.Now()
	}
}

// ForEachPlayer calls fn for each player in the world while holding the lock.
func (w *WorldState) ForEachPlayer(fn func(string, *CharacterInstance)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, ps := range w.players {
		fn(id, ps)
	}
}

// Subscriber provides the ability to subscribe to message subjects.
type Subscriber interface {
	Subscribe(subject string, handler func(data []byte)) (unsubscribe func(), err error)
}

// Tick processes zone resets and regenerates out-of-combat entities.
func (w *WorldState) Tick(ctx context.Context) error {
	for _, zi := range w.instances {
		err := zi.Reset(false, w.instances)
		if err != nil {
			return err
		}
	}

	// Regenerate out-of-combat entities.
	w.ForEachPlayer(func(_ string, ps *CharacterInstance) {
		if !ps.InCombat && ps.CurrentHP < ps.MaxHP {
			ps.Regenerate(1)
		}
	})
	for _, zi := range w.instances {
		for _, ri := range zi.rooms {
			ri.mu.RLock()
			for _, mi := range ri.mobiles {
				if !mi.InCombat && mi.CurrentHP < mi.MaxHP {
					mi.Regenerate(1)
				}
			}
			ri.mu.RUnlock()
		}
	}

	return nil
}
