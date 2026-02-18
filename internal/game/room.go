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

// ObjectSpawn defines an object to spawn in a room during zone reset.
// Contents lists objects to spawn inside this container (requires the container flag).
// Supports nesting — content items can themselves be containers with contents.
type ObjectSpawn struct {
	ObjectId string        `json:"object_id"`
	Contents []ObjectSpawn `json:"contents,omitempty"`
}

// Exit defines a destination for movement from a room.
type Exit struct {
	ZoneId string `json:"zone_id,omitempty"` // Optional; defaults to current zone
	RoomId string `json:"room_id"`
}

// Room represents a location within a zone.
type Room struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	ZoneId      string          `json:"zone_id"`
	Exits       map[string]Exit `json:"exits"`                   // direction -> destination
	MobSpawns   []string        `json:"mobile_spawns,omitempty"` // mobile IDs to spawn; list duplicates for multiple
	ObjSpawns   []ObjectSpawn   `json:"object_spawns,omitempty"` // objects to spawn
}

// Validate satisfies storage.ValidatingSpec.
// TODO: Add cross-reference validation to ensure:
// - The room's zone_id references an existing zone
// - All exit destinations (zone_id + room_id) reference existing rooms
// This would require access to the zone and room stores, possibly via a
// ValidateWithStores(zones, rooms) method or a post-load validation pass.
func (r *Room) Validate() error {
	el := errors.NewErrorList()

	if r.Name == "" {
		el.Add(fmt.Errorf("room name is required"))
	}
	if r.ZoneId == "" {
		el.Add(fmt.Errorf("zone_id is required"))
	}

	for dir, exit := range r.Exits {
		if exit.RoomId == "" {
			el.Add(fmt.Errorf("exit %s: room_id is required", dir))
		}
	}

	return el.Err()
}

// RoomInstance holds the runtime state for a room — spawned mobs, objects, and players.
// The mutex protects the players and mobiles maps. Object operations delegate to
// the objects Inventory, which handles its own locking.
type RoomInstance struct {
	RoomId     storage.Identifier
	Definition *Room

	mu      sync.RWMutex
	mobiles map[string]*MobileInstance
	objects *Inventory
	players map[storage.Identifier]*PlayerState
}

// NewRoomInstance creates an empty RoomInstance.
func NewRoomInstance(roomId storage.Identifier, def *Room) *RoomInstance {
	return &RoomInstance{
		RoomId:     roomId,
		Definition: def,
		mobiles:    make(map[string]*MobileInstance),
		objects:    NewInventory(),
		players:    make(map[storage.Identifier]*PlayerState),
	}
}

// Reset clears all mobs and objects and respawns them from the room definition.
// Players are preserved.
func (ri *RoomInstance) Reset(mobiles storage.Storer[*Mobile], objects storage.Storer[*Object]) {
	ri.mu.Lock()
	ri.mobiles = make(map[string]*MobileInstance)
	for _, mobileId := range ri.Definition.MobSpawns {
		ri.spawnMob(storage.Identifier(mobileId), mobiles.Get(mobileId))
	}
	ri.mu.Unlock()

	ri.objects.Clear()
	for _, spawn := range ri.Definition.ObjSpawns {
		ri.AddObj(ri.spawnObj(spawn, objects))
	}
}

// FindMob searches room mobs for one whose definition matches the given name.
func (ri *RoomInstance) FindMob(name string) *MobileInstance {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	for _, mi := range ri.mobiles {
		if mi.Definition.MatchName(name) {
			return mi
		}
	}
	return nil
}

// spawnMob creates a new MobileInstance and adds it to the room.
// Caller must hold the write lock.
func (ri *RoomInstance) spawnMob(mobileId storage.Identifier, def *Mobile) *MobileInstance {
	mi := &MobileInstance{
		InstanceId: uuid.New().String(),
		MobileId:   mobileId,
		Definition: def,
	}
	ri.mobiles[mi.InstanceId] = mi
	return mi
}

// spawnObj creates an ObjectInstance from an ObjectSpawn,
// recursively spawning any contents for containers.
func (ri *RoomInstance) spawnObj(spawn ObjectSpawn, objDefs storage.Storer[*Object]) *ObjectInstance {
	instance := NewObjectInstance(storage.Identifier(spawn.ObjectId), objDefs.Get(spawn.ObjectId))
	for _, contentSpawn := range spawn.Contents {
		instance.Contents.AddObj(ri.spawnObj(contentSpawn, objDefs))
	}
	return instance
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
	sb.WriteString(ri.Definition.Name)
	sb.WriteString("\n")
	sb.WriteString(ri.Definition.Description)
	sb.WriteString("\n")

	// Show objects
	for _, oi := range ri.objects.Items {
		if oi.Definition == nil {
			continue
		}
		if oi.Definition.LongDesc != "" {
			sb.WriteString(oi.Definition.LongDesc)
		} else {
			sb.WriteString(fmt.Sprintf("%s is here.", oi.Definition.ShortDesc))
		}
		sb.WriteString("\n")
	}

	ri.mu.RLock()
	// Show mobs
	for _, mi := range ri.mobiles {
		if mi.Definition == nil {
			continue
		}
		if mi.Definition.LongDesc != "" {
			sb.WriteString(mi.Definition.LongDesc)
		} else {
			sb.WriteString(fmt.Sprintf("%s is here.", mi.Definition.ShortDesc))
		}
		sb.WriteString("\n")
	}

	// Show other players
	for _, ps := range ri.players {
		if ps.Character != nil && ps.Character.Name != actorName {
			sb.WriteString(fmt.Sprintf("%s is here.\n", ps.Character.Name))
		}
	}
	ri.mu.RUnlock()

	sb.WriteString("\n")
	sb.WriteString(formatExits(ri.Definition.Exits))

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
