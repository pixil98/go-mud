package commands

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// Scope defines where to look for targets. Can be combined with bitwise OR.
type Scope int

const (
	ScopeRoom      Scope = 1 << iota // Players/mobs/items in current room
	ScopeInventory                   // Items in actor's inventory
	ScopeWorld                       // All online players
	ScopeZone                        // Players in current zone
)

// ScopeFromString converts a scope string to a Scope value.
func ScopeFromString(s string) Scope {
	switch strings.ToLower(s) {
	case "room":
		return ScopeRoom
	case "inventory":
		return ScopeInventory
	case "world":
		return ScopeWorld
	case "zone":
		return ScopeZone
	default:
		return 0
	}
}

// ScopesFromConfig parses scope from config, which can be a string or []string.
func ScopesFromConfig(v any) Scope {
	switch s := v.(type) {
	case string:
		return ScopeFromString(s)
	case []any:
		var combined Scope
		for _, item := range s {
			if str, ok := item.(string); ok {
				combined |= ScopeFromString(str)
			}
		}
		return combined
	default:
		return 0
	}
}

// EntityType defines what kind of entity to resolve.
type EntityType string

const (
	EntityPlayer EntityType = "player"
	EntityMob    EntityType = "mob"
	EntityItem   EntityType = "item"
	EntityTarget EntityType = "target" // Polymorphic: tries player, mob, item
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
// Returns *PlayerRef, *MobRef, *ItemRef, or *TargetRef based on entityType.
func (r *Resolver) Resolve(actorState *game.PlayerState, name string, entityType EntityType, scope Scope) (any, error) {
	switch entityType {
	case EntityPlayer:
		return r.resolvePlayer(actorState, name, scope)
	case EntityMob:
		return r.resolveMob(actorState, name, scope)
	case EntityItem:
		return r.resolveItem(actorState, name, scope)
	case EntityTarget:
		return r.resolveTarget(actorState, name, scope)
	default:
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}
}

// resolvePlayer resolves a player by name within the given scope.
func (r *Resolver) resolvePlayer(actorState *game.PlayerState, name string, scope Scope) (*PlayerRef, error) {
	nameLower := strings.ToLower(name)
	actorZone, actorRoom := actorState.Location()

	for charId, char := range r.world.Characters().GetAll() {
		if strings.ToLower(char.Name) != nameLower {
			continue
		}

		// Check if player is online
		state := r.world.GetPlayer(charId)
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

		return PlayerRefFrom(char), nil
	}

	return nil, NewUserError(fmt.Sprintf("Player '%s' not found", name))
}

// resolveMob resolves a mob by name within the given scope.
func (r *Resolver) resolveMob(actorState *game.PlayerState, name string, scope Scope) (*MobRef, error) {
	// TODO: Implement mob resolution
	return nil, NewUserError(fmt.Sprintf("Mob '%s' not found", name))
}

// resolveItem resolves an item by name within the given scope.
func (r *Resolver) resolveItem(actorState *game.PlayerState, name string, scope Scope) (*ItemRef, error) {
	// TODO: Implement item resolution
	return nil, NewUserError(fmt.Sprintf("Item '%s' not found", name))
}

// resolveTarget tries to resolve as player, then mob, then item.
func (r *Resolver) resolveTarget(actorState *game.PlayerState, name string, scope Scope) (*TargetRef, error) {
	// Try player first
	if player, err := r.resolvePlayer(actorState, name, scope); err == nil {
		return &TargetRef{
			Type:   "player",
			Player: player,
			Name:   player.Name,
		}, nil
	}

	// Try mob
	if mob, err := r.resolveMob(actorState, name, scope); err == nil {
		return &TargetRef{
			Type: "mob",
			Mob:  mob,
			Name: mob.Name,
		}, nil
	}

	// Try item
	if item, err := r.resolveItem(actorState, name, scope); err == nil {
		return &TargetRef{
			Type: "item",
			Item: item,
			Name: item.Name,
		}, nil
	}

	return nil, NewUserError(fmt.Sprintf("Target '%s' not found", name))
}

// ResolveDirective represents a $resolve directive from config.
type ResolveDirective struct {
	Resolve  EntityType
	Scope    Scope
	Input    string
	Optional bool
}

// IsResolveDirective checks if a config value is a $resolve directive.
// Returns the parsed directive if valid, nil otherwise.
func IsResolveDirective(v any) (*ResolveDirective, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}

	resolve, hasResolve := m["$resolve"].(string)
	if !hasResolve {
		return nil, false
	}

	directive := &ResolveDirective{
		Resolve: EntityType(resolve),
	}

	if scopeVal, ok := m["$scope"]; ok {
		directive.Scope = ScopesFromConfig(scopeVal)
	}
	if input, ok := m["$input"].(string); ok {
		directive.Input = input
	}
	if optional, ok := m["$optional"].(bool); ok {
		directive.Optional = optional
	}

	return directive, true
}
