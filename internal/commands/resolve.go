package commands

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/display"
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

type ExitFinder interface {
	FindExit(string) (string, *game.Exit)
}

// TargetFinder combines all finder interfaces.
// RoomInstance and ZoneInstance satisfy this.
type TargetFinder interface {
	PlayerFinder
	ObjectFinder
	MobileFinder
	ExitFinder
}

// --- Source interfaces ---

// ObjectRemover can have objects removed from it.
// Inventory, Equipment, and RoomInstance all satisfy this.
type ObjectRemover interface {
	RemoveObj(instanceId string) *game.ObjectInstance
}

// --- Ref types ---

// actorCondition returns a description of an actor's health based on HP percentage.
func actorCondition(currentHP, maxHP int) string {
	if maxHP <= 0 {
		return "is in excellent condition"
	}
	pct := (currentHP * 100) / maxHP
	switch {
	case pct >= 100:
		return "is in excellent condition"
	case pct >= 90:
		return "has a few scratches"
	case pct >= 75:
		return "has some small wounds"
	case pct >= 50:
		return "has quite a few wounds"
	case pct >= 30:
		return "has some big nasty wounds"
	case pct >= 15:
		return "looks pretty hurt"
	default:
		return "is in awful condition"
	}
}

// describeActor returns a detailed description for a player or mob,
// including condition and equipped items.
func describeActor(description, name string, actor *game.ActorInstance) string {
	cName := display.Capitalize(name)
	lines := []string{display.Wrap(description)}
	lines = append(lines, fmt.Sprintf("%s %s.", cName, actorCondition(actor.CurrentHP, actor.MaxHP)))
	if eqLines := FormatEquippedItems(actor.Equipment); eqLines != nil {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s is using:", cName))
		lines = append(lines, eqLines...)
	}
	return strings.Join(lines, "\n")
}

// PlayerRef is the template-facing view of a resolved player.
type PlayerRef struct {
	CharId      storage.Identifier
	Name        string
	Description string
	session     *game.PlayerState
}

func playerRefFromState(ps *game.PlayerState) *PlayerRef {
	if ps == nil || ps.Character == nil {
		return nil
	}
	return &PlayerRef{
		CharId:      ps.CharId,
		Name:        ps.Character.Name,
		Description: ps.Character.DetailedDesc,
		session:     ps,
	}
}

// Describe returns a detailed description of the player, including equipped items.
func (r *PlayerRef) Describe() string {
	return describeActor(r.Description, r.Name, &r.session.Character.ActorInstance)
}

// MobileRef is the template-facing view of a resolved mob.
type MobileRef struct {
	InstanceId  string
	Name        string
	Description string
	instance    *game.MobileInstance
}

func mobRefFromInstance(mi *game.MobileInstance) *MobileRef {
	if mi == nil || mi.Mobile.Get() == nil {
		return nil
	}
	return &MobileRef{
		InstanceId:  mi.InstanceId,
		Name:        mi.Mobile.Get().ShortDesc,
		Description: mi.Mobile.Get().DetailedDesc,
		instance:    mi,
	}
}

// Describe returns a detailed description of the mob, including equipped items.
func (r *MobileRef) Describe() string {
	return describeActor(r.Description, r.Name, &r.instance.ActorInstance)
}

// ObjectRef is the template-facing view of a resolved object.
type ObjectRef struct {
	InstanceId  string
	Name        string
	Description string
	source      ObjectRemover
	instance    *game.ObjectInstance
}

func objRefFromInstance(oi *game.ObjectInstance, source ObjectRemover) *ObjectRef {
	if oi == nil || oi.Object.Get() == nil {
		return nil
	}
	return &ObjectRef{
		InstanceId:  oi.InstanceId,
		Name:        oi.Object.Get().ShortDesc,
		Description: oi.Object.Get().DetailedDesc,
		source:      source,
		instance:    oi,
	}
}

