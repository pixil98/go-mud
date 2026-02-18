package commands

import (
	"github.com/pixil98/go-mud/internal/game"
)

// objectOnlyFinder wraps an ObjectFinder (like Inventory or Equipment)
// into a full TargetFinder. FindPlayer and FindMob always return nil.
type objectOnlyFinder struct {
	ObjectFinder
}

func (f objectOnlyFinder) FindPlayer(string) *game.PlayerState { return nil }
func (f objectOnlyFinder) FindMob(string) *game.MobileInstance { return nil }

// WorldScopes implements TargetScopes using *game.WorldState.
// It translates scope flags into the correct search spaces by looking up
// rooms, zones, inventory, and equipment from the world state.
type WorldScopes struct {
	world *game.WorldState
}

// NewWorldScopes creates a TargetScopes backed by the given WorldState.
func NewWorldScopes(world *game.WorldState) *WorldScopes {
	return &WorldScopes{world: world}
}

// SpacesFor returns search spaces for the given scope flags, ordered from
// narrowest (inventory) to broadest (world).
func (ws *WorldScopes) SpacesFor(scope Scope, actor *game.Character, session *game.PlayerState) ([]SearchSpace, error) {
	zoneId, roomId := session.Location()

	var spaces []SearchSpace

	if scope&ScopeInventory != 0 && actor.Inventory != nil {
		spaces = append(spaces, SearchSpace{
			Finder:  objectOnlyFinder{actor.Inventory},
			Remover: actor.Inventory,
		})
	}
	if scope&ScopeEquipment != 0 && actor.Equipment != nil {
		spaces = append(spaces, SearchSpace{
			Finder:  objectOnlyFinder{actor.Equipment},
			Remover: actor.Equipment,
		})
	}
	if scope&ScopeRoom != 0 {
		room := ws.world.Instances()[zoneId].GetRoom(roomId)
		spaces = append(spaces, SearchSpace{
			Finder:  room,
			Remover: room,
		})
	}
	if scope&ScopeZone != 0 {
		zone := ws.world.Instances()[zoneId]
		spaces = append(spaces, SearchSpace{
			Finder: zone,
		})
	}
	if scope&ScopeWorld != 0 {
		for _, zi := range ws.world.Instances() {
			spaces = append(spaces, SearchSpace{
				Finder: zi,
			})
		}
	}

	return spaces, nil
}
