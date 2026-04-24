package commands

import (
	"github.com/pixil98/go-mud/internal/game"
)

// objectOnlyFinder wraps an ObjectFinder (like Inventory or Equipment)
// into a full TargetFinder. Players and mobs always return empty.
type objectOnlyFinder struct {
	ObjectFinder
}

func (f objectOnlyFinder) FindPlayers(func(*game.CharacterInstance) bool) []*game.CharacterInstance {
	return nil
}
func (f objectOnlyFinder) FindMobs(func(*game.MobileInstance) bool) []*game.MobileInstance {
	return nil
}
func (f objectOnlyFinder) FindExit(string) (string, *game.ResolvedExit) { return "", nil }

// playerOnlyFinder wraps a PlayerGroup into a full TargetFinder.
// FindPlayers searches members via the matcher; mobs, objects, and exits always return empty.
type playerOnlyFinder struct {
	game.PlayerGroup
}

func (f playerOnlyFinder) FindPlayers(match func(*game.CharacterInstance) bool) []*game.CharacterInstance {
	var out []*game.CharacterInstance
	f.ForEachPlayer(func(_ string, ci *game.CharacterInstance) {
		if ci != nil && match(ci) {
			out = append(out, ci)
		}
	})
	return out
}

func (f playerOnlyFinder) FindObjs(func(*game.ObjectInstance) bool) []*game.ObjectInstance { return nil }
func (f playerOnlyFinder) FindMobs(func(*game.MobileInstance) bool) []*game.MobileInstance { return nil }
func (f playerOnlyFinder) FindExit(string) (string, *game.ResolvedExit)                    { return "", nil }

// darkRoomFinder wraps a room so that all room-scope target lookups fail in
// the dark. Movement isn't affected because the move handler calls FindExit
// directly on the room, bypassing target resolution.
type darkRoomFinder struct{}

func (darkRoomFinder) FindPlayers(func(*game.CharacterInstance) bool) []*game.CharacterInstance {
	return nil
}
func (darkRoomFinder) FindMobs(func(*game.MobileInstance) bool) []*game.MobileInstance { return nil }
func (darkRoomFinder) FindObjs(func(*game.ObjectInstance) bool) []*game.ObjectInstance { return nil }
func (darkRoomFinder) FindExit(string) (string, *game.ResolvedExit)                    { return "", nil }

// followerFinder searches the actor's followers list. When groupedOnly is
// true, only grouped followers are included. Satisfies TargetFinder by
// checking each follower's type (player or mob) against the matcher.
type followerFinder struct {
	actor       game.Actor
	groupedOnly bool
}

func (f followerFinder) followers() []game.Actor {
	if f.groupedOnly {
		return f.actor.GroupedFollowers()
	}
	return f.actor.Followers()
}

func (f followerFinder) FindPlayers(match func(*game.CharacterInstance) bool) []*game.CharacterInstance {
	followers := f.followers()
	out := make([]*game.CharacterInstance, 0, len(followers))
	for _, ft := range followers {
		if ci, ok := ft.(*game.CharacterInstance); ok && match(ci) {
			out = append(out, ci)
		}
	}
	return out
}

func (f followerFinder) FindMobs(match func(*game.MobileInstance) bool) []*game.MobileInstance {
	followers := f.followers()
	out := make([]*game.MobileInstance, 0, len(followers))
	for _, ft := range followers {
		if mi, ok := ft.(*game.MobileInstance); ok && match(mi) {
			out = append(out, mi)
		}
	}
	return out
}

func (f followerFinder) FindObjs(func(*game.ObjectInstance) bool) []*game.ObjectInstance { return nil }
func (f followerFinder) FindExit(string) (string, *game.ResolvedExit)                    { return "", nil }

// WorldScopes translates scope flags into search spaces by navigating
// from the actor's room pointer.
type WorldScopes struct{}

// NewWorldScopes creates a WorldScopes.
func NewWorldScopes() *WorldScopes {
	return &WorldScopes{}
}

// SpacesFor returns search spaces for the given scope flags, ordered from
// narrowest (inventory) to broadest (world).
func (ws *WorldScopes) SpacesFor(s scope, actor game.Actor) ([]SearchSpace, error) {

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
		var finder TargetFinder = room
		if !CanSee(actor, room) {
			finder = darkRoomFinder{}
		}
		spaces = append(spaces, SearchSpace{
			Finder:  finder,
			Remover: room,
		})
	}
	if s&scopeFollowers != 0 {
		spaces = append(spaces, SearchSpace{
			Finder: followerFinder{actor: actor},
		})
	}
	if s&scopeGroupedFollowers != 0 {
		spaces = append(spaces, SearchSpace{
			Finder: followerFinder{actor: actor, groupedOnly: true},
		})
	}
	if s&scopeGroup != 0 {
		if leader := game.GroupLeader(actor); leader != nil {
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
