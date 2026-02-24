package commands

import (
	"context"

	"github.com/pixil98/go-mud/internal/game"
)

// LookHandlerFactory creates handlers that display the current room.
type LookHandlerFactory struct {
	world *game.WorldState
	pub   game.Publisher
}

// NewLookHandlerFactory creates a new LookHandlerFactory with access to world state.
func NewLookHandlerFactory(world *game.WorldState, pub game.Publisher) *LookHandlerFactory {
	return &LookHandlerFactory{world: world, pub: pub}
}

func (f *LookHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypePlayer | TargetTypeMobile | TargetTypeObject, Required: false},
		},
	}
}

func (f *LookHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *LookHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		// Check if target was resolved (from targets section)
		if target := cmdCtx.Targets["target"]; target != nil {
			return f.showTarget(cmdCtx, target)
		}

		return f.showRoom(cmdCtx)
	}, nil
}

// showRoom displays the current room description.
func (f *LookHandlerFactory) showRoom(cmdCtx *CommandContext) error {
	zoneId, roomId := cmdCtx.Session.Location()

	ri := f.world.Instances()[zoneId].GetRoom(roomId)
	if ri == nil {
		return NewUserError("You are in an invalid location.")
	}

	roomDesc := ri.Describe(cmdCtx.Actor.Name)
	if f.pub != nil {
		return f.pub.Publish(game.SinglePlayer(cmdCtx.Session.CharId), nil, []byte(roomDesc))
	}

	return nil
}

// showTarget displays information about a specific target.
func (f *LookHandlerFactory) showTarget(cmdCtx *CommandContext, target *TargetRef) error {
	var msg string
	switch target.Type {
	case TargetTypePlayer:
		msg = target.Player.Describe()
	case TargetTypeMobile:
		msg = target.Mob.Describe()
	case TargetTypeObject:
		msg = target.Obj.Describe()
	default:
		return NewUserError("You can't look at that.")
	}

	if f.pub != nil {
		return f.pub.Publish(game.SinglePlayer(cmdCtx.Session.CharId), nil, []byte(msg))
	}
	return nil
}
