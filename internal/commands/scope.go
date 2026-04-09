package commands

import (
	"strings"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
)

// objectOnlyFinder wraps an ObjectFinder (like Inventory or Equipment)
// into a full TargetFinder. FindPlayer and FindMob always return nil.
type objectOnlyFinder struct {
	ObjectFinder
}

func (f objectOnlyFinder) FindPlayer(string) *game.CharacterInstance { return nil }
func (f objectOnlyFinder) FindMob(string) *game.MobileInstance       { return nil }
func (f objectOnlyFinder) FindExit(string) (string, *game.ResolvedExit) { return "", nil }

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
func (f playerOnlyFinder) FindExit(string) (string, *game.ResolvedExit) { return "", nil }

// WorldScopes translates scope flags into search spaces by navigating
// from the actor's room pointer.
type WorldScopes struct{}

// NewWorldScopes creates a WorldScopes.
func NewWorldScopes() *WorldScopes {
	return &WorldScopes{}
}

// SpacesFor returns search spaces for the given scope flags, ordered from
// narrowest (inventory) to broadest (world).
func (ws *WorldScopes) SpacesFor(s scope, actor shared.Actor) ([]SearchSpace, error) {

	var spaces []SearchSpace

	if s&scopeInventory != 0 {
		if i := actor.Inventory(); i != nil {
			spaces = append(spaces, SearchSpace{
				Finder:  objectOnlyFinder{i},
				Remover: i,
			})
		}
	}
	if s&scopeEquipment != 0 {
		if eq := actor.Equipment(); eq != nil {
			spaces = append(spaces, SearchSpace{
				Finder:  objectOnlyFinder{eq},
				Remover: eq,
			})
		}
	}
	if s&scopeRoom != 0 {
		room := actor.Room()
		spaces = append(spaces, SearchSpace{
			Finder:  room,
			Remover: room,
		})
	}
	if s&scopeGroup != 0 {
		if leader := groupLeader(actor); leader != nil {
			spaces = append(spaces, SearchSpace{
				Finder: playerOnlyFinder{game.GroupPublishTarget(leader)},
			})
		}
	}
	if s&scopeZone != 0 {
		spaces = append(spaces, SearchSpace{
			Finder: actor.Room().Zone(),
		})
	}
	if s&scopeWorld != 0 {
		for _, zi := range actor.Room().Zone().World().Instances() {
			spaces = append(spaces, SearchSpace{
				Finder: zi,
			})
		}
	}

	return spaces, nil
}
