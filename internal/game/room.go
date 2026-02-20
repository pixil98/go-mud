package game

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// Exit defines a destination for movement from a room.
type Exit struct {
	Zone storage.SmartIdentifier[*Zone] `json:"zone_id"` // Optional; defaults to current zone
	Room storage.SmartIdentifier[*Room] `json:"room_id"`
}

// Room represents a location within a zone.
type Room struct {
	Name        string                             `json:"name"`
	Description string                             `json:"description"`
	Zone        storage.SmartIdentifier[*Zone]     `json:"zone_id"`
	Exits       map[string]Exit                    `json:"exits"`         // direction -> destination
	MobSpawns   []storage.SmartIdentifier[*Mobile] `json:"mobile_spawns"` // mobile IDs to spawn; list duplicates for multiple
	ObjSpawns   []ObjectSpawn                      `json:"object_spawns"` // objects to spawn
}

// Validate a room definition
func (r *Room) Validate() error {
	el := errors.NewErrorList()

	if r.Name == "" {
		el.Add(fmt.Errorf("room name is required"))
	}
	el.Add(r.Zone.Validate())

	for dir, exit := range r.Exits {
		if err := exit.Room.Validate(); err != nil {
			el.Add(fmt.Errorf("exit %s: %w", dir, err))
		}
	}

	return el.Err()
}

func (r *Room) Resolve(dict *Dictionary) error {
	el := errors.NewErrorList()
	el.Add(r.Zone.Resolve(dict.Zones))
	for dir, exit := range r.Exits {
		el.Add(exit.Room.Resolve(dict.Rooms))
		if exit.Zone.Id() != "" {
			el.Add(exit.Zone.Resolve(dict.Zones))
			r.Exits[dir] = exit
		}
	}

	for i := range r.MobSpawns {
		el.Add(r.MobSpawns[i].Resolve(dict.Mobiles))
	}
	for i := range r.ObjSpawns {
		el.Add(r.ObjSpawns[i].Resolve(dict.Objects))
	}
	return el.Err()
}

// RoomInstance holds the runtime state for a room — spawned mobs, objects, and players.
// The mutex protects the players and mobiles maps. Object operations delegate to
// the objects Inventory, which handles its own locking.
type RoomInstance struct {
	Room storage.SmartIdentifier[*Room]

	mu      sync.RWMutex
	mobiles map[string]*MobileInstance
	objects *Inventory
	players map[storage.Identifier]*PlayerState
}

// NewRoomInstance creates a RoomInstance from a resolved SmartIdentifier.
func NewRoomInstance(room storage.SmartIdentifier[*Room]) (*RoomInstance, error) {
	if room.Get() == nil {
		return nil, fmt.Errorf("unable to create instance from unresolved room %q", room.Id())
	}
	return &RoomInstance{
		Room:    room,
		mobiles: make(map[string]*MobileInstance),
		objects: NewInventory(),
		players: make(map[storage.Identifier]*PlayerState),
	}, nil
}

// Reset clears all mobs and objects and respawns them from the room definition.
// Players are preserved.
func (ri *RoomInstance) Reset() error {
	def := ri.Room.Get()
	ri.mu.Lock()
	ri.mobiles = make(map[string]*MobileInstance)
	for _, mob := range def.MobSpawns {
		ri.spawnMob(mob)
	}
	ri.mu.Unlock()

	ri.objects.Clear()
	for _, spawn := range def.ObjSpawns {
		oi, err := spawn.Spawn()
		if err != nil {
			return fmt.Errorf("resetting room %q: %w", ri.Room.Id(), err)
		}
		ri.AddObj(oi)
	}
	return nil
}

// FindMob searches room mobs for one whose definition matches the given name.
func (ri *RoomInstance) FindMob(name string) *MobileInstance {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	for _, mi := range ri.mobiles {
		if mi.Mobile.Get().MatchName(name) {
			return mi
		}
	}
	return nil
}

