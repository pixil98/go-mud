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

// HP returns the current and max hit points.
func (mi *MobileInstance) HP() (current, max int) {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	return mi.currentHP, mi.maxHP
}

// SetHP sets the current and max hit points.
func (mi *MobileInstance) SetHP(current, max int) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.currentHP = current
	mi.maxHP = max
}

// AdjustHP changes current HP by delta (positive = heal, negative = damage),
// clamping the result to [0, maxHP].
func (mi *MobileInstance) AdjustHP(delta int) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.ActorInstance.adjustHP(delta)
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
