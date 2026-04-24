package assets

import (
	"errors"
	"fmt"

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
	RoomFlagDeath                   // Death trap; character dies on entry
	RoomFlagNoMob                   // Mobs cannot wander in
	RoomFlagSingleOccupant          // Only one player allowed at a time
	RoomFlagDark                    // Room is dark; occupants without darkvision can't see
)

// parseRoomFlag converts a flag name to its enum value. Input must be
// lowercase; Validate catches any non-lowercase or otherwise unknown flag at
// load time so hot paths can skip the case-folding.
func parseRoomFlag(s string) RoomFlag {
	switch s {
	case "death":
		return RoomFlagDeath
	case "nomob":
		return RoomFlagNoMob
	case "single_occupant":
		return RoomFlagSingleOccupant
	case "dark":
		return RoomFlagDark
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

// Validate checks that the extra description has at least one keyword and a non-empty body.
func (ed *ExtraDesc) Validate() error {
	var errs []error
	if len(ed.Keywords) == 0 {
		errs = append(errs, errors.New("at least one keyword is required"))
	}
	if ed.Description == "" {
		errs = append(errs, errors.New("description is required"))
	}
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Exit
// ---------------------------------------------------------------------------

// Exit defines a destination for movement from a room.
type Exit struct {
	Zone        storage.SmartIdentifier[*Zone] `json:"zone_id"` // Optional; defaults to current zone
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
	var errs []error

	if r.Name == "" {
		errs = append(errs, errors.New("room name is required"))
	}
	errs = append(errs, r.Zone.Validate())

	for dir, exit := range r.Exits {
		if err := exit.Room.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("exit %s: %w", dir, err))
		}
		if exit.Closure != nil {
			if exit.Closure.Name == "" {
				errs = append(errs, fmt.Errorf("exit %s closure: name is required", dir))
			}
			if err := exit.Closure.Validate(); err != nil {
				errs = append(errs, fmt.Errorf("exit %s closure: %w", dir, err))
			}
		}
	}

	for _, f := range r.Flags {
		if parseRoomFlag(f) == RoomFlagUnknown {
			errs = append(errs, fmt.Errorf("unknown flag %q", f))
		}
	}

	for i := range r.ExtraDescs {
		if err := r.ExtraDescs[i].Validate(); err != nil {
			errs = append(errs, fmt.Errorf("extra_descs[%d]: %w", i, err))
		}
	}

	return errors.Join(errs...)
}

// Resolve resolves foreign key references on the room definition.
func (r *Room) Resolve(zones storage.Storer[*Zone], rooms storage.Storer[*Room], mobiles storage.Storer[*Mobile], objects storage.Storer[*Object]) error {
	var errs []error
	errs = append(errs, r.Zone.Resolve(zones))
	for dir, exit := range r.Exits {
		errs = append(errs, exit.Room.Resolve(rooms))
		if exit.Zone.Id() != "" {
			errs = append(errs, exit.Zone.Resolve(zones))
		}
		if exit.Closure != nil {
			errs = append(errs, exit.Closure.Resolve(objects))
		}
		r.Exits[dir] = exit
	}

	for i := range r.MobSpawns {
		errs = append(errs, r.MobSpawns[i].Resolve(mobiles))
	}
	for i := range r.ObjSpawns {
		errs = append(errs, r.ObjSpawns[i].Resolve(objects))
	}
	return errors.Join(errs...)
}
