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
	world *game.WorldState
	pub   Publisher
}

func NewWearHandlerFactory(world *game.WorldState, pub Publisher) *WearHandlerFactory {
	return &WearHandlerFactory{world: world, pub: pub}
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

		// Look up object definition
		obj := f.world.Objects().Get(string(target.Obj.ObjectId))
		if obj == nil {
			return NewUserError("Wear what?")
		}

		// Check if the item is wearable
		if !obj.HasFlag(game.ObjectFlagWearable) {
			return NewUserError(fmt.Sprintf("You can't wear %s.", obj.ShortDesc))
		}

		// Look up the character's race for slot validation
		var race *game.Race
		if cmdCtx.Actor.Race != "" {
			race = f.world.Races().Get(string(cmdCtx.Actor.Race))
		}

		// Initialize equipment if needed
		if cmdCtx.Actor.Equipment == nil {
			cmdCtx.Actor.Equipment = game.NewEquipment()
		}

		// Find first wear slot that the race supports and has capacity
		var slot string
		for _, s := range obj.WearSlots {
			maxSlots := 0
			if race != nil {
				maxSlots = race.SlotCount(s)
				if maxSlots == 0 {
					continue // Race doesn't have this slot type
				}
			}
			if cmdCtx.Actor.Equipment.SlotCount(s) < maxSlots || maxSlots == 0 {
				slot = s
				break
			}
		}
		if slot == "" {
			// Distinguish "race can't wear this" from "slots full"
			if race != nil {
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
			}
			return NewUserError("You're already wearing something in that slot.")
		}

		// Remove from source and equip
		oi := target.Obj.source.RemoveObj(target.Obj.InstanceId)
		if oi == nil {
			return NewUserError(fmt.Sprintf("You're not carrying %s.", target.Obj.Name))
		}

		maxSlots := 0
		if race != nil {
			maxSlots = race.SlotCount(slot)
		}
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
