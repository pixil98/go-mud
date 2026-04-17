package game

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// ObjectInstance represents a single spawned instance of an Object definition.
// Location is tracked by the containing structure (room map or inventory).
type ObjectInstance struct {
	InstanceId     string
	Object         storage.SmartIdentifier[*assets.Object]
	Contents       *Inventory // Non-nil for containers; holds objects stored inside
	Closed         bool       // Runtime open/closed state for containers with a Closure
	Locked         bool       // Runtime lock state for containers with a Lock
	RemainingTicks int        // Ticks until decay; 0 = not decaying
	decaying       bool       // True once ActivateDecay has been called
}

// NewObjectInstance creates an ObjectInstance linked to its definition.
// Containers are initialized with an empty Contents inventory.
func NewObjectInstance(obj storage.SmartIdentifier[*assets.Object]) (*ObjectInstance, error) {
	if obj.Get() == nil {
		return nil, fmt.Errorf("unable create %q from unresolved object", obj.Id())
	}

	def := obj.Get()
	oi := &ObjectInstance{
		InstanceId: uuid.New().String(),
		Object:     obj,
	}
	if def.HasFlag(assets.ObjectFlagContainer) {
		oi.Contents = NewInventory()
		if def.Closure != nil {
			oi.Closed = def.Closure.Closed
			if def.Closure.Lock != nil {
				oi.Locked = def.Closure.Lock.Locked
			}
		}
	}
	return oi, nil
}

// ActivateDecay starts the decay timer if this object has a finite Lifetime
// and hasn't already been activated. Call this when a player acquires the item.
func (oi *ObjectInstance) ActivateDecay() {
	if !oi.decaying && oi.Object.Get().Lifetime > 0 {
		oi.RemainingTicks = oi.Object.Get().Lifetime
		oi.decaying = true
	}
}

// Expired returns true if this object was decaying and has run out of time.
func (oi *ObjectInstance) Expired() bool {
	return oi.decaying && oi.RemainingTicks == 0
}

// Tick decrements the decay timer if active and recursively ticks container
// contents. The caller is responsible for removing this instance when
// RemainingTicks reaches zero.
func (oi *ObjectInstance) Tick() {
	if oi.Contents != nil {
		oi.Contents.Tick()
	}
	if oi.RemainingTicks > 0 {
		oi.RemainingTicks--
	}
}

// Resolve resolves this instance's object definition and recursively resolves
// any contents. Containers are initialized with an empty Contents inventory
// if they don't already have one.
func (oi *ObjectInstance) Resolve(objs storage.Storer[*assets.Object]) error {
	if err := oi.Object.Resolve(objs); err != nil {
		return err
	}
	if oi.Object.Get().HasFlag(assets.ObjectFlagContainer) && oi.Contents == nil {
		oi.Contents = NewInventory()
	}
	if oi.Contents != nil {
		for _, ci := range oi.Contents.objs {
			if err := ci.Resolve(objs); err != nil {
				return err
			}
		}
	}
	return nil
}

// SpawnObject creates an ObjectInstance from an assets.ObjectSpawn spec,
// recursively spawning any contents for containers.
func SpawnObject(spec assets.ObjectSpawn) (*ObjectInstance, error) {
	oi, err := NewObjectInstance(spec.Object)
	if err != nil {
		return nil, fmt.Errorf("spawning: %w", err)
	}

	for _, contentSpawn := range spec.Contents {
		soi, err := SpawnObject(contentSpawn)
		if err != nil {
			return nil, err
		}
		oi.Contents.AddObj(soi)
	}
	return oi, nil
}

// materializeInventoryEquipment batch-spawns a set of inventory and equipment
// specs into runtime containers. Shared by character and mobile construction.
func materializeInventoryEquipment(invSpawns []assets.ObjectSpawn, eqSpawns []assets.EquipmentSpawn) (*Inventory, *Equipment, error) {
	inv := NewInventory()
	for _, spawn := range invSpawns {
		oi, err := SpawnObject(spawn)
		if err != nil {
			return nil, nil, err
		}
		inv.AddObj(oi)
	}

	eq := NewEquipment()
	for _, es := range eqSpawns {
		oi, err := SpawnObject(es.ObjectSpawn)
		if err != nil {
			return nil, nil, err
		}
		eq.equip(es.Slot, oi)
	}

	return inv, eq, nil
}
