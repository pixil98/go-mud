package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// WearHandlerFactory creates handlers for equipping wearable items.
// Targets:
//   - target (required): the object to wear
type WearHandlerFactory struct {
	pub Publisher
}

func NewWearHandlerFactory(pub Publisher) *WearHandlerFactory {
	return &WearHandlerFactory{pub: pub}
}

func (f *WearHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypeObject, Required: true},
		},
	}
}

func (f *WearHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *WearHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		target := cmdCtx.Targets["target"]
		if target == nil || target.Obj == nil {
			return NewUserError("Wear what?")
		}

		// Use resolved object definition
		obj := target.Obj.instance.Object.Get()

		// Check if the item is wearable
		if !obj.HasFlag(game.ObjectFlagWearable) {
			return NewUserError(fmt.Sprintf("You can't wear %s.", obj.ShortDesc))
		}

		race := cmdCtx.Actor.Race.Get()

		// Initialize equipment if needed
		if cmdCtx.Actor.Equipment == nil {
			cmdCtx.Actor.Equipment = game.NewEquipment()
		}

		// Find first wear slot that the race supports and has capacity
		var slot string
		for _, s := range obj.WearSlots {
			maxSlots := race.SlotCount(s)
			if maxSlots == 0 {
				continue // Race doesn't have this slot type
			}
			if cmdCtx.Actor.Equipment.SlotCount(s) < maxSlots {
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
		err := cmdCtx.Actor.Equipment.Equip(slot, maxSlots, oi)
		if err != nil {
			// Put it back on failure
			cmdCtx.Actor.Inventory.AddObj(oi)
			return NewUserError("You're already wearing something in that slot.")
		}

		// Broadcast to room
		if f.pub != nil {
			msg := fmt.Sprintf("%s wears %s.", cmdCtx.Actor.Name, obj.ShortDesc)
			return f.pub.PublishToRoom(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId, []byte(msg))
		}

		return nil
	}, nil
}
