package game

import (
	"fmt"
	"strings"
	"sync"

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

// FollowTarget is the interface stored in the follow tree. Both
// CharacterInstance and MobileInstance satisfy it.
type FollowTarget interface {
	Id() string
	Name() string
	Notify(msg string)
	IsInCombat() bool
	Location() (string, string)
	Move(from, to *RoomInstance)
	Resource(name string) (current, maximum int)
	IsCharacter() bool
	Following() FollowTarget
	SetFollowing(FollowTarget)
	Followers() []FollowTarget
	AddFollower(FollowTarget)
	RemoveFollower(id string)
	SetFollowerGrouped(id string, grouped bool)
	IsFollowerGrouped(id string) bool
	GroupedFollowers() []FollowTarget
}

// followerEntry pairs a follow-tree pointer with a group membership flag.
type followerEntry struct {
	target  FollowTarget
	grouped bool
}

// ActorInstance holds resource pools, inventory, equipment, and perks shared
// between CharacterInstance and MobileInstance.
type ActorInstance struct {
	mu   sync.RWMutex
	self FollowTarget // set by owning type during construction

	InstanceId string
	inventory  *Inventory
	equipment  *Equipment
	resources  map[string]int // current values only; max derived from PerkCache
	level      int

	zoneId   string
	roomId   string
	inCombat bool

	following FollowTarget
	followers map[string]*followerEntry

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

// WearSlots returns the ordered list of equipment slot types granted to this
// actor via PerkGrantWearSlot perks. Duplicate entries represent multiple
// slots of the same type (e.g. two "finger" entries = two ring slots).
func (a *ActorInstance) WearSlots() []string {
	return a.GrantArgs(assets.PerkGrantWearSlot)
}

// countSlot returns how many times slot appears in the list.
func countSlot(slots []string, slot string) int {
	count := 0
	for _, s := range slots {
		if s == slot {
			count++
		}
	}
	return count
}

// Id returns the actor's unique identifier.
func (a *ActorInstance) Id() string { return a.InstanceId }

// Level returns the actor's current level.
func (a *ActorInstance) Level() int { return a.level }

// Location returns the actor's current zone and room.
func (a *ActorInstance) Location() (zoneId, roomId string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.zoneId, a.roomId
}

// IsInCombat returns whether the actor is currently in combat.
func (a *ActorInstance) IsInCombat() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.inCombat
}

// SetInCombat sets the actor's combat state.
func (a *ActorInstance) SetInCombat(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.inCombat = v
}

// Resource returns the current and max for a named resource.
func (a *ActorInstance) Resource(name string) (current, maximum int) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.resource(name)
}

// SetResource sets the current value for a named resource, clamped to [0, max].
func (a *ActorInstance) SetResource(name string, current int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	mx := a.resourceMax(name)
	a.setResourceCurrent(name, max(0, min(current, mx)))
}

// AdjustResource changes a resource's current value by delta, clamping to [0, max].
// When overfill is true the max clamp is skipped, allowing values above maximum.
func (a *ActorInstance) AdjustResource(name string, delta int, overfill bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.adjustResource(name, delta, overfill)
}

// Inventory returns the actor's inventory.
func (a *ActorInstance) Inventory() *Inventory {
	return a.inventory
}

// Equipment returns the actor's equipment.
func (a *ActorInstance) Equipment() *Equipment {
	return a.equipment
}

// IsAlive returns whether the actor has more than zero hit points.
func (a *ActorInstance) IsAlive() bool {
	cur, _ := a.Resource(assets.ResourceHp)
	return cur > 0
}

// ForEachResource calls fn for each resource while holding the lock.
func (a *ActorInstance) ForEachResource(fn func(name string, current, maximum int)) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for name, cur := range a.resources {
		fn(name, cur, a.resourceMax(name))
	}
}

// --- Follow tree ---

// Following returns the actor this actor is following, or nil.
func (a *ActorInstance) Following() FollowTarget {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.following
}

// SetFollowing sets who this actor follows. Manages reverse links automatically:
// removes self from the old leader's followers and adds self to the new leader's.
func (a *ActorInstance) SetFollowing(target FollowTarget) {
	a.mu.Lock()
	old := a.following
	a.following = target
	a.mu.Unlock()

	if old != nil {
		old.RemoveFollower(a.InstanceId)
	}
	if target != nil {
		target.AddFollower(a.self)
	}
}

// Followers returns a snapshot of all actors following this actor.
func (a *ActorInstance) Followers() []FollowTarget {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]FollowTarget, 0, len(a.followers))
	for _, entry := range a.followers {
		out = append(out, entry.target)
	}
	return out
}

// AddFollower adds an actor to this actor's follower list.
func (a *ActorInstance) AddFollower(ft FollowTarget) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.followers == nil {
		a.followers = make(map[string]*followerEntry)
	}
	a.followers[ft.Id()] = &followerEntry{target: ft}
}

// RemoveFollower removes an actor from this actor's follower list.
func (a *ActorInstance) RemoveFollower(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.followers, id)
}

// SetFollowerGrouped sets the grouped flag for a follower.
func (a *ActorInstance) SetFollowerGrouped(id string, grouped bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if entry, ok := a.followers[id]; ok {
		entry.grouped = grouped
	}
}

// IsFollowerGrouped reports whether a follower has the grouped flag set.
func (a *ActorInstance) IsFollowerGrouped(id string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if entry, ok := a.followers[id]; ok {
		return entry.grouped
	}
	return false
}

// GroupedFollowers returns a snapshot of all followers with the grouped flag set.
func (a *ActorInstance) GroupedFollowers() []FollowTarget {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []FollowTarget
	for _, entry := range a.followers {
		if entry.grouped {
			out = append(out, entry.target)
		}
	}
	return out
}

// ResourceLine returns a formatted display line for a resource (e.g. "HP: 45/50").
func ResourceLine(name string, current, maximum int) string {
	return fmt.Sprintf("%s: %d/%d", name, current, maximum)
}
