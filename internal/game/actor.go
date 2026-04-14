package game

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pixil98/go-mud/internal/assets"
)

// Actor is the interface satisfied by both CharacterInstance and
// MobileInstance. It provides everything the ability, combat, and command
// systems need to interact with an entity without depending on concrete types.
type Actor interface {
	Id() string
	Name() string
	Room() *RoomInstance
	IsInCombat() bool
	IsAlive() bool
	Level() int
	Resource(name string) (current, max int)
	AdjustResource(name string, delta int, overfill bool)
	SpendAP(cost int) bool
	HasGrant(key, arg string) bool
	ModifierValue(key string) int
	GrantArgs(key string) []string
	AddTimedPerks(name string, perks []assets.Perk, ticks int)
	SetInCombat(bool)
	CombatTargetId() string
	SetCombatTargetId(id string)
	OnDeath() []*ObjectInstance
	IsCharacter() bool
	Inventory() *Inventory
	Equipment() *Equipment
	Notify(msg string)
	Following() Actor
	SetFollowing(Actor)
	Followers() []Actor
	AddFollower(Actor)
	RemoveFollower(id string)
	SetFollowerGrouped(id string, grouped bool)
	IsFollowerGrouped(id string) bool
	GroupedFollowers() []Actor
	Move(from, to *RoomInstance)
	ForEachResource(func(name string, current, maximum int))
	EnsureThreat(enemyId string, enemy Actor)
	AddThreatFrom(sourceId string, amount int)
	SetThreatFrom(sourceId string, amount int)
	TopThreatFrom(sourceId string)
	HasThreatFrom(enemyId string) bool
	ThreatSnapshot() map[string]int
	ClaimDeath() bool
	QueueTickMsg(msg string)
}

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

// Commander executes commands on behalf of an actor. Both mobs and players
// receive one; it wraps Handler.Exec bound to the specific actor instance.
type Commander interface {
	ExecCommand(ctx context.Context, cmd string, args ...string) error
	ExecAbility(ctx context.Context, abilityId string, target Actor) error
}

// CommanderFactory creates a per-actor Commander. Used during mob spawning
// and player login to bind the command handler to each actor.
type CommanderFactory func(Actor) Commander

// followerEntry pairs a follow-tree pointer with a group membership flag.
type followerEntry struct {
	target  Actor
	grouped bool
}

// ActorInstance holds resource pools, inventory, equipment, and perks shared
// between CharacterInstance and MobileInstance.
type ActorInstance struct {
	mu   sync.RWMutex
	self Actor // set by owning type during construction

	InstanceId string
	inventory  *Inventory
	equipment  *Equipment
	resources  map[string]int // current values only; max derived from PerkCache
	level      int

	room           *RoomInstance
	inCombat       bool
	deathProcessed atomic.Bool
	threatTable    ThreatTable
	cooldown       map[string][]int // auto_use arg → per-duplicate cooldown counters
	commander      Commander

	tickMsgBuf []string // per-tick message buffer, flushed at end of world tick

	following Actor
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

// Room returns the room the actor is currently in.
func (a *ActorInstance) Room() *RoomInstance {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.room
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
func (a *ActorInstance) Following() Actor {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.following
}

// SetFollowing sets who this actor follows. Manages reverse links automatically:
// removes self from the old leader's followers and adds self to the new leader's.
func (a *ActorInstance) SetFollowing(target Actor) {
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
func (a *ActorInstance) Followers() []Actor {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]Actor, 0, len(a.followers))
	for _, entry := range a.followers {
		out = append(out, entry.target)
	}
	return out
}

// AddFollower adds an actor to this actor's follower list.
func (a *ActorInstance) AddFollower(ft Actor) {
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
func (a *ActorInstance) GroupedFollowers() []Actor {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []Actor
	for _, entry := range a.followers {
		if entry.grouped {
			out = append(out, entry.target)
		}
	}
	return out
}

// --- Commander ---

// SetCommander sets the actor's command executor.
func (a *ActorInstance) SetCommander(c Commander) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.commander = c
}

// --- Threat table ---

// EnsureThreat idempotently adds an enemy with an initial threat of 1.
func (a *ActorInstance) EnsureThreat(enemyId string, enemy Actor) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.threatTable.ensureEntry(enemyId, enemy)
}

