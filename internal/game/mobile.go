package game

import (
	"fmt"
	"sync"

	"github.com/pixil98/go-mud/internal/storage"

	"github.com/pixil98/go-mud/internal/assets"
)

// MobileInstance represents a single spawned instance of a Mobile definition.
// Location is tracked by the containing structure (room map).
type MobileInstance struct {
	mu sync.RWMutex

	InstanceId    string
	Mobile        storage.SmartIdentifier[*assets.Mobile]
	inCombat      bool
	threats       map[string]int // opaque combatant ID → threat; keys owned by combat package
	contributions map[string]int // opaque combatant ID → contribution total (damage + heals) for XP

	ActorInstance
}

// --- Accessor methods ---

// IsInCombat returns whether the mobile is currently in combat.
func (mi *MobileInstance) IsInCombat() bool {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	return mi.inCombat
}

// SetInCombat sets the mobile's combat state.
func (mi *MobileInstance) SetInCombat(v bool) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.inCombat = v
}

// Resource returns the current and max for a named resource.
func (mi *MobileInstance) Resource(name string) (current, max int) {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	return mi.resource(name)
}

// SetResource sets the current value for a named resource, clamped to [0, max].
func (mi *MobileInstance) SetResource(name string, current int) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mx := mi.resourceMax(name)
	mi.setResourceCurrent(name, max(0, min(current, mx)))
}

// AdjustResource changes a resource's current value by delta, clamping to [0, max].
func (mi *MobileInstance) AdjustResource(name string, delta int) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.adjustResource(name, delta)
}

// RegenTick regenerates all resources based on perk-driven regen values.
func (mi *MobileInstance) RegenTick() {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.regenTick()
}

// GetInventory returns the mobile's inventory.
// Inventory is self-locking; its methods are safe for concurrent use.
func (mi *MobileInstance) GetInventory() *Inventory {
	return mi.inventory
}

// GetEquipment returns the mobile's equipment.
// Equipment is self-locking; its methods are safe for concurrent use.
func (mi *MobileInstance) GetEquipment() *Equipment {
	return mi.equipment
}

// IsAlive returns whether the mobile has more than zero hit points.
func (mi *MobileInstance) IsAlive() bool {
	cur, _ := mi.Resource(assets.ResourceHp)
	return cur > 0
}

// Flags returns display labels for the mobile's current state.
func (mi *MobileInstance) Flags() []string {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	var flags []string
	if mi.inCombat {
		flags = append(flags, "fighting")
	}
	return flags
}

// AddThreat adds amount to this mob's threat toward id.
func (mi *MobileInstance) AddThreat(id string, amount int) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.threats[id] += amount
	if mi.threats[id] < 0 {
		mi.threats[id] = 0
	}
}

// ClearThreatFor removes a single entry from this mob's threat table.
func (mi *MobileInstance) ClearThreatFor(id string) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	delete(mi.threats, id)
}

// ClearAllThreats wipes the threat and contribution tables and ends combat.
func (mi *MobileInstance) ClearAllThreats() {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.threats = make(map[string]int)
	mi.contributions = make(map[string]int)
	mi.inCombat = false
}

// HasThreat reports whether id has an entry in this mob's threat table.
func (mi *MobileInstance) HasThreat(id string) bool {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	_, ok := mi.threats[id]
	return ok
}

// RecordContribution adds amount to id's contribution total (used for XP).
func (mi *MobileInstance) RecordContribution(id string, amount int) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.contributions[id] += amount
}

// SnapshotContributions returns a copy of the contribution map for XP calculation.
func (mi *MobileInstance) SnapshotContributions() map[string]int {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	out := make(map[string]int, len(mi.contributions))
	for k, v := range mi.contributions {
		out[k] = v
	}
	return out
}

// SnapshotThreats returns a copy of the threat map. The combat package uses this
// to perform CombatID-aware target selection without locking the mob.
func (mi *MobileInstance) SnapshotThreats() map[string]int {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	out := make(map[string]int, len(mi.threats))
	for k, v := range mi.threats {
		out[k] = v
	}
	return out
}

// StatSections returns the mobile's stat display sections.
func (mi *MobileInstance) StatSections() []StatSection {
	mob := mi.Mobile.Get()
	return []StatSection{
		{Lines: []StatLine{
			{Value: mob.ShortDesc, Center: true},
			{Value: fmt.Sprintf("Level %d", mob.Level), Center: true},
		}},
	}
}
