package commands

import (
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)


// objectOnlyFinder wraps an ObjectFinder (like Inventory or Equipment)
// into a full TargetFinder. FindPlayer and FindMob always return nil.
type objectOnlyFinder struct {
	ObjectFinder
}

func (f objectOnlyFinder) FindPlayer(string) *game.CharacterInstance  { return nil }
func (f objectOnlyFinder) FindMob(string) *game.MobileInstance  { return nil }
func (f objectOnlyFinder) FindExit(string) (string, *assets.Exit) { return "", nil }

// playerOnlyFinder wraps a PlayerGroup into a full TargetFinder.
// FindPlayer searches members by name; mobs, objects, and exits always return nil.
type playerOnlyFinder struct {
	game.PlayerGroup
}

func (f playerOnlyFinder) FindPlayer(name string) *game.CharacterInstance {
	lower := strings.ToLower(name)
	var found *game.CharacterInstance
	f.ForEachPlayer(func(_ string, ps *game.CharacterInstance) {
		if found != nil || ps == nil {
			return
		}
		if strings.ToLower(ps.Character.Get().Name) == lower {
			found = ps
		}
	})
	return found
}

func (f playerOnlyFinder) FindObj(string) *game.ObjectInstance { return nil }
func (f playerOnlyFinder) FindMob(string) *game.MobileInstance   { return nil }
func (f playerOnlyFinder) FindExit(string) (string, *assets.Exit) { return "", nil }

// WorldScopes implements TargetScopes using a WorldView.
// It translates scope flags into the correct search spaces by looking up
// rooms, zones, inventory, and equipment from the world view.
type WorldScopes struct {
	world WorldView
}

// NewWorldScopes creates a TargetScopes backed by the given WorldView.
func NewWorldScopes(world WorldView) *WorldScopes {
	return &WorldScopes{world: world}
}

// SpacesFor returns search spaces for the given scope flags, ordered from
// narrowest (inventory) to broadest (world).
func (ws *WorldScopes) SpacesFor(scope Scope, ci *game.CharacterInstance) ([]SearchSpace, error) {
	zoneId, roomId := ci.Location()

	var spaces []SearchSpace

	if scope&ScopeInventory != 0 && ci.Inventory != nil {
		spaces = append(spaces, SearchSpace{
			Finder:  objectOnlyFinder{ci.Inventory},
			Remover: ci.Inventory,
		})
	}
	if scope&ScopeEquipment != 0 && ci.Equipment != nil {
		spaces = append(spaces, SearchSpace{
			Finder:  objectOnlyFinder{ci.Equipment},
			Remover: ci.Equipment,
		})
	}
	if scope&ScopeRoom != 0 {
		room := ws.world.GetRoom(zoneId, roomId)
		spaces = append(spaces, SearchSpace{
			Finder:  room,
			Remover: room,
		})
	}
	if scope&ScopeZone != 0 {
		zone := ws.world.GetZone(zoneId)
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
	if scope&ScopeGroup != 0 && ci.Group != nil {
		spaces = append(spaces, SearchSpace{
			Finder: playerOnlyFinder{ci.Group},
		})
	}

	return spaces, nil
}
