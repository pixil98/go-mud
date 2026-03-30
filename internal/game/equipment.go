package game

import (
	"fmt"
	"sync"

	"github.com/pixil98/go-mud/internal/assets"
)

// EquipSlot pairs a slot type name with the equipped object instance.
type EquipSlot struct {
	Slot string          `json:"slot"`
	Obj  *ObjectInstance `json:"obj"`
}

// Equipment holds items equipped by a character or mobile.
// Multiple items may share the same slot type (e.g., two rings in "finger").
// All methods are safe for concurrent use.
type Equipment struct {
	mu   sync.RWMutex
	objs []EquipSlot
	PerkCache
}

// NewEquipment creates an empty equipment set.
func NewEquipment() *Equipment {
	return &Equipment{
		PerkCache: *NewPerkCache(nil, nil),
	}
}

// --- Equip / Unequip ---

// Equip adds an object to the given slot type. maxSlots limits how many items
// can occupy that slot type (0 means no limit). Returns an error if the slot
// is already at capacity.
func (eq *Equipment) Equip(slot string, maxSlots int, obj *ObjectInstance) error {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if maxSlots > 0 && eq.slotCount(slot) >= maxSlots {
		return fmt.Errorf("no available %q slot", slot)
	}
	eq.objs = append(eq.objs, EquipSlot{Slot: slot, Obj: obj})
	eq.rebuildPerks()
	return nil
}

// RemoveObj finds and unequips an object by instance ID.
func (eq *Equipment) RemoveObj(instanceId string) *ObjectInstance {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	for i, item := range eq.objs {
		if item.Obj.InstanceId == instanceId {
			eq.objs = append(eq.objs[:i], eq.objs[i+1:]...)
			eq.rebuildPerks()
			return item.Obj
		}
	}
	return nil
}

// Drain atomically removes and returns all equipped objects.
func (eq *Equipment) Drain() []*ObjectInstance {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	var items []*ObjectInstance
	for _, slot := range eq.objs {
		if slot.Obj != nil {
			items = append(items, slot.Obj)
		}
	}
	eq.objs = []EquipSlot{}
	eq.rebuildPerks()
	return items
}

// Tick advances the embedded PerkCache tick and decays equipped items.
// Expired items are removed and perks are rebuilt if needed.
func (eq *Equipment) Tick() {
	eq.PerkCache.Tick()

	eq.mu.Lock()
	defer eq.mu.Unlock()

	n := 0
	for _, slot := range eq.objs {
		slot.Obj.Tick()
		if slot.Obj.Expired() {
			continue
		}
		eq.objs[n] = slot
		n++
	}
	if n < len(eq.objs) {
		eq.objs = eq.objs[:n]
		eq.rebuildPerks()
	}
}

// --- Queries ---

// SlotCount returns how many items are equipped in the given slot type.
func (eq *Equipment) SlotCount(slot string) int {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	return eq.slotCount(slot)
}

// FindObj searches equipped items for one whose definition matches the given alias.
func (eq *Equipment) FindObj(name string) *ObjectInstance {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	for _, slot := range eq.objs {
		if slot.Obj == nil {
			continue
		}
		if slot.Obj.Object.Get().MatchName(name) {
			return slot.Obj
		}
	}
	return nil
}

// ForEachSlot calls fn for each equipment slot while holding the read lock.
func (eq *Equipment) ForEachSlot(fn func(EquipSlot)) {
	eq.mu.RLock()
	defer eq.mu.RUnlock()
	for _, slot := range eq.objs {
		fn(slot)
	}
}

// Len returns the number of equipped items.
func (eq *Equipment) Len() int {
	eq.mu.RLock()
	defer eq.mu.RUnlock()
	return len(eq.objs)
}

// --- Perks ---

// Snapshot returns the pre-resolved equipment perks and version atomically.
func (eq *Equipment) Snapshot() (*ResolvedPerks, uint64) {
	eq.mu.RLock()
	defer eq.mu.RUnlock()
	return eq.PerkCache.Snapshot()
}

// rebuildPerks aggregates perks from all equipped items into the embedded PerkCache.
// Caller must hold the write lock.
func (eq *Equipment) rebuildPerks() {
	var perks []assets.Perk
	for _, slot := range eq.objs {
		if slot.Obj != nil {
			perks = append(perks, slot.Obj.Object.Get().Perks...)
		}
	}
	eq.SetOwn(perks)
}

// slotCount returns how many items are equipped in the given slot type.
// Caller must hold at least a read lock.
func (eq *Equipment) slotCount(slot string) int {
	count := 0
	for _, item := range eq.objs {
		if item.Slot == slot {
			count++
		}
	}
	return count
}
