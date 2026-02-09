package commands

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// EntityType defines what kind of entity to resolve.
type EntityType string

const (
	EntityPlayer EntityType = "player"
	EntityMob    EntityType = "mobile"
	EntityObj    EntityType = "object"
	EntityTarget EntityType = "target" // Polymorphic: tries player, mobile, object
)

// Resolver resolves target names to game entities.
// Used by the framework to process $resolve directives.
type Resolver struct {
	world *game.WorldState
}

// NewResolver creates a new Resolver.
func NewResolver(world *game.WorldState) *Resolver {
	return &Resolver{world: world}
}

// Resolve resolves a target name to an entity based on type and scope.
// Returns *PlayerRef, *MobileRef, *ObjectRef, or *TargetRef based on entityType.
func (r *Resolver) Resolve(charId storage.Identifier, name string, entityType EntityType, scope Scope) (any, error) {
	switch entityType {
	case EntityPlayer:
		return r.resolvePlayer(charId, name, scope)
	case EntityMob:
		return r.resolveMob(charId, name, scope)
	case EntityObj:
		return r.resolveObject(charId, name, scope)
	case EntityTarget:
		return r.resolveTarget(charId, name, scope)
	default:
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}
}

// resolvePlayer resolves a player by name within the given scope.
func (r *Resolver) resolvePlayer(charId storage.Identifier, name string, scope Scope) (*PlayerRef, error) {
	nameLower := strings.ToLower(name)
	actorState := r.world.GetPlayer(charId)
	if actorState == nil {
		return nil, fmt.Errorf("actor not found: %s", charId)
	}
	actorZone, actorRoom := actorState.Location()

	for targetCharId, char := range r.world.Characters().GetAll() {
		if strings.ToLower(char.Name) != nameLower {
			continue
		}

		// Check if player is online
		state := r.world.GetPlayer(targetCharId)
		if state == nil {
			continue
		}

		// Check scope (any matching scope allows the match)
		playerZone, playerRoom := state.Location()
		matches := false

		if scope&ScopeWorld != 0 {
			matches = true
		}
		if scope&ScopeZone != 0 && playerZone == actorZone {
			matches = true
		}
		if scope&ScopeRoom != 0 && playerZone == actorZone && playerRoom == actorRoom {
			matches = true
		}

		if !matches && scope != 0 {
			continue
		}

		return PlayerRefFrom(targetCharId, char), nil
	}

	return nil, NewUserError(fmt.Sprintf("Player '%s' not found", name))
}

// resolveMob resolves a mob by name within the given scope.
func (r *Resolver) resolveMob(charId storage.Identifier, name string, scope Scope) (*MobileRef, error) {
	nameLower := strings.ToLower(name)
	actorState := r.world.GetPlayer(charId)
	if actorState == nil {
		return nil, fmt.Errorf("actor not found: %s", charId)
	}
	actorZone, actorRoom := actorState.Location()

	// matchMob checks if a mob matches the given name by alias
	matchMob := func(mi *game.MobileInstance) *MobileRef {
		mob := r.world.Mobiles().Get(string(mi.MobileId))
		if mob == nil {
			return nil
		}
		for _, alias := range mob.Aliases {
			if strings.ToLower(alias) == nameLower {
				return MobRefFrom(mob, mi)
			}
		}
		return nil
	}

	// Check room scope
	if scope&ScopeRoom != 0 {
		for _, mi := range r.world.GetMobilesInRoom(actorZone, actorRoom) {
			if ref := matchMob(mi); ref != nil {
				return ref, nil
			}
		}
	}

	// Check zone scope (all rooms in current zone)
	if scope&ScopeZone != 0 {
		for roomId, room := range r.world.Rooms().GetAll() {
			if room.ZoneId != string(actorZone) {
				continue
			}
			for _, mi := range r.world.GetMobilesInRoom(actorZone, storage.Identifier(roomId)) {
				if ref := matchMob(mi); ref != nil {
					return ref, nil
				}
			}
		}
	}

	// Check world scope (all rooms in all zones)
	if scope&ScopeWorld != 0 {
		for roomId, room := range r.world.Rooms().GetAll() {
			zoneId := storage.Identifier(room.ZoneId)
			for _, mi := range r.world.GetMobilesInRoom(zoneId, storage.Identifier(roomId)) {
				if ref := matchMob(mi); ref != nil {
					return ref, nil
				}
			}
		}
	}

	return nil, NewUserError(fmt.Sprintf("Mob '%s' not found", name))
}

