package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// WearActor provides the character state needed by the wear handler.
type WearActor interface {
	CommandActor
	Location() (zoneId, roomId string)
	GetInventory() *game.Inventory
	GetEquipment() *game.Equipment
	Asset() *assets.Character
}

var _ WearActor = (*game.CharacterInstance)(nil)

// WearHandlerFactory creates handlers for equipping wearable items.
// Targets:
//   - target (required): the object to wear
type WearHandlerFactory struct {
	zones ZoneLocator
	pub   game.Publisher
}

func NewWearHandlerFactory(zones ZoneLocator, pub game.Publisher) *WearHandlerFactory {
	return &WearHandlerFactory{zones: zones, pub: pub}
}

func (f *WearHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeObject, Required: true},
		},
	}
}

func (f *WearHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *WearHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[WearActor](f.handle), nil
}

func (f *WearHandlerFactory) handle(ctx context.Context, char WearActor, in *CommandInput) error {
	target := in.Targets["target"]
	if target == nil || target.Obj == nil {
		return NewUserError("Wear what?")
	}

	// Use resolved object definition
	obj := target.Obj.instance.Object.Get()

	// Check if the item is wearable
	if !obj.HasFlag(assets.ObjectFlagWearable) {
		return NewUserError(fmt.Sprintf("You can't wear %s.", obj.ShortDesc))
	}

	actor := char.Asset()
	race := actor.Race.Get()

	// Find first wear slot that the race supports and has capacity
	var slot string
	for _, s := range obj.WearSlots {
		maxSlots := race.SlotCount(s)
		if maxSlots == 0 {
			continue // Race doesn't have this slot type
		}
		if char.GetEquipment().SlotCount(s) < maxSlots {
			slot = s
			break
		}
	}
	if slot == "" {
		// Distinguish "race can't wear this" from "slots full"
		hasSlot := false
		for _, s := range obj.WearSlots {
			if race.SlotCount(s) > 0 {
				hasSlot = true
				break
			}
		}
		if !hasSlot {
			return NewUserError(fmt.Sprintf("Your body can't wear %s.", obj.ShortDesc))
		}
		return NewUserError("You're already wearing something in that slot.")
	}

	// Remove from source and equip
	oi := target.Obj.source.RemoveObj(target.Obj.InstanceId)
	if oi == nil {
		return NewUserError(fmt.Sprintf("You're not carrying %s.", target.Obj.Name))
	}

	maxSlots := race.SlotCount(slot)
	err := char.GetEquipment().Equip(slot, maxSlots, oi)
	if err != nil {
		// Put it back on failure
		char.GetInventory().AddObj(oi)
		return NewUserError("You're already wearing something in that slot.")
	}

	// Send self message
	selfMsg := fmt.Sprintf("You wear %s.", obj.ShortDesc)
	if err := f.pub.Publish(game.SinglePlayer(char.Id()), nil, []byte(selfMsg)); err != nil {
		return err
	}

	// Broadcast to room
	roomMsg := fmt.Sprintf("%s wears %s.", actor.Name, obj.ShortDesc)
	zoneId, roomId := char.Location()
	room := f.zones.GetZone(zoneId).GetRoom(roomId)
	return f.pub.Publish(room, []string{char.Id()}, []byte(roomMsg))
}
