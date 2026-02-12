package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// RemoveHandlerFactory creates handlers for unequipping worn items.
// TODO: This handler requires the target to be scoped to "equipment" in the
// command JSON. Same scope-coupling issue as WearHandlerFactory — see that
// handler's TODO for details.
// Targets:
//   - target (required): the object to remove (from equipment)
type RemoveHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

func NewRemoveHandlerFactory(world *game.WorldState, pub Publisher) *RemoveHandlerFactory {
	return &RemoveHandlerFactory{world: world, pub: pub}
}

func (f *RemoveHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypeObject, Required: true},
		},
	}
}

func (f *RemoveHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *RemoveHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		target := cmdCtx.Targets["target"]
		if target == nil || target.Obj == nil {
			return NewUserError("Remove what?")
		}

		// Find the item in equipment
		if cmdCtx.Actor.Equipment == nil {
			return NewUserError(fmt.Sprintf("You're not wearing %s.", target.Name))
		}
		slot, oi := cmdCtx.Actor.Equipment.FindByInstance(target.Obj.InstanceId)
		if oi == nil {
			return NewUserError(fmt.Sprintf("You're not wearing %s.", target.Name))
		}

		// Unequip it
		cmdCtx.Actor.Equipment.Unequip(slot)

		// Add back to inventory
		if cmdCtx.Actor.Inventory == nil {
			cmdCtx.Actor.Inventory = game.NewInventory()
		}
		cmdCtx.Actor.Inventory.Add(oi)

		// Broadcast to room
		if f.pub != nil {
			obj := f.world.Objects().Get(string(oi.ObjectId))
			msg := fmt.Sprintf("%s removes %s.", cmdCtx.Actor.Name, obj.ShortDesc)
			return f.pub.PublishToRoom(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId, []byte(msg))
		}

		return nil
	}, nil
}