// ClosureName returns a capitalized display name for the object's closure.
// Uses the closure's Name if set, otherwise falls back to the object's ShortDesc.
func (r *ObjectRef) ClosureName() string {
	def := r.instance.Object.Get()
	name := r.Name
	if def.Closure != nil && def.Closure.Name != "" {
		name = def.Closure.Name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// Describe returns a detailed description of the object, including container contents.
func (r *ObjectRef) Describe() string {
	lines := []string{display.Wrap(r.Description)}
	if r.instance.Object.Get().HasFlag(game.ObjectFlagContainer) {
		lines = append(lines, "")
		if r.instance.Locked {
			lines = append(lines, "It is locked.")
		} else if r.instance.Closed {
			lines = append(lines, "It is closed.")
		} else {
			lines = append(lines, "It contains:")
			lines = append(lines, FormatInventoryItems(r.instance.Contents)...)
		}
	}
	return strings.Join(lines, "\n")
}

// ExitRef is the template-facing view of a resolved exit.
type ExitRef struct {
	Direction string     // Direction key (e.g., "north", "south")
	exit      *game.Exit // The exit definition
}

func exitRefFrom(direction string, exit *game.Exit) *ExitRef {
	return &ExitRef{
		Direction: direction,
		exit:      exit,
	}
}

// TargetRef is a polymorphic target reference that could be a player, mobile, object, or exit.
type TargetRef struct {
	Type   TargetType // "player", "mobile", "object", or "exit"
	Player *PlayerRef // Non-nil if Type == "player"
	Mob    *MobileRef // Non-nil if Type == "mobile"
	Obj    *ObjectRef // Non-nil if Type == "object"
	Exit   *ExitRef   // Non-nil if Type == "exit"
}

// Name returns the display name of the target regardless of type.
func (r *TargetRef) Name() string {
	switch {
	case r.Player != nil:
		return r.Player.Name
	case r.Mob != nil:
		return r.Mob.Name
	case r.Obj != nil:
		return r.Obj.Name
	case r.Exit != nil:
		return r.Exit.Direction
	default:
		return ""
	}
}

// --- SearchSpace and FindTarget ---

// SearchSpace pairs a TargetFinder with an optional ObjectRemover.
// The Remover is used as the Source in ObjectRef when an object is found.
type SearchSpace struct {
	Finder  TargetFinder
	Remover ObjectRemover // optional, used for ObjectRef.Source
}

// FindTarget searches spaces in order for the first matching target.
// It checks player, then mobile, then object, then exit within each space,
// filtering by the allowed target types. Returns the first match.
func FindTarget(name string, tt TargetType, spaces []SearchSpace) (*TargetRef, error) {
	for _, sp := range spaces {
		if tt&TargetTypePlayer != 0 {
			if ps := sp.Finder.FindPlayer(name); ps != nil {
				return &TargetRef{
					Type:   TargetTypePlayer,
					Player: playerRefFromState(ps),
				}, nil
			}
		}
		if tt&TargetTypeMobile != 0 {
			if mi := sp.Finder.FindMob(name); mi != nil {
				return &TargetRef{
					Type: TargetTypeMobile,
					Mob:  mobRefFromInstance(mi),
				}, nil
			}
		}
		if tt&TargetTypeObject != 0 {
			if oi := sp.Finder.FindObj(name); oi != nil {
				return &TargetRef{
					Type: TargetTypeObject,
					Obj:  objRefFromInstance(oi, sp.Remover),
				}, nil
			}
		}
		if tt&TargetTypeExit != 0 {
			if dir, exit := sp.Finder.FindExit(name); exit != nil {
				return &TargetRef{
					Type: TargetTypeExit,
					Exit: exitRefFrom(dir, exit),
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

// notFoundContext is the template context for TargetSpec.NotFound templates.
type notFoundContext struct {
	Inputs map[string]any // All parsed player inputs
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
			ref, err := findWithNotFound(name, spec, spaces, inputs)
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

		ref, err := findWithNotFound(name, spec, spaces, inputs)
		if err != nil {
			return nil, err
		}
		targets[spec.Name] = ref
	}

	return targets, nil
}

// findWithNotFound wraps FindTarget and replaces the default error with the
// spec's NotFound template when one is configured.
func findWithNotFound(name string, spec TargetSpec, spaces []SearchSpace, inputs map[string]any) (*TargetRef, error) {
	ref, err := FindTarget(name, spec.TargetType(), spaces)
	if err != nil && spec.NotFound != "" {
		msg, tmplErr := ExpandTemplate(spec.NotFound, &notFoundContext{Inputs: inputs})
		if tmplErr != nil {
			return nil, fmt.Errorf("expanding not_found template for target %q: %w", spec.Name, tmplErr)
		}
		return nil, NewUserError(msg)
	}
	return ref, err
}

// containerSpaces checks if a spec has a scope_target and returns container-only
// search spaces if the referenced target resolved to a container object.
// Returns (spaces, handled, error) where handled=true means container scoping applies.
func containerSpaces(spec TargetSpec, targets map[string]*TargetRef) ([]SearchSpace, bool, error) {
	if spec.ScopeTarget == "" {
		return nil, false, nil
	}

	scopeRef := targets[spec.ScopeTarget]
	if scopeRef == nil || scopeRef.Obj == nil || scopeRef.Obj.instance == nil {
		// Scope target not resolved (likely optional) — fall through to normal scopes
		return nil, false, nil
	}

	// Validate it's a container
	if !scopeRef.Obj.instance.Object.Get().HasFlag(game.ObjectFlagContainer) {
		capName := strings.ToUpper(scopeRef.Obj.Name[:1]) + scopeRef.Obj.Name[1:]
		return nil, false, NewUserError(fmt.Sprintf("%s is not a container.", capName))
	}

	// Check closure state
	if scopeRef.Obj.instance.Locked {
		return nil, false, NewUserError(fmt.Sprintf("%s is locked.", scopeRef.Obj.ClosureName()))
	}
	if scopeRef.Obj.instance.Closed {
		return nil, false, NewUserError(fmt.Sprintf("%s is closed.", scopeRef.Obj.ClosureName()))
	}

	// Resolve exclusively from container contents
	contents := scopeRef.Obj.instance.Contents
	if contents == nil {
		return []SearchSpace{}, true, nil
	}

	return []SearchSpace{
		{Finder: objectOnlyFinder{contents}, Remover: contents},
	}, true, nil
}
