package game

import (
	"fmt"

	"github.com/pixil98/go-errors"
)

// Exit defines a destination for movement from a room.
type Exit struct {
	ZoneId string `json:"zone_id,omitempty"` // Optional; defaults to current zone
	RoomId string `json:"room_id"`
}

// Room represents a location within a zone.
type Room struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	ZoneId      string          `json:"zone_id"`
	Exits       map[string]Exit `json:"exits"`             // direction -> destination
	Spawns      []string        `json:"spawns,omitempty"`  // mobile IDs to spawn; list duplicates for multiple
}

// ValidDirections defines the allowed exit directions.
var ValidDirections = map[string]bool{
	"north": true,
	"south": true,
	"east":  true,
	"west":  true,
	"up":    true,
	"down":  true,
}

// Validate satisfies storage.ValidatingSpec.
// TODO: Add cross-reference validation to ensure:
// - The room's zone_id references an existing zone
// - All exit destinations (zone_id + room_id) reference existing rooms
// This would require access to the zone and room stores, possibly via a
// ValidateWithStores(zones, rooms) method or a post-load validation pass.
func (r *Room) Validate() error {
	el := errors.NewErrorList()

	if r.Name == "" {
		el.Add(fmt.Errorf("room name is required"))
	}
	if r.ZoneId == "" {
		el.Add(fmt.Errorf("zone_id is required"))
	}

	for dir, exit := range r.Exits {
		if !ValidDirections[dir] {
			el.Add(fmt.Errorf("invalid exit direction: %s", dir))
		}
		if exit.RoomId == "" {
			el.Add(fmt.Errorf("exit %s: room_id is required", dir))
		}
	}

	return el.Err()
}
