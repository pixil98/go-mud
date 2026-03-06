package game

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

type ZoneInstance struct {
	Zone storage.SmartIdentifier[*assets.Zone]

	nextReset        time.Time     // when zone should next reset (runtime only)
	lifespanDuration time.Duration // parsed lifespan

	rooms map[string]*RoomInstance

	Perks *TimedPerkCache
}

func NewZoneInstance(zone storage.SmartIdentifier[*assets.Zone]) (*ZoneInstance, error) {
	def := zone.Get()
	if def == nil {
		return nil, fmt.Errorf("unable to create instance from unresolved zone %q", zone.Id())
	}
	zi := &ZoneInstance{
		Zone:  zone,
		rooms: make(map[string]*RoomInstance),
		Perks: NewTimedPerkCache(nil),
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
// TimedPerkCache as a source for the room's TimedPerkCache.
func (z *ZoneInstance) AddRoom(ri *RoomInstance) {
	ri.Perks.AddSource("zone", z.Perks)
	z.rooms[ri.Room.Id()] = ri
}

// Reset checks reset conditions and respawns mobs/objects if appropriate.
// If force is true, bypasses time/occupancy checks.
// instances is the full set of zone instances for cross-zone door synchronization.
func (z *ZoneInstance) Reset(force bool, instances map[string]*ZoneInstance) error {
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
		err := ri.Reset(instances)
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

func (z *ZoneInstance) GetRoom(roomId string) *RoomInstance {
	return z.rooms[roomId]
}

// FindPlayer searches all rooms in the zone for a player whose character name matches.
func (z *ZoneInstance) FindPlayer(name string) *CharacterInstance {
	for _, r := range z.rooms {
		ps := r.FindPlayer(name)
		if ps != nil {
			return ps
		}
	}
	return nil
}

// FindMob searches room mobs for one whose definition matches the given name.
func (z *ZoneInstance) FindMob(name string) *MobileInstance {
	for _, r := range z.rooms {
		mi := r.FindMob(name)
		if mi != nil {
			return mi
		}
	}

	return nil
}

// FindObj searches room objects for one whose definition matches the given name.
func (z *ZoneInstance) FindObj(name string) *ObjectInstance {
	for _, r := range z.rooms {
		oi := r.FindObj(name)
		if oi != nil {
			return oi
		}
	}
	return nil
}

// FindExit always returns ("", nil) — exits are only meaningful in room scope.
func (z *ZoneInstance) FindExit(string) (string, *assets.Exit) { return "", nil }
