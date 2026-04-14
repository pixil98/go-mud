package game

import (
	"context"
	"sync"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// WorldState is the single source of truth for all mutable game state.
// All access must go through its methods to ensure thread-safety.
type WorldState struct {
	mu         sync.RWMutex
	subscriber Subscriber
	players    map[string]*CharacterInstance

	zones            map[string]*ZoneInstance
	perks            *PerkCache
	commanderFactory CommanderFactory
}

// SetCommanderFactory sets the factory used to create per-actor Commanders
// at spawn time and when players join.
func (w *WorldState) SetCommanderFactory(f CommanderFactory) {
	w.commanderFactory = f
}

// ResetAll resets all zones, spawning mobs and objects.
func (w *WorldState) ResetAll() error {
	for _, zi := range w.zones {
		if err := zi.Reset(true, w.commanderFactory); err != nil {
			return err
		}
	}
	return nil
}

// NewWorldState creates a new WorldState with zone and room instances initialized.
func NewWorldState(sub Subscriber, zones storage.Storer[*assets.Zone], rooms storage.Storer[*assets.Room]) (*WorldState, error) {
	worldPerks := NewPerkCache(nil, nil)

	w := &WorldState{
		subscriber: sub,
		players:    make(map[string]*CharacterInstance),
		zones:      make(map[string]*ZoneInstance),
		perks:      worldPerks,
	}

	// Build zone instances
	for zoneId, zone := range zones.GetAll() {
		zi, err := NewZoneInstance(storage.NewResolvedSmartIdentifier(string(zoneId), zone), w)
		if err != nil {
			return nil, err
		}
		zi.Perks.AddSource("world", worldPerks)
		w.zones[zoneId] = zi
	}

	// Build room instances and add to their zones
	for roomId, room := range rooms.GetAll() {
		zoneId := room.Zone.Id()
		if zi, ok := w.zones[zoneId]; ok {
			ri, err := NewRoomInstance(storage.NewResolvedSmartIdentifier(string(roomId), room))
			if err != nil {
				return nil, err
			}
			zi.AddRoom(ri)
		}
	}

	// Resolve exit destination pointers now that all rooms exist.
	for _, zi := range w.zones {
		for _, ri := range zi.rooms {
			for _, re := range ri.exits {
				destZoneId := re.Exit.Zone.Id()
				if destZoneId == "" {
					destZoneId = zi.Zone.Id()
				}
				if destZi, ok := w.zones[destZoneId]; ok {
					re.Dest = destZi.rooms[re.Exit.Room.Id()]
				}
			}
		}
	}

	return w, nil
}

// Perks returns the world-level perk cache.
func (w *WorldState) Perks() *PerkCache {
	return w.perks
}

// Instances returns all zone instances.
func (w *WorldState) Instances() map[string]*ZoneInstance {
	return w.zones
}

// GetZone returns the zone instance for the given zone ID.
// Returns nil if the zone is not found.
func (w *WorldState) GetZone(zoneId string) *ZoneInstance {
	return w.zones[zoneId]
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
	charId := ci.Id()
	if _, exists := w.players[charId]; exists {
		w.mu.Unlock()
		return ErrPlayerExists
	}
	ci.subscriber = w.subscriber
	w.players[charId] = ci
	room := ci.Room()
	ci.AddSource("room", room.Perks)
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

	room := ps.Room()
	delete(w.players, charId)
	w.mu.Unlock()

	room.RemovePlayer(charId)
	return nil
}

// SetPlayerQuit sets the quit flag for a player.
func (w *WorldState) SetPlayerQuit(charId string, quit bool) error {
	w.mu.RLock()
	p, exists := w.players[charId]
	w.mu.RUnlock()
	if !exists {
		return ErrPlayerNotFound
	}

	p.SetQuit(quit)
	return nil
}

// MarkPlayerActive resets the player's idle timer.
func (w *WorldState) MarkPlayerActive(charId string) {
	w.mu.RLock()
	p, ok := w.players[charId]
	w.mu.RUnlock()
	if ok {
		p.MarkActive()
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

// Tick processes zone resets and ticks the full hierarchy:
// world perks → players → zones → rooms → mobs.
func (w *WorldState) Tick(ctx context.Context) error {
	for _, zi := range w.zones {
		if err := zi.Reset(false, w.commanderFactory); err != nil {
			return err
		}
	}

	w.perks.Tick()
	w.ForEachPlayer(func(_ string, ps *CharacterInstance) {
		ps.Tick(ctx)
	})
	for _, zi := range w.zones {
		zi.Tick()
	}
	w.tickMobs(ctx)

	w.ForEachPlayer(func(_ string, ci *CharacterInstance) {
		ci.flushTickMessages()
	})

	return nil
}

// tickMobs snapshots all mobs in the world and ticks each one once.
// A world-level snapshot prevents double-ticking when mobs wander across
// rooms or zones during their tick.
func (w *WorldState) tickMobs(ctx context.Context) {
	var mobs []*MobileInstance
	for _, zi := range w.zones {
		zi.ForEachRoom(func(_ string, ri *RoomInstance) {
			ri.ForEachMob(func(mi *MobileInstance) {
				mobs = append(mobs, mi)
			})
		})
	}
	for _, mi := range mobs {
		mi.Tick(ctx)
	}
}