// resolveObject resolves an object by name within the given scope.
func (r *Resolver) resolveObject(charId storage.Identifier, name string, scope Scope) (*ObjectRef, error) {
	nameLower := strings.ToLower(name)
	actorState := r.world.GetPlayer(charId)
	if actorState == nil {
		return nil, fmt.Errorf("actor not found: %s", charId)
	}
	actorZone, actorRoom := actorState.Location()

	// matchObject checks if an object matches the given name by alias
	matchObject := func(oi *game.ObjectInstance) *ObjectRef {
		obj := r.world.Objects().Get(string(oi.ObjectId))
		if obj == nil {
			return nil
		}
		for _, alias := range obj.Aliases {
			if strings.ToLower(alias) == nameLower {
				return ObjectRefFrom(obj, oi)
			}
		}
		return nil
	}

	// Check inventory scope
	if scope&ScopeInventory != 0 {
		actor := r.world.Characters().Get(string(charId))
		if actor != nil && actor.Inventory != nil {
			for _, oi := range actor.Inventory.Items {
				if ref := matchObject(oi); ref != nil {
					return ref, nil
				}
			}
		}
	}

	// Check room scope
	if scope&ScopeRoom != 0 {
		for _, oi := range r.world.GetObjectsInRoom(actorZone, actorRoom) {
			if ref := matchObject(oi); ref != nil {
				return ref, nil
			}
		}
	}

	// Check zone scope (all rooms in current zone)
	if scope&ScopeZone != 0 {
		for roomId, room := range r.world.Rooms().GetAll() {
			if room.ZoneId != string(actorZone) {
				continue
			}
			for _, oi := range r.world.GetObjectsInRoom(actorZone, storage.Identifier(roomId)) {
				if ref := matchObject(oi); ref != nil {
					return ref, nil
				}
			}
		}
	}

	// Check world scope (all rooms in all zones)
	if scope&ScopeWorld != 0 {
		for roomId, room := range r.world.Rooms().GetAll() {
			zoneId := storage.Identifier(room.ZoneId)
			for _, oi := range r.world.GetObjectsInRoom(zoneId, storage.Identifier(roomId)) {
				if ref := matchObject(oi); ref != nil {
					return ref, nil
				}
			}
		}
	}

	return nil, NewUserError(fmt.Sprintf("Object '%s' not found", name))
}

// resolveTarget tries to resolve as player, then mobile, then object.
func (r *Resolver) resolveTarget(charId storage.Identifier, name string, scope Scope) (*TargetRef, error) {
	// Try player first
	if player, err := r.resolvePlayer(charId, name, scope); err == nil {
		return &TargetRef{
			Type:   "player",
			Player: player,
			Name:   player.Name,
		}, nil
	}

	// Try mobile
	if mob, err := r.resolveMob(charId, name, scope); err == nil {
		return &TargetRef{
			Type: "mobile",
			Mob:  mob,
			Name: mob.Name,
		}, nil
	}

	// Try object
	if obj, err := r.resolveObject(charId, name, scope); err == nil {
		return &TargetRef{
			Type: "object",
			Obj:  obj,
			Name: obj.Name,
		}, nil
	}

	return nil, NewUserError(fmt.Sprintf("Target '%s' not found", name))
}
