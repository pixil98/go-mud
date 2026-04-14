package game

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/storage"
)

// --- ResolvedExit ---

// ResolvedExit holds a resolved pointer to the destination room along with the
// original exit definition and runtime closure state.
type ResolvedExit struct {
	Dest   *RoomInstance // nil until resolved during world init
	Exit   assets.Exit   // original definition (closure, description, etc.)
	closed bool
	locked bool
}

// IsClosed returns whether the exit is closed.
func (re *ResolvedExit) IsClosed() bool { return re.closed }

// SetClosed sets the closed state.
func (re *ResolvedExit) SetClosed(v bool) { re.closed = v }

// IsLocked returns whether the exit is locked.
func (re *ResolvedExit) IsLocked() bool { return re.locked }

// SetLocked sets the locked state.
func (re *ResolvedExit) SetLocked(v bool) { re.locked = v }

// OtherSide finds the reverse exit on the destination room that leads back
// to source. Returns the direction and resolved exit, or ("", nil) if not found.
func (re *ResolvedExit) OtherSide(source *RoomInstance) (string, *ResolvedExit) {
	if re.Dest == nil {
		return "", nil
	}
	for dir, other := range re.Dest.exits {
		if other.Exit.Closure == nil {
			continue
		}
		if other.Dest == source {
			return dir, other
		}
	}
	return "", nil
}

// --- RoomInstance ---

// RoomInstance holds the runtime state for a room — spawned mobs, objects, and players.
// The mutex protects the players and mobiles maps. Object operations delegate to
// the objects Inventory, which handles its own locking.
type RoomInstance struct {
	Room storage.SmartIdentifier[*assets.Room]

	zone  *ZoneInstance
	exits map[string]*ResolvedExit // direction → resolved destination

	mu      sync.RWMutex
	mobiles map[string]*MobileInstance
	objects *Inventory
	players map[string]*CharacterInstance

	Perks *PerkCache
}

// NewRoomInstance creates a RoomInstance from a resolved SmartIdentifier.
// Exits are created with nil Dest pointers; call ResolveExits after all rooms exist.
func NewRoomInstance(room storage.SmartIdentifier[*assets.Room]) (*RoomInstance, error) {
	if room.Get() == nil {
		return nil, fmt.Errorf("unable to create instance from unresolved room %q", room.Id())
	}
	ri := &RoomInstance{
		Room:    room,
		exits:   make(map[string]*ResolvedExit),
		mobiles: make(map[string]*MobileInstance),
		objects: NewInventory(),
		players: make(map[string]*CharacterInstance),
		Perks:   NewPerkCache(room.Get().Perks, nil),
	}
	for dir, exit := range room.Get().Exits {
		ri.exits[dir] = &ResolvedExit{Exit: exit}
	}
	ri.initExitClosures()
	return ri, nil
}

// Tick advances one game tick for the room: expires timed perks and object decay.
func (ri *RoomInstance) Tick() {
	ri.Perks.Tick()
	ri.objects.Tick()
}

// Zone returns the zone this room belongs to.
func (ri *RoomInstance) Zone() *ZoneInstance {
	return ri.zone
}

