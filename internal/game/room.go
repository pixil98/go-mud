package game

import (
	"fmt"
	"log/slog"
	"maps"
	"sort"
	"strings"
	"sync"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/storage"
)

// RoomInstance holds the runtime state for a room — spawned mobs, objects, and players.
// The mutex protects the players, mobiles, and exit closure state. Object operations
// delegate to the objects Inventory, which handles its own locking.
type RoomInstance struct {
	Room storage.SmartIdentifier[*assets.Room]

	mu         sync.RWMutex
	mobiles    map[string]*MobileInstance
	objects    *Inventory
	players    map[string]*CharacterInstance
	exitClosed map[string]bool // runtime closed state for exits with a Closure
	exitLocked map[string]bool // runtime locked state for exits with a Lock

	Perks *PerkCache
}

// NewRoomInstance creates a RoomInstance from a resolved SmartIdentifier.
func NewRoomInstance(room storage.SmartIdentifier[*assets.Room]) (*RoomInstance, error) {
	if room.Get() == nil {
		return nil, fmt.Errorf("unable to create instance from unresolved room %q", room.Id())
	}
	ri := &RoomInstance{
		Room:       room,
		mobiles:    make(map[string]*MobileInstance),
		objects:    NewInventory(),
		players:    make(map[string]*CharacterInstance),
		exitClosed: make(map[string]bool),
		exitLocked: make(map[string]bool),
		Perks:      NewPerkCache(room.Get().Perks, nil),
	}
	ri.initExitClosures()
	return ri, nil
}

