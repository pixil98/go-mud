package game

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// ObjectType defines the category of an object.
type ObjectType int

const (
	ObjectTypeUnknown ObjectType = iota
	ObjectTypeOther
)

// Object defines a type of object/item loaded from asset files.
// Multiple instances can be spawned from one definition.
// Object IDs follow the convention <zone>-<name> (e.g., "millbrook-sword").
type Object struct {
	Entity

	// Aliases are keywords players can use to target this object (e.g., ["sword", "blade"])
	Aliases []string `json:"aliases"`

	// ShortDesc is used in action messages (e.g., "You pick up a rusty sword.")
	ShortDesc string `json:"short_desc"`

	// LongDesc is shown when the object is on the ground in a room
	// (e.g., "A rusty sword lies discarded in the corner.")
	LongDesc string `json:"long_desc"`

	// TypeStr is the object type from JSON
	TypeStr string `json:"type"`
}

// Type returns the parsed ObjectType from TypeStr.
func (o *Object) Type() ObjectType {
	switch strings.ToLower(o.TypeStr) {
	case "other":
		return ObjectTypeOther
	default:
		return ObjectTypeUnknown
	}
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
	if o.TypeStr == "" {
		el.Add(fmt.Errorf("object type is required"))
	} else if o.Type() == ObjectTypeUnknown {
		el.Add(fmt.Errorf("object type %q is invalid", o.TypeStr))
	}
	return el.Err()
}

// ObjectInstance represents a single spawned instance of an Object definition.
// TODO: Expand to support inventory/equipment when needed.
type ObjectInstance struct {
	InstanceId string             // Unique ID
	ObjectId   storage.Identifier // Reference to the Object definition
	ZoneId     storage.Identifier // Zone where object is located
	RoomId     storage.Identifier // Room where object is located
}
