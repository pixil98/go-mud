package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
)

// --- Finder interfaces ---

// PlayerFinder finds players accepted by a matcher within a search scope.
type PlayerFinder interface {
	FindPlayers(func(*game.CharacterInstance) bool) []*game.CharacterInstance
}

// ObjectFinder finds objects accepted by a matcher within a search scope.
type ObjectFinder interface {
	FindObjs(func(*game.ObjectInstance) bool) []*game.ObjectInstance
}

// MobileFinder finds mobiles accepted by a matcher within a search scope.
type MobileFinder interface {
	FindMobs(func(*game.MobileInstance) bool) []*game.MobileInstance
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

// --- Matchers ---

// matchAll accepts any entity. Used for bare "all" targeting.
func matchAll[T any](_ T) bool { return true }

func mobNameMatcher(name string) func(*game.MobileInstance) bool {
	return func(mi *game.MobileInstance) bool {
		return mi.Mobile.Get().MatchName(name)
	}
}

func playerNameMatcher(name string) func(*game.CharacterInstance) bool {
	return func(ci *game.CharacterInstance) bool {
		return ci.Character.Get().MatchName(name)
	}
}

func objNameMatcher(name string) func(*game.ObjectInstance) bool {
	return func(oi *game.ObjectInstance) bool {
		return oi.Object.Get().MatchName(name)
	}
}

// --- Target prefix parsing ---

// parseTargetPrefix extracts N. or all. prefixes from a raw target string.
// Returns the clean name, the 1-based index (0 when all), and whether all
// matches are requested.
func parseTargetPrefix(raw string) (name string, index int, all bool) {
	if strings.EqualFold(raw, "all") {
		return "", 0, true
	}
	dot := strings.IndexByte(raw, '.')
	if dot < 1 {
		return raw, 1, false
	}
	prefix := raw[:dot]
	rest := raw[dot+1:]
	if strings.EqualFold(prefix, "all") {
		return rest, 0, true
	}
	if n, err := strconv.Atoi(prefix); err == nil && n > 0 {
		return rest, n, false
	}
	return raw, 1, false
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
	actor       game.Actor
	CharId      string // charId for players (used for message routing); empty for mobs
	Name        string
	Description string
}

// Actor returns the underlying game.Actor.
func (r *ActorRef) Actor() game.Actor { return r.actor }

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

func actorRefFromActor(a game.Actor) *ActorRef {
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
	return display.Capitalize(name)
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

// FindTarget searches spaces for matching targets. It parses N. and all.
// prefixes: "wolf" returns the first match, "2.wolf" the second, "all.wolf"
// all matches, and bare "all" every entity of the allowed types.
func FindTarget(raw string, tt targetType, spaces []SearchSpace) ([]*TargetRef, error) {
	name, index, all := parseTargetPrefix(raw)

	// Build matchers based on whether this is an "all" (no name) or name-based search.
	mobMatch := mobNameMatcher(name)
	playerMatch := playerNameMatcher(name)
	objMatch := objNameMatcher(name)
	if all && name == "" {
		mobMatch = matchAll[*game.MobileInstance]
		playerMatch = matchAll[*game.CharacterInstance]
		objMatch = matchAll[*game.ObjectInstance]
	}

	var refs []*TargetRef
	for _, sp := range spaces {
		if tt&targetTypePlayer != 0 {
			for _, ci := range sp.Finder.FindPlayers(playerMatch) {
				refs = append(refs, &TargetRef{
					Type:  targetTypeActor,
					Actor: actorRefFromPlayer(ci),
				})
			}
		}
		if tt&targetTypeMobile != 0 {
			for _, mi := range sp.Finder.FindMobs(mobMatch) {
				refs = append(refs, &TargetRef{
					Type:  targetTypeActor,
					Actor: actorRefFromMob(mi),
				})
			}
		}
		if tt&targetTypeObject != 0 {
			for _, oi := range sp.Finder.FindObjs(objMatch) {
				refs = append(refs, &TargetRef{
					Type: targetTypeObject,
					Obj:  objRefFromInstance(oi, sp.Remover),
				})
			}
		}
		if tt&targetTypeExit != 0 && !all {
			if dir, exit := sp.Finder.FindExit(name); exit != nil {
				refs = append(refs, &TargetRef{
					Type: targetTypeExit,
					Exit: exitRefFrom(dir, exit),
				})
			}
		}

		if !all && len(refs) >= index {
			break
		}
	}

	if len(refs) == 0 {
		return nil, NewUserError(fmt.Sprintf("%s %q not found.", tt.Label(), raw))
	}
	if all {
		return refs, nil
	}
	if index > len(refs) {
		return nil, NewUserError(fmt.Sprintf("%s %q not found.", tt.Label(), raw))
	}
	return refs[index-1 : index], nil
}

// --- TargetScopes ---

// TargetScopes maps scope flags to search spaces for a given actor.
// Implementations decide where to look (room, zone, world, inventory, etc.)
// without coupling the resolver to any particular game state type.
type TargetScopes interface {
	SpacesFor(s scope, actor game.Actor) ([]SearchSpace, error)
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
func (r *TargetResolver) ResolveSpecs(specs []assets.TargetSpec, inputs map[string]any, actor game.Actor) (map[string][]*TargetRef, error) {
	if len(specs) == 0 {
		return make(map[string][]*TargetRef), nil
	}

	targets := make(map[string][]*TargetRef, len(specs))

	for _, spec := range specs {
		// Get the input value; inputs were already parsed and validated.
		name, _ := inputs[spec.Input].(string)
		if name == "" {
			switch spec.Default {
			case assets.DefaultCombatTarget:
				name = actor.CombatTargetId()
			case assets.DefaultSelf:
				targets[spec.Name] = []*TargetRef{{
					Type:  targetTypeActor,
					Actor: actorRefFromActor(actor),
				}}
				continue
			case assets.DefaultRoomEnemies:
				targets[spec.Name] = resolveRoomEnemies(actor)
				continue
			case assets.DefaultGroupInRoom:
				targets[spec.Name] = resolveGroupInRoom(actor)
				continue
			}
		}
		if name == "" {
			if spec.Optional {
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

		_, _, isAll := parseTargetPrefix(name)
		if isAll && !spec.AllowAll {
			return nil, NewUserError("You can't do that to multiple things at once.")
		}

		refs, err := findWithNotFound(name, spec, spaces, inputs)
		if err != nil {
			if spec.AllowUnresolved {
				continue
			}
			return nil, err
		}
		targets[spec.Name] = refs
	}

	return targets, nil
}

// findWithNotFound wraps FindTarget and replaces the default error with the
// spec's NotFound template when one is configured.
func findWithNotFound(name string, spec assets.TargetSpec, spaces []SearchSpace, inputs map[string]any) ([]*TargetRef, error) {
	refs, err := FindTarget(name, parseTargetType(spec.Types), spaces)
	if err != nil && spec.NotFound != "" {
		msg, tmplErr := ExpandTemplate(spec.NotFound, &notFoundContext{Inputs: inputs})
		if tmplErr != nil {
			return nil, fmt.Errorf("expanding not_found template for target %q: %w", spec.Name, tmplErr)
		}
		return nil, NewUserError(msg)
	}
	return refs, err
}

// resolveRoomEnemies returns all actors in the room on the opposite side of the
// caster, excluding the caster. Side is determined by game.IsPlayerSide.
func resolveRoomEnemies(actor game.Actor) []*TargetRef {
	ri := actor.Room()
	if ri == nil {
		return nil
	}
	actorId := actor.Id()
	casterPlayerSide := game.IsPlayerSide(actor)

	var refs []*TargetRef
	ri.ForEachActor(func(a game.Actor) {
		if a.Id() == actorId {
			return
		}
		if game.IsPlayerSide(a) == casterPlayerSide {
			return
		}
		refs = append(refs, &TargetRef{
			Type:  targetTypeActor,
			Actor: actorRefFromActor(a),
		})
	})
	return refs
}

// resolveGroupInRoom returns all group members present in the actor's room,
// including the actor. A solo actor (not in a group) returns only themselves.
func resolveGroupInRoom(actor game.Actor) []*TargetRef {
	ri := actor.Room()
	if ri == nil {
		return nil
	}
	root := game.GroupLeader(actor)
	if root == nil {
		root = actor
	}

	var refs []*TargetRef
	game.WalkGroup(root, func(member game.Actor) {
		if member.Room() == ri {
			refs = append(refs, &TargetRef{
				Type:  targetTypeActor,
				Actor: actorRefFromActor(member),
			})
		}
	})
	return refs
}

// containerSpaces checks if a spec has a scope_target and returns container-only
// search spaces if the referenced target resolved to a container object.
// Returns (spaces, handled, error) where handled=true means container scoping applies.
func containerSpaces(spec assets.TargetSpec, targets map[string][]*TargetRef) ([]SearchSpace, bool, error) {
	if spec.ScopeTarget == "" {
		return nil, false, nil
	}

	scopeRefs := targets[spec.ScopeTarget]
	if len(scopeRefs) == 0 {
		return nil, false, nil
	}
	scopeRef := scopeRefs[0]
	if scopeRef.Obj == nil || scopeRef.Obj.instance == nil {
		// Scope target not resolved (likely optional) — fall through to normal scopes
		return nil, false, nil
	}

	// Validate it's a container
	if !scopeRef.Obj.instance.Object.Get().HasFlag(assets.ObjectFlagContainer) {
		return nil, false, NewUserError(fmt.Sprintf("%s is not a container.", display.Capitalize(scopeRef.Obj.Name)))
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
