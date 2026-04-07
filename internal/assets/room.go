package assets

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// ---------------------------------------------------------------------------
// Room flags
// ---------------------------------------------------------------------------

// RoomFlag defines a boolean property of a room.
type RoomFlag int

// RoomFlag values.
const (
	RoomFlagUnknown        RoomFlag = iota
	RoomFlagDeath                    // Death trap; character dies on entry
	RoomFlagNoMob                    // Mobs cannot wander in
	RoomFlagSingleOccupant           // Only one player allowed at a time
)

func parseRoomFlag(s string) RoomFlag {
	switch strings.ToLower(s) {
	case "death":
		return RoomFlagDeath
	case "nomob":
		return RoomFlagNoMob
	case "single_occupant":
		return RoomFlagSingleOccupant
	default:
		return RoomFlagUnknown
	}
}

// ---------------------------------------------------------------------------
// Extra descriptions
// ---------------------------------------------------------------------------

// ExtraDesc is a keyword-accessible description on a room or object.
// Players access it via "look at <keyword>".
type ExtraDesc struct {
	Keywords    []string `json:"keywords"`
	Description string   `json:"description"`
}

// ---------------------------------------------------------------------------
// Exit
// ---------------------------------------------------------------------------

// Exit defines a destination for movement from a room.
type Exit struct {
	Zone        storage.SmartIdentifier[*Zone] `json:"zone_id"`               // Optional; defaults to current zone
	Room        storage.SmartIdentifier[*Room] `json:"room_id"`
	Closure     *Closure                       `json:"closure,omitempty"`     // Optional open/close/lock barrier
	Description string                         `json:"description,omitempty"` // Shown when player looks in this direction
}

// ---------------------------------------------------------------------------
// Room
// ---------------------------------------------------------------------------

// Room represents a location within a zone.
type Room struct {
	Name        string                             `json:"name"`
	Description string                             `json:"description"`
	Zone        storage.SmartIdentifier[*Zone]     `json:"zone_id"`
	Exits       map[string]Exit                    `json:"exits"`
	MobSpawns   []storage.SmartIdentifier[*Mobile] `json:"mobile_spawns"` // mobile IDs to spawn; list duplicates for multiple
	ObjSpawns   []ObjectSpawn                      `json:"object_spawns"` // objects to spawn
	Perks       []Perk                             `json:"perks,omitempty"`
	Flags       []string                           `json:"flags,omitempty"`
	ExtraDescs  []ExtraDesc                        `json:"extra_descs,omitempty"`
}

// HasFlag returns true if the room has the given flag.
func (r *Room) HasFlag(flag RoomFlag) bool {
	for _, f := range r.Flags {
		if parseRoomFlag(f) == flag {
			return true
		}
	}
	return false
}

// Validate returns an error if the room definition is invalid.
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

	for _, f := range r.Flags {
		if parseRoomFlag(f) == RoomFlagUnknown {
			el.Add(fmt.Errorf("unknown flag %q", f))
		}
	}

	for i, ed := range r.ExtraDescs {
		if len(ed.Keywords) == 0 {
			el.Add(fmt.Errorf("extra_descs[%d]: at least one keyword is required", i))
		}
		if ed.Description == "" {
			el.Add(fmt.Errorf("extra_descs[%d]: description is required", i))
		}
	}

	return el.Err()
}

// Resolve resolves foreign key references on the room definition.
func (r *Room) Resolve(zones storage.Storer[*Zone], rooms storage.Storer[*Room], mobiles storage.Storer[*Mobile], objects storage.Storer[*Object]) error {
	el := errors.NewErrorList()
	el.Add(r.Zone.Resolve(zones))
	for dir, exit := range r.Exits {
		el.Add(exit.Room.Resolve(rooms))
		if exit.Zone.Id() != "" {
			el.Add(exit.Zone.Resolve(zones))
		}
		if exit.Closure != nil {
			el.Add(exit.Closure.Resolve(objects))
		}
		r.Exits[dir] = exit
	}

	for i := range r.MobSpawns {
		el.Add(r.MobSpawns[i].Resolve(mobiles))
	}
	for i := range r.ObjSpawns {
		el.Add(r.ObjSpawns[i].Resolve(objects))
	}
	return el.Err()
}
