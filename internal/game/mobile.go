package game

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/storage"
	"github.com/pixil98/go-errors"
)

// Mobile defines a type of mobile entity loaded from asset files.
// Multiple instances can be spawned from one definition.
// Mobile IDs follow the convention <zone>-<name> (e.g., "millbrook-guard").
type Mobile struct {
	Entity
	Description string `json:"description"`
}

// Validate satisfies storage.ValidatingSpec
func (m *Mobile) Validate() error {
	el := errors.NewErrorList()
	if m.EntityName == "" {
		el.Add(fmt.Errorf("mobile name is required"))
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
