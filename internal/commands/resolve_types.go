package commands

import (
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// ObjectRemover can have objects removed from it.
// Equipment, Inventory, and RoomObjectHolder all satisfy this interface,
// allowing handlers to remove objects from the source scope without knowing
// which specific container type it is.
type ObjectRemover interface {
	Remove(instanceId string) *game.ObjectInstance
}

// ObjectHolder can have objects added and removed.
// Inventory and RoomObjectHolder satisfy this. Equipment does not (it uses Equip).
type ObjectHolder interface {
	ObjectRemover
	Add(obj *game.ObjectInstance)
}

// Stable template-facing types
// These types decouple templates from internal game structs.
// Templates access .Target.Name

// PlayerRef is the template-facing view of a resolved player.
type PlayerRef struct {
	CharId      storage.Identifier
	Name        string
	Description string
}

// PlayerRefFrom creates a PlayerRef from a game.Character.
func PlayerRefFrom(charId storage.Identifier, char *game.Character) *PlayerRef {
	if char == nil {
		return nil
	}
	return &PlayerRef{
		CharId:      charId,
		Name:        char.Name,
		Description: char.DetailedDesc,
	}
}

// MobileRef is the template-facing view of a resolved mob.
type MobileRef struct {
	InstanceId  string // Unique instance identifier
	Name        string
	Description string
}

// MobRefFrom creates a MobileRef from a game.Mobile and its instance.
func MobRefFrom(mob *game.Mobile, instance *game.MobileInstance) *MobileRef {
	if mob == nil || instance == nil {
		return nil
	}
	return &MobileRef{
		InstanceId:  instance.InstanceId,
		Name:        mob.ShortDesc,
		Description: mob.DetailedDesc,
	}
}

// ObjectRef is the template-facing view of a resolved object.
// TODO: Handlers mutate Instance fields (Inventory, Equipment, Contents) without
// locking. Consider adding a mutex to container types to make concurrent access safe.
type ObjectRef struct {
	InstanceId  string             // Unique instance identifier
	ObjectId    storage.Identifier // Reference to the Object definition
	Name        string
	Description string
	Source      ObjectRemover        // Container the object was resolved from
	Instance    *game.ObjectInstance // Direct reference to the instance
}

// ObjectRefFrom creates an ObjectRef from a game.Object and its instance.
func ObjectRefFrom(obj *game.Object, instance *game.ObjectInstance, source ObjectRemover) *ObjectRef {
	if obj == nil || instance == nil {
		return nil
	}
	return &ObjectRef{
		InstanceId:  instance.InstanceId,
		ObjectId:    instance.ObjectId,
		Name:        obj.ShortDesc,
		Description: obj.DetailedDesc,
		Source:      source,
		Instance:    instance,
	}
}

// TargetRef is a polymorphic target reference that could be a player, mobile, or object.
type TargetRef struct {
	Type   string     // "player", "mob", or "object"
	Player *PlayerRef // Non-nil if Type == "player"
	Mob    *MobileRef // Non-nil if Type == "mobile"
	Obj    *ObjectRef // Non-nil if Type == "object"
}
