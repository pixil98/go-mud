package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// GetHandlerFactory creates handlers for picking up objects.
// Targets:
//   - target (required): the object to pick up
//   - container (optional): the container to get from (TODO: implement)
type GetHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

func NewGetHandlerFactory(world *game.WorldState, pub Publisher) *GetHandlerFactory {
	return &GetHandlerFactory{world: world, pub: pub}
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

		container := cmdCtx.Targets["container"]
		if container != nil {
			// TODO: implement getting from containers
			return NewUserError("Getting items from containers is not yet supported.")
		}

		// Remove from room
		oId := f.world.RemoveObjectFromRoom(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId, target.Obj.InstanceId)
		if oId == nil {
			return NewUserError(fmt.Sprintf("You don't see %s here.", target.Name))
		}

		// Add to inventory
		if cmdCtx.Actor.Inventory == nil {
			cmdCtx.Actor.Inventory = game.NewInventory()
		}
		cmdCtx.Actor.Inventory.Add(oId)

		// Broadcast to room
		if f.pub != nil {
			obj := f.world.Objects().Get(string(oId.ObjectId))
			msg := fmt.Sprintf("%s picks up %s.", cmdCtx.Actor.Name, obj.ShortDesc)
			roomChannel := fmt.Sprintf("zone-%s-room-%s", cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId)
			_ = f.pub.Publish(roomChannel, []byte(msg))
		}

		return nil
	}, nil
}
