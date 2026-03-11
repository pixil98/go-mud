package commands

import (
	"github.com/pixil98/go-mud/internal/assets"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// --- Target types ---

// targetType is a bitmask representing the type of entity a target resolves to.
// Commands define target types as string slices in assets; this bitmask is the
// internal representation used for efficient matching during resolution.
type targetType int

const (
	targetTypePlayer targetType = 1 << iota
	targetTypeMobile
	targetTypeObject
	targetTypeExit

	targetTypeActor = targetTypePlayer | targetTypeMobile
)

// String returns the lowercase name of a single target type, or "target" for combined types.
func (tt targetType) String() string {
	switch tt {
	case targetTypePlayer:
		return "player"
	case targetTypeMobile:
		return "mobile"
	case targetTypeObject:
		return "object"
	case targetTypeExit:
		return "exit"
	default:
		return "target"
	}
}

// Label returns a human-readable label for the target type.
func (tt targetType) Label() string {
	return cases.Title(language.English).String(tt.String())
}

// parseTargetType converts string type names to a targetType bitmask.
func parseTargetType(types []string) targetType {
	var result targetType
	for _, s := range types {
		switch s {
		case assets.TargetPlayer:
			result |= targetTypePlayer
		case assets.TargetMobile:
			result |= targetTypeMobile
		case assets.TargetObject:
			result |= targetTypeObject
		case assets.TargetExit:
			result |= targetTypeExit
		}
	}
	return result
}

// --- Scopes ---

// scope is a bitmask for target resolution scopes.
// Commands define scopes as string slices in assets; this bitmask is the
// internal representation used for efficient matching during resolution.
type scope int

const (
	scopeWorld scope = 1 << iota
	scopeZone
	scopeRoom
	scopeInventory
	scopeEquipment
	scopeContents
	scopeGroup
)

// parseScope converts string scope names to a scope bitmask.
func parseScope(scopes []string) scope {
	var result scope
	for _, s := range scopes {
		switch s {
		case assets.ScopeRoom:
			result |= scopeRoom
		case assets.ScopeInventory:
			result |= scopeInventory
		case assets.ScopeEquipment:
			result |= scopeEquipment
		case assets.ScopeWorld:
			result |= scopeWorld
		case assets.ScopeZone:
			result |= scopeZone
		case assets.ScopeContents:
			result |= scopeContents
		case assets.ScopeGroup:
			result |= scopeGroup
		}
	}
	return result
}
