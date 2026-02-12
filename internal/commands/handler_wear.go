package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// WearHandlerFactory creates handlers for equipping wearable items.
// TODO: This handler requires the target to be scoped to "inventory" in the
// command JSON. If misconfigured (e.g., scoped to "room"), the resolver will
// find the item but the handler will fail because it looks in inventory.
// HandlerSpec should be able to express required scopes so this can be
// validated at compile time rather than failing silently at runtime.
// Targets:
//   - target (required): the object to wear (from inventory)
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

		// Find the instance in inventory to get the ObjectId
		if cmdCtx.Actor.Inventory == nil {
			return NewUserError(fmt.Sprintf("You're not carrying %s.", target.Name))
		}
		oi := cmdCtx.Actor.Inventory.Get(target.Obj.InstanceId)
		if oi == nil {
			return NewUserError(fmt.Sprintf("You're not carrying %s.", target.Name))
		}

		// Look up object definition
		obj := f.world.Objects().Get(string(oi.ObjectId))
		if obj == nil {
			return NewUserError("Wear what?")
		}

		// Check if the item is wearable
		if !obj.HasFlag(game.ObjectFlagWearable) {
			return NewUserError(fmt.Sprintf("You can't wear %s.", obj.ShortDesc))
		}

		// Initialize equipment if needed
		if cmdCtx.Actor.Equipment == nil {
			cmdCtx.Actor.Equipment = game.NewEquipment()
		}

		// Find first available slot
		var slot string
		for _, s := range obj.WearSlots {
			if cmdCtx.Actor.Equipment.GetSlot(s) == nil {
				slot = s
				break
			}
		}
		if slot == "" {
			return NewUserError("You're already wearing something in that slot.")
		}

		// Remove from inventory and equip
		cmdCtx.Actor.Inventory.Remove(target.Obj.InstanceId)
		err := cmdCtx.Actor.Equipment.Equip(slot, oi)
		if err != nil {
			// Put it back on failure
			cmdCtx.Actor.Inventory.Add(oi)
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
