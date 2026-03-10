package game

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// ObjectInstance represents a single spawned instance of an Object definition.
// Location is tracked by the containing structure (room map or inventory).
type ObjectInstance struct {
	InstanceId string                                  `json:"-"` // Unique ID
	Object     storage.SmartIdentifier[*assets.Object] `json:"object_id"`
	Contents   *Inventory                              `json:"contents,omitempty"` // Non-nil for containers; holds objects stored inside
	Closed     bool                                    `json:"closed,omitempty"`   // Runtime open/closed state for containers with a Closure
	Locked     bool                                    `json:"locked,omitempty"`   // Runtime lock state for containers with a Lock
}

// UnmarshalJSON deserializes an ObjectInstance and assigns it a new unique InstanceId.
func (oi *ObjectInstance) UnmarshalJSON(b []byte) error {
	type Alias ObjectInstance
	err := json.Unmarshal(b, (*Alias)(oi))
	if err != nil {
		return err
	}
	oi.InstanceId = uuid.New().String()
	return nil
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
