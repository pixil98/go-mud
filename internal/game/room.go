package game

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/storage"
)

// Exit defines a destination for movement from a room.
type Exit struct {
	Zone    storage.SmartIdentifier[*Zone] `json:"zone_id"` // Optional; defaults to current zone
	Room    storage.SmartIdentifier[*Room] `json:"room_id"`
	Closure *Closure                       `json:"closure,omitempty"` // Optional open/close/lock barrier
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
		if exit.Closure != nil {
			if exit.Closure.Name == "" {
				el.Add(fmt.Errorf("exit %s closure: name is required", dir))
			}
			if err := exit.Closure.Validate(); err != nil {
				el.Add(fmt.Errorf("exit %s closure: %w", dir, err))
			}
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
		}
		if exit.Closure != nil {
			el.Add(exit.Closure.Resolve(dict.Objects))
		}
		r.Exits[dir] = exit
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
// The mutex protects the players, mobiles, and exit closure state. Object operations
// delegate to the objects Inventory, which handles its own locking.
type RoomInstance struct {
	Room storage.SmartIdentifier[*Room]

	mu         sync.RWMutex
	mobiles    map[string]*MobileInstance
	objects    *Inventory
	players    map[storage.Identifier]*PlayerState
	exitClosed map[string]bool // runtime closed state for exits with a Closure
	exitLocked map[string]bool // runtime locked state for exits with a Lock
}

// NewRoomInstance creates a RoomInstance from a resolved SmartIdentifier.
func NewRoomInstance(room storage.SmartIdentifier[*Room]) (*RoomInstance, error) {
	if room.Get() == nil {
		return nil, fmt.Errorf("unable to create instance from unresolved room %q", room.Id())
	}
	ri := &RoomInstance{
		Room:       room,
		mobiles:    make(map[string]*MobileInstance),
		objects:    NewInventory(),
		players:    make(map[storage.Identifier]*PlayerState),
		exitClosed: make(map[string]bool),
		exitLocked: make(map[string]bool),
	}
	ri.initExitClosures()
	return ri, nil
}

// initExitClosures populates exitClosed/exitLocked from exit definitions.
// Caller must hold the write lock or call before the instance is shared.
func (ri *RoomInstance) initExitClosures() {
	ri.exitClosed = make(map[string]bool)
	ri.exitLocked = make(map[string]bool)
	for dir, exit := range ri.Room.Get().Exits {
		if exit.Closure != nil {
			ri.exitClosed[dir] = exit.Closure.Closed
			if exit.Closure.Lock != nil {
				ri.exitLocked[dir] = exit.Closure.Lock.Locked
			}
		}
	}
}

// IsExitClosed returns whether the exit in the given direction is closed.
// Returns false if the direction has no closure.
func (ri *RoomInstance) IsExitClosed(direction string) bool {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	return ri.exitClosed[direction]
}

// SetExitClosed sets the closed state for a direction.
func (ri *RoomInstance) SetExitClosed(direction string, closed bool) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.exitClosed[direction] = closed
}

// IsExitLocked returns whether the exit in the given direction is locked.
// Returns false if the direction has no lock.
func (ri *RoomInstance) IsExitLocked(direction string) bool {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	return ri.exitLocked[direction]
}

// SetExitLocked sets the locked state for a direction.
func (ri *RoomInstance) SetExitLocked(direction string, locked bool) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.exitLocked[direction] = locked
}

// ExitClosure returns the Closure definition for the given direction, or nil.
func (ri *RoomInstance) ExitClosure(direction string) *Closure {
	exit, ok := ri.Room.Get().Exits[direction]
	if !ok {
		return nil
	}
	return exit.Closure
}

// FindOtherSide looks up the destination room of an exit and finds the
// reverse exit that leads back to sourceZone/sourceRoom.
// Returns the destination RoomInstance and the direction of the reverse exit,
// or (nil, "") if no matching reverse exit with a closure is found.
func FindOtherSide(exit Exit, sourceZone, sourceRoom storage.Identifier, instances map[storage.Identifier]*ZoneInstance) (*RoomInstance, string) {
	destZone := storage.Identifier(exit.Zone.Id())
	if destZone == "" {
		destZone = sourceZone
	}
	zi, ok := instances[destZone]
	if !ok {
		return nil, ""
	}
	destRoomInst := zi.GetRoom(storage.Identifier(exit.Room.Id()))
	if destRoomInst == nil {
		return nil, ""
	}

	for dir, otherExit := range destRoomInst.Room.Get().Exits {
		if otherExit.Closure == nil {
			continue
		}
		otherDestZone := storage.Identifier(otherExit.Zone.Id())
		if otherDestZone == "" {
			otherDestZone = destZone
		}
		if otherDestZone == sourceZone && storage.Identifier(otherExit.Room.Id()) == sourceRoom {
			return destRoomInst, dir
		}
	}
	return nil, ""
}

