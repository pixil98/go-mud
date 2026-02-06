package game

import (
	"fmt"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// Mobile defines a type of mobile entity loaded from asset files.
// Multiple instances can be spawned from one definition.
// Mobile IDs follow the convention <zone>-<name> (e.g., "millbrook-guard").
type Mobile struct {
	Entity

	// Aliases are keywords players can use to target this mobile (e.g., ["guard", "town"])
	Aliases []string `json:"aliases"`

	// ShortDesc is used in action messages (e.g., "The town guard hits you.")
	ShortDesc string `json:"short_desc"`

	// LongDesc is shown when the mobile is in its default position in a room
	// (e.g., "A burly guard in chain mail keeps watch over the square.")
	LongDesc string `json:"long_desc"`
}

// Validate satisfies storage.ValidatingSpec
func (m *Mobile) Validate() error {
	el := errors.NewErrorList()
	if len(m.Aliases) < 1 {
		el.Add(fmt.Errorf("mobile alias is required"))
	}
	if m.ShortDesc == "" {
		el.Add(fmt.Errorf("mobile short description is required"))
	}
	return el.Err()
}

// MobileInstance represents a single spawned instance of a Mobile definition.
// TODO: Evaluate extracting shared location fields (ZoneId, RoomId) with PlayerState
type MobileInstance struct {
	InstanceId string             // Unique ID: "<mobile-id>-<counter>" e.g., "millbrook-guard-1"
	MobileId   storage.Identifier // Reference to the Mobile definition
	ZoneId     storage.Identifier
	RoomId     storage.Identifier
}

// Location returns the mobile instance's current zone and room.
func (mi *MobileInstance) Location() (zoneId, roomId storage.Identifier) {
	return mi.ZoneId, mi.RoomId
}
