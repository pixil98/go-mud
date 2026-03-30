package game

import "sync"

// Inventory holds object instances carried by a character or mobile.
// All methods are safe for concurrent use.
type Inventory struct {
	mu   sync.RWMutex
	objs map[string]*ObjectInstance
}

// NewInventory creates an empty inventory.
func NewInventory() *Inventory {
	return &Inventory{
		objs: make(map[string]*ObjectInstance),
	}
}

// AddObj adds an object instance to the inventory.
func (inv *Inventory) AddObj(obj *ObjectInstance) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if inv.objs == nil {
		inv.objs = make(map[string]*ObjectInstance)
	}
	inv.objs[obj.InstanceId] = obj
}

// RemoveObj removes an object instance from the inventory.
// Returns the removed instance, or nil if not found.
func (inv *Inventory) RemoveObj(instanceId string) *ObjectInstance {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if obj, ok := inv.objs[instanceId]; ok {
		delete(inv.objs, instanceId)
		return obj
	}
	return nil
}

// FindObj searches inventory items for one whose definition matches the given alias.
// Returns nil if not found.
func (inv *Inventory) FindObj(name string) *ObjectInstance {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	for _, oi := range inv.objs {
		if oi.Object.Get().MatchName(name) {
			return oi
		}
	}
	return nil
}

// FindObjByDef searches for an object whose definition ID matches defId.
// Returns nil if not found.
func (inv *Inventory) FindObjByDef(defId string) *ObjectInstance {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	for _, oi := range inv.objs {
		if oi.Object.Id() == defId {
			return oi
		}
	}
	return nil
}

// ForEachObj calls fn for each object in the inventory while holding the read lock.
func (inv *Inventory) ForEachObj(fn func(string, *ObjectInstance)) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	for id, oi := range inv.objs {
		fn(id, oi)
	}
}

// Len returns the number of items in the inventory.
func (inv *Inventory) Len() int {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return len(inv.objs)
}

// Clear removes all items.
func (inv *Inventory) Clear() {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	inv.objs = make(map[string]*ObjectInstance)
}

// Tick advances decay on all items. Each object's Tick is called, then any
// object whose RemainingTicks has reached zero is removed.
func (inv *Inventory) Tick() {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	for id, oi := range inv.objs {
		oi.Tick()
		if oi.Expired() {
			delete(inv.objs, id)
		}
	}
}

// Drain atomically removes and returns all items.
func (inv *Inventory) Drain() []*ObjectInstance {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	items := make([]*ObjectInstance, 0, len(inv.objs))
	for _, obj := range inv.objs {
		items = append(items, obj)
	}
	inv.objs = make(map[string]*ObjectInstance)
	return items
}
