package commands

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
)

// --- Finder interfaces ---

// PlayerFinder finds a player by name within a search scope.
type PlayerFinder interface {
	FindPlayer(string) *game.CharacterInstance
}

// ObjectFinder finds an object by name within a search scope.
type ObjectFinder interface {
	FindObj(string) *game.ObjectInstance
}

// MobileFinder finds a mobile by name within a search scope.
type MobileFinder interface {
	FindMob(string) *game.MobileInstance
}

// ExitFinder finds an exit by direction name within a search scope.
type ExitFinder interface {
	FindExit(string) (string, *game.ResolvedExit)
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
func describeActor(description, name string, currentHP, maxHP int, equipment *game.Equipment) string {
	cName := display.Capitalize(name)
	lines := []string{display.Wrap(description)}
	lines = append(lines, fmt.Sprintf("%s %s.", cName, actorCondition(currentHP, maxHP)))
	if eqLines := FormatEquippedItems(equipment); eqLines != nil {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s is using:", cName))
		lines = append(lines, eqLines...)
	}
	return strings.Join(lines, "\n")
}

// ActorRef is the template-facing view of a resolved player or mob.
type ActorRef struct {
	actor       shared.Actor
	CharId      string // charId for players (used for message routing); empty for mobs
	Name        string
	Description string
}

// Actor returns the underlying shared.Actor.
func (r *ActorRef) Actor() shared.Actor { return r.actor }

// Inventory returns the actor's inventory.
func (r *ActorRef) Inventory() *game.Inventory { return r.actor.Inventory() }

func actorRefFromPlayer(ci *game.CharacterInstance) *ActorRef {
	return &ActorRef{
		actor:       ci,
		CharId:      ci.Id(),
		Name:        ci.Name(),
		Description: ci.Character.Get().DetailedDesc,
	}
}

func actorRefFromMob(mi *game.MobileInstance) *ActorRef {
	if mi == nil || mi.Mobile.Get() == nil {
		return nil
	}
	return &ActorRef{
		actor:       mi,
		Name:        mi.Name(),
		Description: mi.Mobile.Get().DetailedDesc,
	}
}

func actorRefFromActor(a shared.Actor) *ActorRef {
	ref := &ActorRef{
		actor: a,
		Name:  a.Name(),
	}
	if a.IsCharacter() {
		ref.CharId = a.Id()
	}
	return ref
}

// Describe returns a detailed description of the actor, including equipped items.
func (r *ActorRef) Describe() string {
	currentHP, maxHP := r.actor.Resource(assets.ResourceHp)
	return describeActor(r.Description, r.Name, currentHP, maxHP, r.actor.Equipment())
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
	if r.instance.Object.Get().HasFlag(assets.ObjectFlagContainer) {
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
	Direction string              // Direction key (e.g., "north", "south")
	exit      *game.ResolvedExit // The resolved exit instance
}

func exitRefFrom(direction string, exit *game.ResolvedExit) *ExitRef {
	return &ExitRef{
		Direction: direction,
		exit:      exit,
	}
}

// TargetRef is a polymorphic target reference that could be a player, mobile, object, or exit.
type TargetRef struct {
	Type  targetType // targetTypeActor, targetTypeObject, or targetTypeExit
	Actor *ActorRef  // Non-nil if Type == targetTypeActor
	Obj   *ObjectRef // Non-nil if Type == targetTypeObject
	Exit  *ExitRef   // Non-nil if Type == targetTypeExit
}

// Name returns the display name of the target regardless of type.
func (r *TargetRef) Name() string {
	switch {
	case r.Actor != nil:
		return r.Actor.Name
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
func FindTarget(name string, tt targetType, spaces []SearchSpace) (*TargetRef, error) {
	for _, sp := range spaces {
		if tt&targetTypePlayer != 0 {
			if ci := sp.Finder.FindPlayer(name); ci != nil {
				return &TargetRef{
					Type:  targetTypeActor,
					Actor: actorRefFromPlayer(ci),
				}, nil
			}
		}
		if tt&targetTypeMobile != 0 {
			if mi := sp.Finder.FindMob(name); mi != nil {
				return &TargetRef{
					Type:  targetTypeActor,
					Actor: actorRefFromMob(mi),
				}, nil
			}
		}
		if tt&targetTypeObject != 0 {
			if oi := sp.Finder.FindObj(name); oi != nil {
				return &TargetRef{
					Type: targetTypeObject,
					Obj:  objRefFromInstance(oi, sp.Remover),
				}, nil
			}
		}
		if tt&targetTypeExit != 0 {
			if dir, exit := sp.Finder.FindExit(name); exit != nil {
				return &TargetRef{
					Type: targetTypeExit,
					Exit: exitRefFrom(dir, exit),
				}, nil
			}
		}
	}
	return nil, NewUserError(fmt.Sprintf("%s %q not found.", tt.Label(), name))
}

// --- TargetScopes ---

// TargetScopes maps scope flags to search spaces for a given actor.
// Implementations decide where to look (room, zone, world, inventory, etc.)
// without coupling the resolver to any particular game state type.
type TargetScopes interface {
	SpacesFor(s scope, actor shared.Actor) ([]SearchSpace, error)
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
func (r *TargetResolver) ResolveSpecs(specs []assets.TargetSpec, inputs map[string]any, actor shared.Actor) (map[string]*TargetRef, error) {
	if len(specs) == 0 {
		return make(map[string]*TargetRef), nil
	}

	targets := make(map[string]*TargetRef, len(specs))

	for _, spec := range specs {
		// Get the input value; inputs were already parsed and validated.
		name, _ := inputs[spec.Input].(string)
		if name == "" {
			switch spec.Default {
			case assets.DefaultCombatTarget:
				name = actor.CombatTargetId()
			case assets.DefaultSelf:
				targets[spec.Name] = &TargetRef{
					Type:  targetTypeActor,
					Actor: actorRefFromActor(actor),
				}
				continue
			}
		}
		if name == "" {
			if spec.Optional {
				targets[spec.Name] = nil
				continue
			}
			return nil, NewUserError(fmt.Sprintf("Input %q is required.", spec.Input))
		}

		// Determine search spaces: container scope or normal scope.
		var spaces []SearchSpace
		if cs, handled, err := containerSpaces(spec, targets); err != nil {
			return nil, err
		} else if handled {
			spaces = cs
		} else {
			s := parseScope(spec.Scopes)
			spaces, err = r.scopes.SpacesFor(s, actor)
			if err != nil {
				return nil, err
			}
		}

		ref, err := findWithNotFound(name, spec, spaces, inputs)
		if err != nil {
			if spec.AllowUnresolved {
				targets[spec.Name] = nil
				continue
			}
			return nil, err
		}
		targets[spec.Name] = ref
	}

	return targets, nil
}

// findWithNotFound wraps FindTarget and replaces the default error with the
// spec's NotFound template when one is configured.
func findWithNotFound(name string, spec assets.TargetSpec, spaces []SearchSpace, inputs map[string]any) (*TargetRef, error) {
	ref, err := FindTarget(name, parseTargetType(spec.Types), spaces)
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
func containerSpaces(spec assets.TargetSpec, targets map[string]*TargetRef) ([]SearchSpace, bool, error) {
	if spec.ScopeTarget == "" {
		return nil, false, nil
	}

	scopeRef := targets[spec.ScopeTarget]
	if scopeRef == nil || scopeRef.Obj == nil || scopeRef.Obj.instance == nil {
		// Scope target not resolved (likely optional) — fall through to normal scopes
		return nil, false, nil
	}

	// Validate it's a container
	if !scopeRef.Obj.instance.Object.Get().HasFlag(assets.ObjectFlagContainer) {
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