// Reset clears all mobs and objects and respawns them from the room definition.
// Players are preserved. Exit closure state is restored to definition defaults.
// If instances is non-nil, cross-zone door state is also synchronized.
func (ri *RoomInstance) Reset(instances map[storage.Identifier]*ZoneInstance) error {
	ri.mu.Lock()
	ri.initExitClosures()

	// Synchronize the other side of any cross-zone exits.
	if instances != nil {
		def := ri.Room.Get()
		thisZone := storage.Identifier(def.Zone.Id())
		thisRoom := storage.Identifier(ri.Room.Id())

		for _, exit := range def.Exits {
			if exit.Closure == nil {
				continue
			}
			if exit.Closure.Closed == false {
				continue
			}
			destZone := storage.Identifier(exit.Zone.Id())
			if destZone == "" || destZone == thisZone {
				continue // same zone, handled by its own reset
			}
			if otherRoom, otherDir := FindOtherSide(exit, thisZone, thisRoom, instances); otherRoom != nil {
				otherExit := otherRoom.Room.Get().Exits[otherDir]
				otherRoom.SetExitClosed(otherDir, otherExit.Closure.Closed)
				if otherExit.Closure.Lock != nil {
					otherRoom.SetExitLocked(otherDir, otherExit.Closure.Lock.Locked)
				}
			}
		}
	}

	def := ri.Room.Get()
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

// FindExit looks up an exit by direction key or closure name (case-insensitive).
// Returns the direction key and a pointer to the exit, or ("", nil) if not found.
func (ri *RoomInstance) FindExit(name string) (string, *Exit) {
	name = strings.ToLower(name)
	// Match by direction key first
	if exit, ok := ri.Room.Get().Exits[name]; ok {
		return name, &exit
	}
	// Fall back to matching by closure name
	for dir, exit := range ri.Room.Get().Exits {
		if exit.Closure != nil && strings.EqualFold(exit.Closure.Name, name) {
			return dir, &exit
		}
	}
	return "", nil
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
	def := mob.Get()
	mi := &MobileInstance{
		InstanceId: uuid.New().String(),
		Mobile:     mob,
		ActorInstance: ActorInstance{
			Inventory: NewInventory(),
			Equipment: NewEquipment(),
			MaxHP:     def.MaxHP,
			CurrentHP: def.MaxHP,
		},
	}
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
	// Snapshot exit closure state under the lock
	ri.mu.RLock()
	exitClosed := make(map[string]bool, len(ri.exitClosed))
	maps.Copy(exitClosed, ri.exitClosed)
	exitLocked := make(map[string]bool, len(ri.exitLocked))
	maps.Copy(exitLocked, ri.exitLocked)
	ri.mu.RUnlock()

	var sb strings.Builder
	def := ri.Room.Get()
	sb.WriteString(display.Colorize(display.Color.Yellow, def.Name))
	sb.WriteString("\n")
	sb.WriteString(display.Wrap(def.Description))
	sb.WriteString("\n")
	sb.WriteString(display.Colorize(display.Color.Cyan, formatExits(def.Exits, exitClosed, exitLocked)))
	sb.WriteString("\n")

	// Show objects
	for _, oi := range ri.objects.Objs {
		desc := oi.Object.Get().LongDesc
		if desc == "" {
			desc = fmt.Sprintf("%s is here.", oi.Object.Get().ShortDesc)
		}
		sb.WriteString(fmt.Sprintf("%s\n", display.Colorize(display.Color.Green, desc)))
	}

	ri.mu.RLock()
	// Show mobs
	for _, mi := range ri.mobiles {
		desc := mi.Mobile.Get().LongDesc
		if desc == "" {
			desc = fmt.Sprintf("%s is here.", mi.Mobile.Get().ShortDesc)
		}
		sb.WriteString(fmt.Sprintf("%s%s\n", display.Colorize(display.Color.Yellow, desc), formatFlags(mi.Flags())))
	}

	// Show other players
	for _, ps := range ri.players {
		if ps.Character.Name != actorName {
			sb.WriteString(display.Colorize(display.Color.Yellow, fmt.Sprintf("%s is here.%s\n", ps.Character.Name, formatFlags(ps.Flags()))))
		}
	}
	ri.mu.RUnlock()

	return strings.TrimRight(sb.String(), "\n")
}

// RemoveMob removes a mobile instance from the room by instance ID.
// Returns the removed instance, or nil if not found.
func (ri *RoomInstance) RemoveMob(instanceId string) *MobileInstance {
	ri.mu.Lock()
	defer ri.mu.Unlock()

	if mi, ok := ri.mobiles[instanceId]; ok {
		delete(ri.mobiles, instanceId)
		return mi
	}
	return nil
}

// ForEachPlayer calls fn for each player in the room while holding the lock.
func (ri *RoomInstance) ForEachPlayer(fn func(storage.Identifier, *PlayerState)) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	for id, ps := range ri.players {
		fn(id, ps)
	}
}

// PlayerCount returns the number of players in the room.
func (ri *RoomInstance) PlayerCount() int {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	return len(ri.players)
}

func formatFlags(flags []string) string {
	var s string
	for _, f := range flags {
		s += " (" + f + ")"
	}
	return s
}

func formatExits(exits map[string]Exit, exitClosed, exitLocked map[string]bool) string {
	if len(exits) == 0 {
		return "[Exits: none]"
	}
	dirs := make([]string, 0, len(exits))
	for dir := range exits {
		label := dir
		if exitLocked[dir] {
			label += " (locked)"
		} else if exitClosed[dir] {
			label += " (closed)"
		}
		dirs = append(dirs, label)
	}
	sort.Strings(dirs)
	return fmt.Sprintf("[Exits: %s]", strings.Join(dirs, ", "))
}