// Reset clears all mobs and objects and respawns them from the room definition.
// Players are preserved. Exit closure state is restored to definition defaults.
// Cross-zone door state is also synchronized via resolved exit pointers.
func (ri *RoomInstance) Reset(cf CommanderFactory) error {
	ri.mu.Lock()
	ri.initExitClosures()

	// Synchronize the other side of any cross-zone exits.
	for _, re := range ri.exits {
		if re.Exit.Closure == nil || !re.Exit.Closure.Closed {
			continue
		}
		if re.Dest == nil || re.Dest.zone == ri.zone {
			continue // same zone, handled by its own reset
		}
		if _, other := re.OtherSide(ri); other != nil {
			other.closed = other.Exit.Closure.Closed
			if other.Exit.Closure.Lock != nil {
				other.locked = other.Exit.Closure.Lock.Locked
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
		if cf != nil {
			mi.commander = cf(mi)
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

// Describe returns the full room description including objects, mobs, players, and exits.
// actorName is excluded from the player list.
func (ri *RoomInstance) Describe(actorName string) string {
	var sb strings.Builder
	def := ri.Room.Get()
	sb.WriteString(display.Colorize(display.Color.Yellow, def.Name))
	sb.WriteString("\n")
	sb.WriteString(display.Wrap(def.Description))
	sb.WriteString("\n")
	sb.WriteString(display.Colorize(display.Color.Cyan, formatExits(ri.exits)))
	sb.WriteString("\n")

	ri.objects.ForEachObj(func(_ string, oi *ObjectInstance) {
		desc := oi.Object.Get().LongDesc
		if desc == "" {
			desc = fmt.Sprintf("%s is here.", oi.Object.Get().ShortDesc)
		}
		fmt.Fprintf(&sb, "%s\n", display.Colorize(display.Color.Green, desc))
	})

	ri.mu.RLock()
	for _, mi := range ri.mobiles {
		desc := mi.Mobile.Get().LongDesc
		if desc == "" {
			desc = fmt.Sprintf("%s is here.", mi.Name())
		}
		fmt.Fprintf(&sb, "%s%s\n", display.Colorize(display.Color.Yellow, desc), formatFlags(mi.Flags()))
	}
	for _, ps := range ri.players {
		if ps.Name() != actorName {
			fmt.Fprintf(&sb, "%s%s\n", display.Colorize(display.Color.Yellow, fmt.Sprintf("%s is here.", ps.Name())), formatFlags(ps.Flags()))
		}
	}
	ri.mu.RUnlock()

	return strings.TrimRight(sb.String(), "\n")
}

// --- Exit operations ---

// FindExit looks up an exit by direction key or closure name (case-insensitive).
// Returns the direction key and the resolved exit, or ("", nil) if not found.
func (ri *RoomInstance) FindExit(name string) (string, *ResolvedExit) {
	name = strings.ToLower(name)
	if re, ok := ri.exits[name]; ok {
		return name, re
	}
	for dir, re := range ri.exits {
		if re.Exit.Closure != nil && strings.EqualFold(re.Exit.Closure.Name, name) {
			return dir, re
		}
	}
	return "", nil
}

// FindExtraDesc searches the room's extra descriptions and then the extra
// descriptions on objects in the room for a keyword match (case-insensitive).
func (ri *RoomInstance) FindExtraDesc(keyword string) *assets.ExtraDesc {
	lower := strings.ToLower(keyword)

	for i := range ri.Room.Get().ExtraDescs {
		ed := &ri.Room.Get().ExtraDescs[i]
		for _, kw := range ed.Keywords {
			if strings.ToLower(kw) == lower {
				return ed
			}
		}
	}

	var found *assets.ExtraDesc
	ri.objects.ForEachObj(func(_ string, oi *ObjectInstance) {
		if found != nil {
			return
		}
		for i := range oi.Object.Get().ExtraDescs {
			ed := &oi.Object.Get().ExtraDescs[i]
			for _, kw := range ed.Keywords {
				if strings.ToLower(kw) == lower {
					found = ed
					return
				}
			}
		}
	})
	return found
}

// initExitClosures resets closed/locked state on resolved exits from their definitions.
// Caller must hold the write lock or call before the instance is shared.
func (ri *RoomInstance) initExitClosures() {
	for _, re := range ri.exits {
		if re.Exit.Closure != nil {
			re.closed = re.Exit.Closure.Closed
			if re.Exit.Closure.Lock != nil {
				re.locked = re.Exit.Closure.Lock.Locked
			}
		} else {
			re.closed = false
			re.locked = false
		}
	}
}

// --- Mob operations ---

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
	return ri.mobiles[name]
}

// GetMob returns the MobileInstance with the given instanceId, or nil if not found.
func (ri *RoomInstance) GetMob(instanceId string) *MobileInstance {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	return ri.mobiles[instanceId]
}

// AddMob places an existing MobileInstance into the room, setting its location.
func (ri *RoomInstance) AddMob(mi *MobileInstance) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.addMob(mi)
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

// ForEachMob calls fn for each mob in the room while holding the lock.
func (ri *RoomInstance) ForEachMob(fn func(*MobileInstance)) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	for _, mi := range ri.mobiles {
		fn(mi)
	}
}

// addMob inserts a mob and sets its location. Caller must hold the write lock.
func (ri *RoomInstance) addMob(mi *MobileInstance) {
	mi.room = ri
	ri.mobiles[mi.Id()] = mi
}

// --- Object operations ---

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

// --- Player operations ---

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

// --- Formatting helpers ---

func formatFlags(flags []string) string {
	var s strings.Builder
	for _, f := range flags {
		s.WriteString(" (" + f + ")")
	}
	return s.String()
}

func formatExits(exits map[string]*ResolvedExit) string {
	if len(exits) == 0 {
		return "[Exits: none]"
	}
	dirs := make([]string, 0, len(exits))
	for dir, re := range exits {
		label := dir
		if re.locked {
			label += " (locked)"
		} else if re.closed {
			label += " (closed)"
		}
		dirs = append(dirs, label)
	}
	sort.Strings(dirs)
	return fmt.Sprintf("[Exits: %s]", strings.Join(dirs, ", "))
}
