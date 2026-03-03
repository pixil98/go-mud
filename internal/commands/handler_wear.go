package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// WearHandlerFactory creates handlers for equipping wearable items.
// Targets:
//   - target (required): the object to wear
type WearHandlerFactory struct {
	rooms RoomLocator
	pub   game.Publisher
}

func NewWearHandlerFactory(rooms RoomLocator, pub game.Publisher) *WearHandlerFactory {
	return &WearHandlerFactory{rooms: rooms, pub: pub}
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
	return func(ctx context.Context, in *CommandInput) error {
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

		actor := in.Char.Character.Get()
		race := actor.Race.Get()

		// Find first wear slot that the race supports and has capacity
		var slot string
		for _, s := range obj.WearSlots {
			maxSlots := race.SlotCount(s)
			if maxSlots == 0 {
				continue // Race doesn't have this slot type
			}
			if in.Char.Equipment.SlotCount(s) < maxSlots {
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
		err := in.Char.Equipment.Equip(slot, maxSlots, oi)
		if err != nil {
			// Put it back on failure
			in.Char.Inventory.AddObj(oi)
			return NewUserError("You're already wearing something in that slot.")
		}

		// Send self message
		selfMsg := fmt.Sprintf("You wear %s.", obj.ShortDesc)
		if err := f.pub.Publish(game.SinglePlayer(in.Char.Character.Id()), nil, []byte(selfMsg)); err != nil {
			return err
		}

		// Broadcast to room
		roomMsg := fmt.Sprintf("%s wears %s.", actor.Name, obj.ShortDesc)
		zoneId, roomId := in.Char.Location()
		room := f.rooms.GetRoom(zoneId, roomId)
		return f.pub.Publish(room, []string{in.Char.Character.Id()}, []byte(roomMsg))
	}, nil
}