// Tick advances one game tick for the room: expires timed perks and ticks all mobs.
func (ri *RoomInstance) Tick() {
	ri.Perks.Tick()
	ri.objects.Tick()
	ri.mu.RLock()
	for _, mi := range ri.mobiles {
		mi.Tick()
	}
	ri.mu.RUnlock()
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
func (ri *RoomInstance) ExitClosure(direction string) *assets.Closure {
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
func FindOtherSide(exit assets.Exit, sourceZone, sourceRoom string, instances map[string]*ZoneInstance) (*RoomInstance, string) {
	destZone := exit.Zone.Id()
	if destZone == "" {
		destZone = sourceZone
	}
	zi, ok := instances[destZone]
	if !ok {
		return nil, ""
	}
	destRoomInst := zi.GetRoom(exit.Room.Id())
	if destRoomInst == nil {
		return nil, ""
	}

	for dir, otherExit := range destRoomInst.Room.Get().Exits {
		if otherExit.Closure == nil {
			continue
		}
		otherDestZone := otherExit.Zone.Id()
		if otherDestZone == "" {
			otherDestZone = destZone
		}
		if otherDestZone == sourceZone && otherExit.Room.Id() == sourceRoom {
			return destRoomInst, dir
		}
	}
	return nil, ""
}

// Reset clears all mobs and objects and respawns them from the room definition.
// Players are preserved. Exit closure state is restored to definition defaults.
// If instances is non-nil, cross-zone door state is also synchronized.
func (ri *RoomInstance) Reset(instances map[string]*ZoneInstance) error {
	ri.mu.Lock()
	ri.initExitClosures()

	// Synchronize the other side of any cross-zone exits.
	if instances != nil {
		def := ri.Room.Get()
		thisZone := def.Zone.Id()
		thisRoom := ri.Room.Id()

		for _, exit := range def.Exits {
			if exit.Closure == nil {
				continue
			}
			if !exit.Closure.Closed {
				continue
			}
			destZone := exit.Zone.Id()
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
		mi, err := NewMobileInstance(mob)
		if err != nil {
			slog.Error("respawning mob", "mob", mob.Id(), "room", ri.Room.Id(), "error", err)
			continue
		}
		ri.addMob(mi)
	}
	ri.mu.Unlock()

	ri.objects.Clear()
	for _, spawn := range def.ObjSpawns {
		oi, err := SpawnObject(spawn)
		if err != nil {
			return fmt.Errorf("resetting room %q: %w", ri.Room.Id(), err)
		}
		ri.AddObj(oi)
	}
	return nil
}

// FindExit looks up an exit by direction key or closure name (case-insensitive).
// Returns the direction key and a pointer to the exit, or ("", nil) if not found.
func (ri *RoomInstance) FindExit(name string) (string, *assets.Exit) {
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
// Falls back to matching by instance ID if no name match is found.
func (ri *RoomInstance) FindMob(name string) *MobileInstance {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	for _, mi := range ri.mobiles {
		if mi.Mobile.Get().MatchName(name) {
			return mi
		}
	}
	// Fall back to instance ID lookup (used by combat target defaulting).
	return ri.mobiles[name]
}

// GetMob returns the MobileInstance with the given instanceId, or nil if not found.
func (ri *RoomInstance) GetMob(instanceId string) *MobileInstance {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	return ri.mobiles[instanceId]
}

// ForEachMob calls fn for each mob in the room while holding the lock.
func (ri *RoomInstance) ForEachMob(fn func(*MobileInstance)) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	for _, mi := range ri.mobiles {
		fn(mi)
	}
}

// AddMob places an existing MobileInstance into the room, setting its location.
func (ri *RoomInstance) AddMob(mi *MobileInstance) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.addMob(mi)
}

// addMob inserts a mob and sets its location. Caller must hold the write lock.
func (ri *RoomInstance) addMob(mi *MobileInstance) {
	mi.zoneId = ri.Room.Get().Zone.Id()
	mi.roomId = ri.Room.Id()
	ri.mobiles[mi.Id()] = mi
}

// FindObj searches room objects for one whose definition matches the given name.
func (ri *RoomInstance) FindObj(name string) *ObjectInstance {
	return ri.objects.FindObj(name)
}

// AddObj places an object instance in this room.
func (ri *RoomInstance) AddObj(obj *ObjectInstance) {
	ri.objects.AddObj(obj)
}

// RemoveObj removes an object instance from this room by instance ID.
func (ri *RoomInstance) RemoveObj(instanceId string) *ObjectInstance {
	return ri.objects.RemoveObj(instanceId)
}

// FindPlayer searches room players for one whose character name matches the given name.
func (ri *RoomInstance) FindPlayer(name string) *CharacterInstance {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	for _, ps := range ri.players {
		if ps.Character.Get().MatchName(name) {
			return ps
		}
	}
	return nil
}

// AddPlayer adds a player to the room.
func (ri *RoomInstance) AddPlayer(charId string, ps *CharacterInstance) {
	ri.mu.Lock()
	defer ri.mu.Unlock()

	ri.players[charId] = ps
}

// RemovePlayer removes a player from the room.
func (ri *RoomInstance) RemovePlayer(charId string) {
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
	ri.objects.ForEachObj(func(_ string, oi *ObjectInstance) {
		desc := oi.Object.Get().LongDesc
		if desc == "" {
			desc = fmt.Sprintf("%s is here.", oi.Object.Get().ShortDesc)
		}
		sb.WriteString(fmt.Sprintf("%s\n", display.Colorize(display.Color.Green, desc)))
	})

	ri.mu.RLock()
	// Show mobs
	for _, mi := range ri.mobiles {
		desc := mi.Mobile.Get().LongDesc
		if desc == "" {
			desc = fmt.Sprintf("%s is here.", mi.Name())
		}
		sb.WriteString(fmt.Sprintf("%s%s\n", display.Colorize(display.Color.Yellow, desc), formatFlags(mi.Flags())))
	}

	// Show other players
	for _, ps := range ri.players {
		if ps.Name() != actorName {
			sb.WriteString(fmt.Sprintf("%s%s\n", display.Colorize(display.Color.Yellow, fmt.Sprintf("%s is here.", ps.Name())), formatFlags(ps.Flags())))
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
func (ri *RoomInstance) ForEachPlayer(fn func(string, *CharacterInstance)) {
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
	var s strings.Builder
	for _, f := range flags {
		s.WriteString(" (" + f + ")")
	}
	return s.String()
}

func formatExits(exits map[string]assets.Exit, exitClosed, exitLocked map[string]bool) string {
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
