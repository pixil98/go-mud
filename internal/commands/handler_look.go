package commands

import (
	"context"

	"github.com/pixil98/go-mud/internal/game"
)

// LookHandlerFactory creates handlers that display the current room.
type LookHandlerFactory struct {
	rooms RoomLocator
	pub   game.Publisher
}

// NewLookHandlerFactory creates a new LookHandlerFactory with access to world state.
func NewLookHandlerFactory(rooms RoomLocator, pub game.Publisher) *LookHandlerFactory {
	return &LookHandlerFactory{rooms: rooms, pub: pub}
}

func (f *LookHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer | targetTypeMobile | targetTypeObject, Required: false},
		},
	}
}

func (f *LookHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *LookHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, in *CommandInput) error {
		// Check if target was resolved (from targets section)
		if target := in.Targets["target"]; target != nil {
			return f.showTarget(in, target)
		}

		return f.showRoom(in)
	}, nil
}

// showRoom displays the current room description.
func (f *LookHandlerFactory) showRoom(in *CommandInput) error {
	zoneId, roomId := in.Char.Location()

	ri := f.rooms.GetRoom(zoneId, roomId)
	if ri == nil {
		return NewUserError("You are in an invalid location.")
	}

	roomDesc := ri.Describe(in.Char.Character.Get().Name)
	if f.pub != nil {
		return f.pub.Publish(game.SinglePlayer(in.Char.Character.Id()), nil, []byte(roomDesc))
	}

	return nil
}

// showTarget displays information about a specific target.
func (f *LookHandlerFactory) showTarget(in *CommandInput, target *TargetRef) error {
	var msg string
	switch target.Type {
	case targetTypePlayer:
		msg = target.Player.Describe()
	case targetTypeMobile:
		msg = target.Mob.Describe()
	case targetTypeObject:
		msg = target.Obj.Describe()
	default:
		return NewUserError("You can't look at that.")
	}

	if f.pub != nil {
		return f.pub.Publish(game.SinglePlayer(in.Char.Character.Id()), nil, []byte(msg))
	}
	return nil
}
