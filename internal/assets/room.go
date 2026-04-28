package assets

import (
	"errors"
	"fmt"

	"github.com/pixil98/go-mud/internal/storage"
)

// ---------------------------------------------------------------------------
// Room flag keys
// ---------------------------------------------------------------------------

// RoomFlag is the perk key string for a well-known room property. Authoring
// stores these as grant perks; runtime checks query the room's PerkCache via
// (*RoomInstance).Restricts. The "room_" prefix encodes the audience and
// keeps keys unique across scopes.
type RoomFlag string

const (
	RoomFlagDeath          RoomFlag = "room_death"           // Death trap; character dies on entry
	RoomFlagNoMob          RoomFlag = "room_nomob"           // Mobs cannot wander in
	RoomFlagSingleOccupant RoomFlag = "room_single_occupant" // Only one player allowed at a time
	RoomFlagDark           RoomFlag = "room_dark"            // Occupants without an "ignore_restriction:room_dark" grant can't see
	RoomFlagNoMagic        RoomFlag = "room_nomagic"         // Spellcasting blocked unless caster has "ignore_restriction:room_nomagic"
	RoomFlagWater          RoomFlag = "room_water"           // Deep water; entry blocked unless actor has "ignore_restriction:room_water"
	RoomFlagPeaceful       RoomFlag = "room_peaceful"        // Combat initiation blocked unless actor has "ignore_restriction:room_peaceful"
)

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
	ExtraDescs  []ExtraDesc                        `json:"extra_descs,omitempty"`
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

	if err := validatePerks(r.Perks); err != nil {
		errs = append(errs, err)
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
