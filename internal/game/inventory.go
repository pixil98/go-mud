package game

// Inventory holds object instances carried by a character or mobile.
// TODO: Add stackable item support (keyed by ObjectId with count) for commodities.
type Inventory struct {
	// Items maps instance IDs to object instances
	Items map[string]*ObjectInstance `json:"items,omitempty"`
}

// NewInventory creates an empty inventory.
func NewInventory() *Inventory {
	return &Inventory{
		Items: make(map[string]*ObjectInstance),
	}
}

// Add adds an object instance to the inventory.
func (inv *Inventory) Add(obj *ObjectInstance) {
	inv.Items[obj.InstanceId] = obj
}

// Remove removes an object instance from the inventory.
// Returns the removed instance, or nil if not found.
func (inv *Inventory) Remove(instanceId string) *ObjectInstance {
	if obj, ok := inv.Items[instanceId]; ok {
		delete(inv.Items, instanceId)
		return obj
	}
	return nil
}

// Get returns an object instance by ID, or nil if not found.
func (inv *Inventory) Get(instanceId string) *ObjectInstance {
	return inv.Items[instanceId]
}

// Contains checks if an object instance is in the inventory.
func (inv *Inventory) Contains(instanceId string) bool {
	_, ok := inv.Items[instanceId]
	return ok
}
