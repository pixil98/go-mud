package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// PutHandlerFactory creates handlers for putting objects into containers.
// Targets:
//   - target (required): the object to put (from inventory)
//   - container (required): the container to put it in (in room)
type PutHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

func NewPutHandlerFactory(world *game.WorldState, pub Publisher) *PutHandlerFactory {
	return &PutHandlerFactory{world: world, pub: pub}
}

func (f *PutHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypeObject, Required: true},
			{Name: "container", Type: TargetTypeObject, Required: true},
		},
	}
}

func (f *PutHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *PutHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		target := cmdCtx.Targets["target"]
		if target == nil || target.Obj == nil {
			return NewUserError("Put what?")
		}

		container := cmdCtx.Targets["container"]
		if container == nil || container.Obj == nil {
			return NewUserError("Put it in what?")
		}

		// Check if the target is actually a container
		containerObj := f.world.Objects().Get(string(container.Obj.ObjectId))
		if containerObj == nil || !containerObj.HasFlag(game.ObjectFlagContainer) {
			return NewUserError(fmt.Sprintf("%s is not a container.", container.Name))
		}

		// Remove from source (inventory)
		oi := target.Obj.Source.Remove(target.Obj.InstanceId)
		if oi == nil {
			return NewUserError(fmt.Sprintf("You're not carrying %s.", target.Name))
		}

		// Initialize container contents if needed
		if container.Obj.Instance.Contents == nil {
			container.Obj.Instance.Contents = game.NewInventory()
		}
		container.Obj.Instance.Contents.Add(oi)

		// Broadcast to room
		if f.pub != nil {
			obj := f.world.Objects().Get(string(target.Obj.ObjectId))
			msg := fmt.Sprintf("%s puts %s in %s.", cmdCtx.Actor.Name, obj.ShortDesc, containerObj.ShortDesc)
			return f.pub.PublishToRoom(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId, []byte(msg))
		}

		return nil
	}, nil
}