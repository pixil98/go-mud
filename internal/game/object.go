package game

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// ObjectFlag defines a boolean property of an object.
type ObjectFlag int

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
	// (e.g., "A rusty sword lies discarded in the corner.")
	LongDesc string `json:"long_desc"`

	// DetailedDesc is shown when a player looks at the object
	DetailedDesc string `json:"detailed_desc"`

	// TypeStr is the object type from JSON
	TypeStr string `json:"type"`

	// Flags are boolean markers for object properties (e.g., "wearable", "container", "nodrop")
	Flags []string `json:"flags,omitempty"`

	// WearSlots lists the slot types this item can be equipped in (e.g., ["head"], ["finger"],
	// ["hand_main", "hand_off"]). Only meaningful when the "wearable" flag is set.
	WearSlots []string `json:"wear_slots,omitempty"`
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
	return el.Err()
}

// ObjectInstance represents a single spawned instance of an Object definition.
// Location is tracked by the containing structure (room map or inventory).
type ObjectInstance struct {
	InstanceId string             // Unique ID
	ObjectId   storage.Identifier // Reference to the Object definition
	Definition *Object            `json:"-" `
	Contents   *Inventory         // Non-nil for containers; holds objects stored inside
}

// NewObjectInstance creates an ObjectInstance linked to its definition.
// Containers are initialized with an empty Contents inventory.
func NewObjectInstance(objectId storage.Identifier, def *Object) *ObjectInstance {
	oi := &ObjectInstance{
		InstanceId: uuid.New().String(),
		ObjectId:   objectId,
		Definition: def,
	}
	if def != nil && def.HasFlag(ObjectFlagContainer) {
		oi.Contents = NewInventory()
	}
	return oi
}
