package game

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

const (
	// wanderChance is the 1-in-N chance a mob attempts to wander each tick.
	wanderChance = 20
	// scavengeChance is the 1-in-N chance a scavenger mob picks up an item each tick.
	scavengeChance = 10
)

// ZoneInstance holds the runtime state for a zone, including its rooms and reset schedule.
type ZoneInstance struct {
	Zone storage.SmartIdentifier[*assets.Zone]

	nextReset        time.Time     // when zone should next reset (runtime only)
	lifespanDuration time.Duration // parsed lifespan

	rooms map[string]*RoomInstance

	Perks *PerkCache
}

// NewZoneInstance creates a ZoneInstance from a resolved zone asset.
func NewZoneInstance(zone storage.SmartIdentifier[*assets.Zone]) (*ZoneInstance, error) {
	def := zone.Get()
	if def == nil {
		return nil, fmt.Errorf("unable to create instance from unresolved zone %q", zone.Id())
	}
	zi := &ZoneInstance{
		Zone:  zone,
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
	ri.Perks.AddSource("zone", z.Perks)
	z.rooms[ri.Room.Id()] = ri
}

// Reset checks reset conditions and respawns mobs/objects if appropriate.
// If force is true, bypasses time/occupancy checks.
// instances is the full set of zone instances for cross-zone door synchronization.
func (z *ZoneInstance) Reset(force bool, instances map[string]*ZoneInstance, mc MobCommander) error {
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
		err := ri.Reset(instances, mc)
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
	z.wanderMobs()
	z.scavengeMobs()
}

// wanderMobs gives each non-sentinel, non-combat mob with a commander
// a chance to move to a random adjacent room.
func (z *ZoneInstance) wanderMobs() {
	for _, ri := range z.rooms {
		ri.mu.RLock()
		mobs := make([]*MobileInstance, 0, len(ri.mobiles))
		for _, mi := range ri.mobiles {
			mobs = append(mobs, mi)
		}
		ri.mu.RUnlock()

		for _, mi := range mobs {
			if mi.Commander == nil || mi.IsInCombat() {
				continue
			}
			if mi.Mobile.Get().HasFlag(assets.MobileFlagSentinel) {
				continue
			}
			if rand.IntN(wanderChance) != 0 {
				continue
			}
			z.tryWander(mi, ri)
		}
	}
}

// tryWander picks a random valid exit and executes the direction command
// via the mob's commander.
func (z *ZoneInstance) tryWander(mi *MobileInstance, from *RoomInstance) {
	def := from.Room.Get()
	if len(def.Exits) == 0 {
		return
	}

	mob := mi.Mobile.Get()
	stayZone := mob.HasFlag(assets.MobileFlagStayZone)
	thisZone := def.Zone.Id()

	var directions []string

	for dir, exit := range def.Exits {
		if from.IsExitClosed(dir) {
			continue
		}

		destRoom := z.rooms[exit.Room.Id()]
		if destRoom == nil {
			continue
		}

		destDef := destRoom.Room.Get()
		if destDef.HasFlag(assets.RoomFlagNoMob) || destDef.HasFlag(assets.RoomFlagDeath) {
			continue
		}

		if stayZone {
			destZone := exit.Zone.Id()
			if destZone == "" {
				destZone = thisZone
			}
			if destZone != thisZone {
				continue
			}
		}

		directions = append(directions, dir)
	}

	if len(directions) == 0 {
		return
	}

	dir := directions[rand.IntN(len(directions))]
	if err := mi.Commander.ExecMobCommand(context.Background(), mi, dir); err != nil {
		slog.Debug("mob wander failed", "mob", mi.Mobile.Id(), "direction", dir, "error", err)
	}
}

// scavengeMobs lets scavenger mobs pick up an item from their room.
func (z *ZoneInstance) scavengeMobs() {
	for _, ri := range z.rooms {
		if ri.objects.Len() == 0 {
			continue
		}

		ri.mu.RLock()
		mobs := make([]*MobileInstance, 0, len(ri.mobiles))
		for _, mi := range ri.mobiles {
			mobs = append(mobs, mi)
		}
		ri.mu.RUnlock()

		for _, mi := range mobs {
			if mi.Commander == nil || mi.IsInCombat() {
				continue
			}
			if !mi.Mobile.Get().HasFlag(assets.MobileFlagScavenger) {
				continue
			}
			if rand.IntN(scavengeChance) != 0 {
				continue
			}
			// Pick up the first non-immobile object in the room.
			var alias string
			ri.objects.ForEachObj(func(_ string, oi *ObjectInstance) {
				if alias != "" {
					return
				}
				def := oi.Object.Get()
				if def.HasFlag(assets.ObjectFlagImmobile) {
					return
				}
				if len(def.Aliases) > 0 {
					alias = def.Aliases[0]
				}
			})
			if alias == "" {
				continue
			}
			if err := mi.Commander.ExecMobCommand(context.Background(), mi, "get", alias); err != nil {
				slog.Debug("mob scavenge failed", "mob", mi.Mobile.Id(), "item", alias, "error", err)
			}
		}
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