// AddThreatFrom increments the threat that sourceId has generated on this actor.
func (a *ActorInstance) AddThreatFrom(sourceId string, amount int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.threatTable.addThreat(sourceId, amount)
}

// SetThreatFrom sets the threat that sourceId has on this actor to an absolute value.
func (a *ActorInstance) SetThreatFrom(sourceId string, amount int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.threatTable.setThreat(sourceId, amount)
}

// TopThreatFrom sets sourceId's threat to one more than the current highest,
// guaranteeing sourceId becomes the top-threat enemy.
func (a *ActorInstance) TopThreatFrom(sourceId string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.threatTable.topThreat(sourceId)
}

// HasThreatFrom reports whether enemyId is on this actor's threat table.
func (a *ActorInstance) HasThreatFrom(enemyId string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.threatTable.hasEntry(enemyId)
}

// HasThreatEntries reports whether the threat table has any entries.
func (a *ActorInstance) HasThreatEntries() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.threatTable.hasEntries()
}

// RemoveThreatEntry removes an enemy from the threat table.
func (a *ActorInstance) RemoveThreatEntry(enemyId string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.threatTable.removeEntry(enemyId)
}

// ThreatSnapshot returns a copy of the threat table for safe iteration outside
// the lock (e.g. XP distribution after death).
func (a *ActorInstance) ThreatSnapshot() map[string]int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.threatTable.snapshot()
}

// ClearThreatTable removes all threat entries and cooldowns.
func (a *ActorInstance) ClearThreatTable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.threatTable.clear()
	clear(a.cooldown)
}

// ResolveCombatTarget returns the best target from the threat table.
// preferredId is checked first; falls back to highest threat.
// Returns nil if the table is empty.
func (a *ActorInstance) ResolveCombatTarget(preferredId string) Actor {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.threatTable.resolveTarget(preferredId)
}

// ThreatEnemies returns a snapshot of all enemy Actor references from the
// threat table. Safe to iterate outside the lock.
func (a *ActorInstance) ThreatEnemies() []Actor {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.threatTable.enemies()
}

// autoUseTick processes auto_use grants for one tick, executing each ready
// ability via the commander. Manages per-grant cooldown counters.
func (a *ActorInstance) autoUseTick(ctx context.Context, grants []string, target Actor) {
	if len(grants) == 0 {
		return
	}
	if a.commander == nil {
		slog.Error("autoUseTick called without commander", "actor", a.InstanceId)
		return
	}
	a.mu.Lock()
	if a.cooldown == nil {
		a.cooldown = make(map[string][]int)
	}

	var abilityIds []string
	seen := make(map[string]int)

	for _, arg := range grants {
		abilityId := arg
		cooldownTicks := 1
		if i := strings.IndexByte(arg, ':'); i >= 0 {
			abilityId = arg[:i]
			if n, err := strconv.Atoi(arg[i+1:]); err == nil && n > 0 {
				cooldownTicks = n
			}
		}

		dupIdx := seen[arg]
		seen[arg]++

		for len(a.cooldown[arg]) <= dupIdx {
			a.cooldown[arg] = append(a.cooldown[arg], 0)
		}

		remaining := a.cooldown[arg][dupIdx]
		if remaining > 0 {
			a.cooldown[arg][dupIdx] = remaining - 1
			continue
		}
		a.cooldown[arg][dupIdx] = cooldownTicks - 1
		abilityIds = append(abilityIds, abilityId)
	}
	a.mu.Unlock()

	for _, id := range abilityIds {
		_ = a.commander.ExecAbility(ctx, id, target)
	}
}

// --- Death ---

// ClaimDeath atomically marks the actor's death as processed. Returns true
// if this caller is the first to claim it; subsequent calls return false.
func (a *ActorInstance) ClaimDeath() bool {
	return a.deathProcessed.CompareAndSwap(false, true)
}

// --- Tick message buffer ---

// QueueTickMsg appends a message to the per-tick buffer. Messages are flushed
// as a single chunk at the end of the world tick.
func (a *ActorInstance) QueueTickMsg(msg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tickMsgBuf = append(a.tickMsgBuf, msg)
}

// ResourceLine returns a formatted display line for a resource (e.g. "HP: 45/50").
func ResourceLine(name string, current, maximum int) string {
	return fmt.Sprintf("%s: %d/%d", name, current, maximum)
}
