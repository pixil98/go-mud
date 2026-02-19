package game

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

const (
	ZoneResetNever    = "never"    // Zone never resets
	ZoneResetLifespan = "lifespan" // Zone resets when lifespan is reached
	ZoneResetEmpty    = "empty"    // Zone resets when lifespan is reached and is empty
)

// Zone represents a region in the game world that contains rooms.
type Zone struct {
	Lifespan  string `json:"lifespan"` // duration string (e.g., "1m", "30s", "2h")
	ResetMode string `json:"reset_mode"`
}

// Validate satisfies storage.ValidatingSpec.
func (z *Zone) Validate() error {
	el := errors.NewErrorList()

	// Validate reset mode is specified and valid
	switch z.ResetMode {
	case ZoneResetNever, ZoneResetLifespan, ZoneResetEmpty:
		// valid
	case "":
		el.Add(fmt.Errorf("reset_mode is required (must be %s, %s, or %s)",
			ZoneResetNever, ZoneResetLifespan, ZoneResetEmpty))
	default:
		el.Add(fmt.Errorf("invalid reset_mode: %s (must be %s, %s, or %s)",
			z.ResetMode, ZoneResetNever, ZoneResetLifespan, ZoneResetEmpty))
	}

	// Parse and validate lifespan for time-based reset modes
	if z.ResetMode == ZoneResetLifespan || z.ResetMode == ZoneResetEmpty {
		if z.Lifespan == "" {
			el.Add(fmt.Errorf("lifespan is required for reset_mode %s", z.ResetMode))
		} else {
			d, err := time.ParseDuration(z.Lifespan)
			if err != nil {
				el.Add(fmt.Errorf("invalid lifespan %q: %w", z.Lifespan, err))
			} else if d <= 0 {
				el.Add(fmt.Errorf("lifespan must be positive for reset_mode %s", z.ResetMode))
			}
		}
	}

	return el.Err()
}

type ZoneInstance struct {
	ZoneId     storage.Identifier
	Definition *Zone

	nextReset        time.Time     // when zone should next reset (runtime only)
	lifespanDuration time.Duration // parsed lifespan

	rooms map[storage.Identifier]*RoomInstance
}

func NewZoneInstance(zoneId storage.Identifier, def *Zone) *ZoneInstance {
	return &ZoneInstance{
		ZoneId:     zoneId,
		Definition: def,
		rooms:      make(map[storage.Identifier]*RoomInstance),
	}
}

// AddRoom adds a room instance to the zone.
func (z *ZoneInstance) AddRoom(roomId storage.Identifier, ri *RoomInstance) {
	z.rooms[roomId] = ri
}

// Reset checks reset conditions and respawns mobs/objects if appropriate.
// If force is true, bypasses time/occupancy checks.
func (z *ZoneInstance) Reset(force bool) {
	now := time.Now()

	if !force {
		if z.Definition.ResetMode == ZoneResetNever {
			return
		}
		if now.Before(z.nextReset) {
			return
		}
		if z.Definition.ResetMode == ZoneResetEmpty && z.IsOccupied() {
			return
		}
	}

	for _, ri := range z.rooms {
		ri.Reset()
	}

	if z.lifespanDuration > 0 {
		z.nextReset = now.Add(z.lifespanDuration)
	}

	slog.Info("zone reset complete", "zone", z.ZoneId, "rooms", len(z.rooms))
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

func (z *ZoneInstance) GetRoom(roomId storage.Identifier) *RoomInstance {
	return z.rooms[roomId]
}

// FindPlayer searches all rooms in the zone for a player whose character name matches.
func (z *ZoneInstance) FindPlayer(name string) *PlayerState {
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
