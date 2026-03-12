package game

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
)

// StatLine is a single line in a stat section.
type StatLine struct {
	Value  string
	Center bool
}

// StatSection is a labeled group of stat lines.
type StatSection struct {
	Header string
	Lines  []StatLine
}

// ActorInstance holds resource pools, inventory, equipment, and perks shared
// between CharacterInstance and MobileInstance.
type ActorInstance struct {
	InstanceId string
	inventory  *Inventory
	equipment  *Equipment
	resources  map[string]int // current values only; max derived from PerkCache
	level      int
	PerkCache
}

// resourceMax computes the max value for a named resource from perks.
// Formula: sum(core.resource.<name>.max) + level * sum(core.resource.<name>.per_level)
func (a *ActorInstance) resourceMax(name string) int {
	return a.ModifierValue(assets.BuildKey(assets.ResourcePrefix, name, assets.ResourceAspectMax)) +
		a.level*a.ModifierValue(assets.BuildKey(assets.ResourcePrefix, name, assets.ResourceAspectPerLevel))
}

// resource returns (current, max) for the named resource.
// Returns (0, 0) if the resource doesn't exist.
func (a *ActorInstance) resource(name string) (current, maximum int) {
	cur, ok := a.resources[name]
	if !ok {
		return 0, 0
	}
	return cur, a.resourceMax(name)
}

// setResourceCurrent sets the current value for a named resource.
func (a *ActorInstance) setResourceCurrent(name string, current int) {
	if a.resources == nil {
		a.resources = make(map[string]int)
	}
	a.resources[name] = current
}

// adjustResource changes a resource's current value by delta, clamping to [0, max].
// When overfill is true the max clamp is skipped, allowing values above maximum.
// No-op if the resource doesn't exist.
func (a *ActorInstance) adjustResource(name string, delta int, overfill bool) {
	cur, ok := a.resources[name]
	if !ok {
		return
	}
	v := cur + delta
	if !overfill {
		v = min(v, a.resourceMax(name))
	}
	a.resources[name] = max(0, v)
}

// initResources discovers all resource perk keys and initializes current = max
// for each resource. Call this after the PerkCache is wired and resolved.
func (a *ActorInstance) initResources() {
	if a.resources == nil {
		a.resources = make(map[string]int)
	}
	for name := range a.resourceNames() {
		a.resources[name] = a.resourceMax(name)
	}
}

// resourceNames returns the set of resource names discovered from perk modifier keys.
func (a *ActorInstance) resourceNames() map[string]struct{} {
	names := make(map[string]struct{})
	resourceKeyPrefix := assets.ResourcePrefix + "."
	for key := range a.Modifiers() {
		if !strings.HasPrefix(key, resourceKeyPrefix) {
			continue
		}
		rest := key[len(resourceKeyPrefix):]
		dotIdx := strings.Index(rest, ".")
		if dotIdx < 0 {
			continue
		}
		names[rest[:dotIdx]] = struct{}{}
	}
	return names
}

// regenTick applies flat regen from perks to all resources.
// Formula per resource: sum(core.resource.<name>.regen).
// Caller must hold the owning type's write lock.
func (a *ActorInstance) regenTick() {
	for name := range a.resources {
		regen := a.ModifierValue(assets.BuildKey(assets.ResourcePrefix, name, assets.ResourceAspectRegen))
		if regen > 0 {
			a.adjustResource(name, regen, false)
		}
	}
}

// Id returns the actor's unique identifier.
// TODO: abstract ActorInstance behind an interface so external packages don't
// need to construct structs directly for testing.
func (a *ActorInstance) Id() string { return a.InstanceId }

// Level returns the actor's current level.
func (a *ActorInstance) Level() int { return a.level }

// ForEachResource calls fn for each resource. Caller must hold the owning type's lock.
func (a *ActorInstance) ForEachResource(fn func(name string, current, maximum int)) {
	for name, cur := range a.resources {
		fn(name, cur, a.resourceMax(name))
	}
}

// ResourceLine returns a formatted display line for a resource (e.g. "HP: 45/50").
func ResourceLine(name string, current, maximum int) string {
	return fmt.Sprintf("%s: %d/%d", name, current, maximum)
}
