package commands

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// --- Finder interfaces ---

type PlayerFinder interface {
	FindPlayer(string) *game.PlayerState
}

type ObjectFinder interface {
	FindObj(string) *game.ObjectInstance
}

type MobileFinder interface {
	FindMob(string) *game.MobileInstance
}

// TargetFinder combines all finder interfaces.
// RoomInstance and ZoneInstance satisfy this.
type TargetFinder interface {
	PlayerFinder
	ObjectFinder
	MobileFinder
}

// --- Source interfaces ---

// ObjectRemover can have objects removed from it.
// Inventory, Equipment, and RoomInstance all satisfy this.
type ObjectRemover interface {
	RemoveObj(instanceId string) *game.ObjectInstance
}

// ObjectHolder can have objects added and removed.
// Inventory and RoomInstance satisfy this. Equipment does not (it uses Equip).
type ObjectHolder interface {
	ObjectRemover
	AddObj(obj *game.ObjectInstance)
}

// --- Ref types ---

// PlayerRef is the template-facing view of a resolved player.
type PlayerRef struct {
	CharId      storage.Identifier
	Name        string
	Description string
}

func PlayerRefFromState(ps *game.PlayerState) *PlayerRef {
	if ps == nil || ps.Character == nil {
		return nil
	}
	return &PlayerRef{
		CharId:      ps.CharId,
		Name:        ps.Character.Name,
		Description: ps.Character.DetailedDesc,
	}
}

// MobileRef is the template-facing view of a resolved mob.
type MobileRef struct {
	InstanceId  string
	Name        string
	Description string
	Instance    *game.MobileInstance
}

func MobRefFromInstance(mi *game.MobileInstance) *MobileRef {
	if mi == nil || mi.Definition == nil {
		return nil
	}
	return &MobileRef{
		InstanceId:  mi.InstanceId,
		Name:        mi.Definition.ShortDesc,
		Description: mi.Definition.DetailedDesc,
		Instance:    mi,
	}
}

// ObjectRef is the template-facing view of a resolved object.
type ObjectRef struct {
	InstanceId  string
	ObjectId    storage.Identifier
	Name        string
	Description string
	Source      ObjectRemover
	Instance    *game.ObjectInstance
}

func ObjRefFromInstance(oi *game.ObjectInstance, source ObjectRemover) *ObjectRef {
	if oi == nil || oi.Definition == nil {
		return nil
	}
	return &ObjectRef{
		InstanceId:  oi.InstanceId,
		ObjectId:    oi.ObjectId,
		Name:        oi.Definition.ShortDesc,
		Description: oi.Definition.DetailedDesc,
		Source:      source,
		Instance:    oi,
	}
}

// TargetRef is a polymorphic target reference that could be a player, mobile, or object.
type TargetRef struct {
	Type   TargetType // "player", "mobile", or "object"
	Player *PlayerRef // Non-nil if Type == "player"
	Mob    *MobileRef // Non-nil if Type == "mobile"
	Obj    *ObjectRef // Non-nil if Type == "object"
}

// --- SearchSpace and FindTarget ---

// SearchSpace pairs a TargetFinder with an optional ObjectRemover.
// The Remover is used as the Source in ObjectRef when an object is found.
type SearchSpace struct {
	Finder  TargetFinder
	Remover ObjectRemover // optional, used for ObjectRef.Source
}

// FindTarget searches spaces in order for the first matching target.
// It checks player, then mobile, then object within each space, filtering
// by the allowed target types. Returns the first match.
func FindTarget(name string, tt TargetType, spaces []SearchSpace) (*TargetRef, error) {
	for _, sp := range spaces {
		if tt&TargetTypePlayer != 0 {
			if ps := sp.Finder.FindPlayer(name); ps != nil {
				return &TargetRef{
					Type:   TargetTypePlayer,
					Player: PlayerRefFromState(ps),
				}, nil
			}
		}
		if tt&TargetTypeMobile != 0 {
			if mi := sp.Finder.FindMob(name); mi != nil {
				return &TargetRef{
					Type: TargetTypeMobile,
					Mob:  MobRefFromInstance(mi),
				}, nil
			}
		}
		if tt&TargetTypeObject != 0 {
			if oi := sp.Finder.FindObj(name); oi != nil {
				return &TargetRef{
					Type: TargetTypeObject,
					Obj:  ObjRefFromInstance(oi, sp.Remover),
				}, nil
			}
		}
	}
	return nil, NewUserError(fmt.Sprintf("%s %q not found.", tt.Label(), name))
}

