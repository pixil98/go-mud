package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// DropHandlerFactory creates handlers for dropping objects from inventory.
// Targets:
//   - target (required): the object to drop
type DropHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

func NewDropHandlerFactory(world *game.WorldState, pub Publisher) *DropHandlerFactory {
	return &DropHandlerFactory{world: world, pub: pub}
}

func (f *DropHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypeObject, Required: true},
		},
	}
}

func (f *DropHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *DropHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		target := cmdCtx.Targets["target"]
		if target == nil || target.Obj == nil {
			return NewUserError("Drop what?")
		}

		// Remove from inventory
		if cmdCtx.Actor.Inventory == nil {
			return NewUserError(fmt.Sprintf("You're not carrying %s.", target.Name))
		}

		oi := cmdCtx.Actor.Inventory.Remove(target.Obj.InstanceId)
		if oi == nil {
			return NewUserError(fmt.Sprintf("You're not carrying %s.", target.Name))
		}

		// Add to room
		f.world.AddObjectToRoom(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId, oi)

		// Broadcast to room
		if f.pub != nil {
			obj := f.world.Objects().Get(string(oi.ObjectId))
			msg := fmt.Sprintf("%s drops %s.", cmdCtx.Actor.Name, obj.ShortDesc)
			return f.pub.PublishToRoom(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId, []byte(msg))
		}

		return nil
	}, nil
}
