package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// GetHandlerFactory creates handlers for picking up objects.
// Targets:
//   - container (optional): the container to get from
//   - target (required): the object to pick up (scoped to container when present)
//
// TODO: I think this can become a generic moveObj handler as long as both ends are an Inventory. Maybe inventory should be an interface here?
type GetHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

func NewGetHandlerFactory(world *game.WorldState, pub Publisher) *GetHandlerFactory {
	return &GetHandlerFactory{world: world, pub: pub}
}

func (f *GetHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "container", Type: TargetTypeObject, Required: false},
			{Name: "target", Type: TargetTypeObject, Required: true},
		},
	}
}

func (f *GetHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *GetHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		target := cmdCtx.Targets["target"]
		if target == nil || target.Obj == nil {
			return NewUserError("Get what?")
		}

		// Remove from source (room or container, handled by Source)
		oi := target.Obj.Source.Remove(target.Obj.InstanceId)
		if oi == nil {
			return NewUserError(fmt.Sprintf("You don't see %s here.", target.Obj.Name))
		}

		if f.world.Objects().Get(string(target.Obj.ObjectId)).HasFlag(game.ObjectFlagImmobile) {
			return NewUserError(fmt.Sprintf("You can't seem to move %s.", target.Obj.Name))
		}

		// Add to inventory
		if cmdCtx.Actor.Inventory == nil {
			cmdCtx.Actor.Inventory = game.NewInventory()
		}
		cmdCtx.Actor.Inventory.Add(oi)

		// Broadcast to room
		if f.pub != nil {
			obj := f.world.Objects().Get(string(target.Obj.ObjectId))
			container := cmdCtx.Targets["container"]
			var msg string
			if container != nil {
				containerObj := f.world.Objects().Get(string(container.Obj.ObjectId))
				msg = fmt.Sprintf("%s gets %s from %s.", cmdCtx.Actor.Name, obj.ShortDesc, containerObj.ShortDesc)
			} else {
				msg = fmt.Sprintf("%s picks up %s.", cmdCtx.Actor.Name, obj.ShortDesc)
			}
			return f.pub.PublishToRoom(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId, []byte(msg))
		}

		return nil
	}, nil
}
