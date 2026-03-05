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

	InstanceId string
	Mobile     storage.SmartIdentifier[*assets.Mobile]
	inCombat   bool

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
