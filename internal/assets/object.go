package assets

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// ObjectFlag defines a boolean property of an object.
type ObjectFlag int

// ObjectFlag values.
const (
	ObjectFlagUnknown ObjectFlag = iota
	ObjectFlagContainer
	ObjectFlagImmobile
	ObjectFlagWearable
)

func parseObjectFlag(s string) ObjectFlag {
	switch strings.ToLower(s) {
	case "container":
		return ObjectFlagContainer
	case "immobile":
		return ObjectFlagImmobile
	case "wearable":
		return ObjectFlagWearable
	default:
		return ObjectFlagUnknown
	}
}

// Object defines a type of object/item loaded from asset files.
// Multiple instances can be spawned from one definition.
// Object IDs follow the convention <zone>-<name> (e.g., "millbrook-sword").
type Object struct {
	// Aliases are keywords players can use to target this object (e.g., ["sword", "blade"])
	Aliases []string `json:"aliases"`

	// ShortDesc is used in action messages (e.g., "You pick up a rusty sword.")
	ShortDesc string `json:"short_desc"`

	// LongDesc is shown when the object is on the ground in a room
	LongDesc string `json:"long_desc"`

	// DetailedDesc is shown when a player looks at the object
	DetailedDesc string `json:"detailed_desc"`

	// Flags are boolean markers for object properties (e.g., "wearable", "container", "nodrop")
	Flags []string `json:"flags,omitempty"`

	// WearSlots lists the slot types this item can be equipped in (e.g., ["head"], ["finger"],
	// ["hand_main", "hand_off"]). Only meaningful when the "wearable" flag is set.
	WearSlots []string `json:"wear_slots,omitempty"`

	// Closure defines open/close/lock behavior. Only meaningful when the "container" flag is set.
	Closure *Closure `json:"closure,omitempty"`

	// Weapon damage dice (intrinsic weapon properties, not additive bonuses).
	DamageDice  int `json:"damage_dice,omitempty"`
	DamageSides int `json:"damage_sides,omitempty"`

	// Perks granted while this item is equipped (AC, stat mods, damage mods, etc.).
	Perks []Perk `json:"perks,omitempty"`

	// Lifetime is the number of game ticks before a spawned instance of this
	// object decays and is removed. Zero (the default) means permanent.
	// Decayable items (Lifetime > 0) are not persisted across logout.
	Lifetime int `json:"lifetime,omitempty"`

	// ExtraDescs are keyword-accessible descriptions on this object.
	ExtraDescs []ExtraDesc `json:"extra_descs,omitempty"`
}

// MatchName returns true if name matches any of this object's aliases (case-insensitive).
func (o *Object) MatchName(name string) bool {
	nameLower := strings.ToLower(name)
	for _, alias := range o.Aliases {
		if strings.ToLower(alias) == nameLower {
			return true
		}
	}
	return false
}

// HasFlag returns true if the object has the given flag.
func (o *Object) HasFlag(flag ObjectFlag) bool {
	for _, f := range o.Flags {
		if parseObjectFlag(f) == flag {
			return true
		}
	}
	return false
}

// Validate satisfies storage.ValidatingSpec
func (o *Object) Validate() error {
	el := errors.NewErrorList()
	if len(o.Aliases) < 1 {
		el.Add(fmt.Errorf("object alias is required"))
	}
	if o.ShortDesc == "" {
		el.Add(fmt.Errorf("object short description is required"))
	}
	for _, f := range o.Flags {
		if parseObjectFlag(f) == ObjectFlagUnknown {
			el.Add(fmt.Errorf("unknown flag %q", f))
		}
	}
	if o.HasFlag(ObjectFlagWearable) && len(o.WearSlots) == 0 {
		el.Add(fmt.Errorf("wearable items must have at least one wear_slot"))
	}
	if !o.HasFlag(ObjectFlagWearable) && len(o.WearSlots) > 0 {
		el.Add(fmt.Errorf("wear_slots requires the wearable flag"))
	}
	for i := range o.Perks {
		if err := o.Perks[i].validate(); err != nil {
			el.Add(fmt.Errorf("perk[%d]: %w", i, err))
		}
	}
	if o.Closure != nil {
		if !o.HasFlag(ObjectFlagContainer) {
			el.Add(fmt.Errorf("closure requires the container flag"))
		}
		el.Add(o.Closure.Validate())
	}
	for i, ed := range o.ExtraDescs {
		if len(ed.Keywords) == 0 {
			el.Add(fmt.Errorf("extra_descs[%d]: at least one keyword is required", i))
		}
		if ed.Description == "" {
			el.Add(fmt.Errorf("extra_descs[%d]: description is required", i))
		}
	}
	return el.Err()
}

// Resolve resolves foreign key references on the object definition.
func (o *Object) Resolve(objs storage.Storer[*Object]) error {
	if o.Closure != nil {
		return o.Closure.Resolve(objs)
	}
	return nil
}

// ObjectSpawn defines an object to spawn in a room or mobile inventory during zone reset.
// Contents lists objects to spawn inside this container (requires the container flag).
// Supports nesting — content items can themselves be containers with contents.
type ObjectSpawn struct {
	Object   storage.SmartIdentifier[*Object] `json:"object_id"`
	Contents []ObjectSpawn                    `json:"contents,omitempty"`
}

// Resolve resolves foreign key references in the spawn spec.
func (s *ObjectSpawn) Resolve(objs storage.Storer[*Object]) error {
	el := errors.NewErrorList()
	el.Add(s.Object.Resolve(objs))
	for i := range s.Contents {
		el.Add(s.Contents[i].Resolve(objs))
	}
	return el.Err()
}

// EquipmentSpawn pairs a slot name with an object spawn for equipment persistence.
type EquipmentSpawn struct {
	Slot string `json:"slot"`
	ObjectSpawn
}

// Resolve resolves the embedded ObjectSpawn's foreign key references.
func (es *EquipmentSpawn) Resolve(objs storage.Storer[*Object]) error {
	return es.ObjectSpawn.Resolve(objs)
}