// spawnMob creates a new MobileInstance and adds it to the room.
// Caller must hold the write lock.
func (ri *RoomInstance) spawnMob(mob storage.SmartIdentifier[*Mobile]) (*MobileInstance, error) {
	mi := &MobileInstance{
		InstanceId: uuid.New().String(),
		Mobile:     mob,
		ActorInstance: ActorInstance{
			Inventory: NewInventory(),
			Equipment: NewEquipment(),
		},
	}
	def := mob.Get()
	for _, spawn := range def.Inventory {
		oi, err := spawn.Spawn()
		if err != nil {
			return nil, fmt.Errorf("spawning %q: %w", mob.Id(), err)
		}
		mi.Inventory.AddObj(oi)
	}
	for slot, spawn := range def.Equipment {
		oi, err := spawn.Spawn()
		if err != nil {
			return nil, fmt.Errorf("spawning %q: %w", mob.Id(), err)
		}
		mi.Equipment.Equip(slot, 0, oi)
	}
	ri.mobiles[mi.InstanceId] = mi
	return mi, nil
}

// FindObj searches room objects for one whose definition matches the given name.
func (ri *RoomInstance) FindObj(name string) *ObjectInstance {
	return ri.objects.FindObj(name)
}

// Add places an object instance in this room.
func (ri *RoomInstance) AddObj(obj *ObjectInstance) {
	ri.objects.AddObj(obj)
}

// Remove removes an object instance from this room by instance ID.
func (ri *RoomInstance) RemoveObj(instanceId string) *ObjectInstance {
	return ri.objects.RemoveObj(instanceId)
}

// FindPlayer searches room players for one whose character name matches the given name.
func (ri *RoomInstance) FindPlayer(name string) *PlayerState {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	for _, ps := range ri.players {
		if ps.Character.MatchName(name) {
			return ps
		}
	}
	return nil
}

// AddPlayer adds a player to the room.
func (ri *RoomInstance) AddPlayer(charId storage.Identifier, ps *PlayerState) {
	ri.mu.Lock()
	defer ri.mu.Unlock()

	ri.players[charId] = ps
}

// RemovePlayer removes a player from the room.
func (ri *RoomInstance) RemovePlayer(charId storage.Identifier) {
	ri.mu.Lock()
	defer ri.mu.Unlock()

	delete(ri.players, charId)
}

// Describe returns the full room description including objects, mobs, players, and exits.
// actorName is excluded from the player list.
func (ri *RoomInstance) Describe(actorName string) string {
	var sb strings.Builder
	def := ri.Room.Get()
	sb.WriteString(def.Name)
	sb.WriteString("\n")
	sb.WriteString(def.Description)
	sb.WriteString("\n")

	// Show objects
	for _, oi := range ri.objects.Objs {
		if oi.Object.Get().LongDesc != "" {
			sb.WriteString(oi.Object.Get().LongDesc)
		} else {
			sb.WriteString(fmt.Sprintf("%s is here.", oi.Object.Get().ShortDesc))
		}
		sb.WriteString("\n")
	}

	ri.mu.RLock()
	// Show mobs
	for _, mi := range ri.mobiles {
		if mi.Mobile.Get().LongDesc != "" {
			sb.WriteString(mi.Mobile.Get().LongDesc)
		} else {
			sb.WriteString(fmt.Sprintf("%s is here.", mi.Mobile.Get().ShortDesc))
		}
		sb.WriteString("\n")
	}

	// Show other players
	for _, ps := range ri.players {
		if ps.Character.Name != actorName {
			sb.WriteString(fmt.Sprintf("%s is here.\n", ps.Character.Name))
		}
	}
	ri.mu.RUnlock()

	sb.WriteString("\n")
	sb.WriteString(formatExits(def.Exits))

	return sb.String()
}

// PlayerCount returns the number of players in the room.
func (ri *RoomInstance) PlayerCount() int {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	return len(ri.players)
}

func formatExits(exits map[string]Exit) string {
	if len(exits) == 0 {
		return "[Exits: none]"
	}
	dirs := make([]string, 0, len(exits))
	for dir := range exits {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return fmt.Sprintf("[Exits: %s]", strings.Join(dirs, ", "))
}