// --- TargetScopes ---

// TargetScopes maps scope flags to search spaces for a given actor and session.
// Implementations decide where to look (room, zone, world, inventory, etc.)
// without coupling the resolver to any particular game state type.
type TargetScopes interface {
	SpacesFor(scope Scope, actor *game.Character, session *game.PlayerState) ([]SearchSpace, error)
}

// --- TargetResolver ---

// TargetResolver resolves command target specs into TargetRefs using a TargetScopes.
type TargetResolver struct {
	scopes TargetScopes
}

// NewTargetResolver creates a TargetResolver backed by the given TargetScopes.
func NewTargetResolver(scopes TargetScopes) *TargetResolver {
	return &TargetResolver{scopes: scopes}
}

// ResolveSpecs resolves all targets from the command's targets section.
// Specs are processed in order so that scope_target references to earlier
// targets work correctly. Inputs are assumed to have been validated by parseInputs.
func (r *TargetResolver) ResolveSpecs(specs []TargetSpec, inputs map[string]any, actor *game.Character, session *game.PlayerState) (map[string]*TargetRef, error) {
	if len(specs) == 0 {
		return make(map[string]*TargetRef), nil
	}

	targets := make(map[string]*TargetRef, len(specs))

	for _, spec := range specs {
		// Get the input value; inputs were already parsed and validated.
		name, _ := inputs[spec.Input].(string)
		if name == "" {
			if spec.Optional {
				targets[spec.Name] = nil
				continue
			}
			return nil, NewUserError(fmt.Sprintf("Input %q is required.", spec.Input))
		}

		// Handle container scope_target
		if spaces, handled, err := containerSpaces(spec, targets); err != nil {
			return nil, err
		} else if handled {
			ref, err := FindTarget(name, spec.TargetType(), spaces)
			if err != nil {
				return nil, err
			}
			targets[spec.Name] = ref
			continue
		}

		// Normal scope resolution
		scope := spec.Scope()
		spaces, err := r.scopes.SpacesFor(scope, actor, session)
		if err != nil {
			return nil, err
		}

		ref, err := FindTarget(name, spec.TargetType(), spaces)
		if err != nil {
			return nil, err
		}
		targets[spec.Name] = ref
	}

	return targets, nil
}

// containerSpaces checks if a spec has a scope_target and returns container-only
// search spaces if the referenced target resolved to a container object.
// Returns (spaces, handled, error) where handled=true means container scoping applies.
func containerSpaces(spec TargetSpec, targets map[string]*TargetRef) ([]SearchSpace, bool, error) {
	if spec.ScopeTarget == "" {
		return nil, false, nil
	}

	scopeRef := targets[spec.ScopeTarget]
	if scopeRef == nil || scopeRef.Obj == nil || scopeRef.Obj.Instance == nil {
		// Scope target not resolved (likely optional) — fall through to normal scopes
		return nil, false, nil
	}

	// Validate it's a container
	if scopeRef.Obj.Instance.Definition == nil || !scopeRef.Obj.Instance.Definition.HasFlag(game.ObjectFlagContainer) {
		capName := strings.ToUpper(scopeRef.Obj.Name[:1]) + scopeRef.Obj.Name[1:]
		return nil, false, NewUserError(fmt.Sprintf("%s is not a container.", capName))
	}

	// Resolve exclusively from container contents
	contents := scopeRef.Obj.Instance.Contents
	if contents == nil {
		return []SearchSpace{}, true, nil
	}

	return []SearchSpace{
		{Finder: objectOnlyFinder{contents}, Remover: contents},
	}, true, nil
}
