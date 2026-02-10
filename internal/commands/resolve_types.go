package commands

import (
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

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
type ObjectRef struct {
	InstanceId  string // Unique instance identifier
	Name        string
	Description string
}

// ObjectRefFrom creates an ObjectRef from a game.Object and its instance.
func ObjectRefFrom(obj *game.Object, instance *game.ObjectInstance) *ObjectRef {
	if obj == nil || instance == nil {
		return nil
	}
	return &ObjectRef{
		InstanceId:  instance.InstanceId,
		Name:        obj.ShortDesc,
		Description: obj.DetailedDesc,
	}
}

// TargetRef is a polymorphic target reference that could be a player, mobile, or object.
type TargetRef struct {
	Type   string     // "player", "mob", or "object"
	Player *PlayerRef // Non-nil if Type == "player"
	Mob    *MobileRef // Non-nil if Type == "mobile"
	Obj    *ObjectRef // Non-nil if Type == "object"
	Name   string     // Always set - the display name for templates
}
