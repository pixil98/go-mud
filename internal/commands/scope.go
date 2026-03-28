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

func (f objectOnlyFinder) FindPlayer(string) *game.CharacterInstance { return nil }
func (f objectOnlyFinder) FindMob(string) *game.MobileInstance       { return nil }
func (f objectOnlyFinder) FindExit(string) (string, *assets.Exit)    { return "", nil }

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
		if strings.ToLower(ps.Name()) == lower {
			found = ps
		}
	})
	return found
}

func (f playerOnlyFinder) FindObj(string) *game.ObjectInstance    { return nil }
func (f playerOnlyFinder) FindMob(string) *game.MobileInstance    { return nil }
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
func (ws *WorldScopes) SpacesFor(s scope, actor ScopeActor) ([]SearchSpace, error) {
	zoneId, roomId := actor.Location()

	var spaces []SearchSpace

	if s&scopeInventory != 0 {
		if i := actor.GetInventory(); i != nil {
			spaces = append(spaces, SearchSpace{
				Finder:  objectOnlyFinder{i},
				Remover: i,
			})
		}
	}
	if s&scopeEquipment != 0 {
		if eq := actor.GetEquipment(); eq != nil {
			spaces = append(spaces, SearchSpace{
				Finder:  objectOnlyFinder{eq},
				Remover: eq,
			})
		}
	}
	if s&scopeRoom != 0 {
		room := ws.world.GetZone(zoneId).GetRoom(roomId)
		spaces = append(spaces, SearchSpace{
			Finder:  room,
			Remover: room,
		})
	}
	if s&scopeGroup != 0 {
		if grp := actor.GetGroup(); grp != nil {
			spaces = append(spaces, SearchSpace{
				Finder: playerOnlyFinder{grp},
			})
		}
	}
	if s&scopeZone != 0 {
		zone := ws.world.GetZone(zoneId)
		spaces = append(spaces, SearchSpace{
			Finder: zone,
		})
	}
	if s&scopeWorld != 0 {
		for _, zi := range ws.world.Instances() {
			spaces = append(spaces, SearchSpace{
				Finder: zi,
			})
		}
	}

	return spaces, nil
}
