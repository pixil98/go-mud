package game

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// ZoneInstance holds the runtime state for a zone, including its rooms and reset schedule.
type ZoneInstance struct {
	Zone storage.SmartIdentifier[*assets.Zone]

	world *WorldState

	nextReset        time.Time     // when zone should next reset (runtime only)
	lifespanDuration time.Duration // parsed lifespan

	rooms map[string]*RoomInstance

	Perks *PerkCache
}

// World returns the world state this zone belongs to.
func (z *ZoneInstance) World() *WorldState {
	return z.world
}

// NewZoneInstance creates a ZoneInstance from a resolved zone asset.
func NewZoneInstance(zone storage.SmartIdentifier[*assets.Zone], world *WorldState) (*ZoneInstance, error) {
	def := zone.Get()
	if def == nil {
		return nil, fmt.Errorf("unable to create instance from unresolved zone %q", zone.Id())
	}
	zi := &ZoneInstance{
		Zone:  zone,
		world: world,
		rooms: make(map[string]*RoomInstance),
		Perks: NewPerkCache(def.Perks, nil),
	}
	if def.Lifespan != "" {
		d, err := time.ParseDuration(def.Lifespan)
		if err != nil {
			return nil, fmt.Errorf("zone %q: invalid lifespan %q: %w", zone.Id(), def.Lifespan, err)
		}
		zi.lifespanDuration = d
	}
	return zi, nil
}

// AddRoom adds a room instance to the zone and wires the zone's
// PerkCache as a source for the room's PerkCache.
func (z *ZoneInstance) AddRoom(ri *RoomInstance) {
	ri.zone = z
	ri.Perks.AddSource("zone", z.Perks)
	z.rooms[ri.Room.Id()] = ri
}

// Reset checks reset conditions and respawns mobs/objects if appropriate.
// If force is true, bypasses time/occupancy checks.
func (z *ZoneInstance) Reset(force bool, cf CommanderFactory) error {
	now := time.Now()

	if !force {
		if z.Zone.Get().ResetMode == assets.ZoneResetNever {
			return nil
		}
		if now.Before(z.nextReset) {
			return nil
		}
		if z.Zone.Get().ResetMode == assets.ZoneResetEmpty && z.IsOccupied() {
			return nil
		}
	}

	for _, ri := range z.rooms {
		err := ri.Reset(cf)
		if err != nil {
			return fmt.Errorf("resetting zone %q: %w", z.Zone.Id(), err)
		}
	}

	if z.lifespanDuration > 0 {
		z.nextReset = now.Add(z.lifespanDuration)
	}

	slog.Info("zone reset complete", "zone", z.Zone.Id(), "rooms", len(z.rooms))

	return nil
}

// Tick advances one game tick for the zone.
func (z *ZoneInstance) Tick() {
	z.Perks.Tick()
	for _, ri := range z.rooms {
		ri.Tick()
	}
}

// ForEachPlayer yields each player across all rooms in the zone.
func (z *ZoneInstance) ForEachPlayer(fn func(string, *CharacterInstance)) {
	for _, ri := range z.rooms {
		ri.ForEachPlayer(fn)
	}
}

// ForEachRoom calls fn for each room in the zone.
func (z *ZoneInstance) ForEachRoom(fn func(roomId string, room *RoomInstance)) {
	for id, ri := range z.rooms {
		fn(id, ri)
	}
}

// IsOccupied returns true if any players are in any room of this zone.
func (z *ZoneInstance) IsOccupied() bool {
	for _, ri := range z.rooms {
		if ri.PlayerCount() > 0 {
			return true
		}
	}
	return false
}

// GetRoom returns the RoomInstance for the given room ID, or nil if not found.
func (z *ZoneInstance) GetRoom(roomId string) *RoomInstance {
	return z.rooms[roomId]
}

// FindPlayers searches all rooms in the zone for players accepted by the matcher.
func (z *ZoneInstance) FindPlayers(match func(*CharacterInstance) bool) []*CharacterInstance {
	var out []*CharacterInstance
	for _, r := range z.rooms {
		out = append(out, r.FindPlayers(match)...)
	}
	return out
}

// FindMobs searches all rooms in the zone for mobs accepted by the matcher.
func (z *ZoneInstance) FindMobs(match func(*MobileInstance) bool) []*MobileInstance {
	var out []*MobileInstance
	for _, r := range z.rooms {
		out = append(out, r.FindMobs(match)...)
	}
	return out
}

// FindObjs searches all rooms in the zone for objects accepted by the matcher.
func (z *ZoneInstance) FindObjs(match func(*ObjectInstance) bool) []*ObjectInstance {
	var out []*ObjectInstance
	for _, r := range z.rooms {
		out = append(out, r.FindObjs(match)...)
	}
	return out
}

// FindExit always returns ("", nil) — exits are only meaningful in room scope.
func (z *ZoneInstance) FindExit(string) (string, *ResolvedExit) { return "", nil }
