package commands

import (
	"context"

	"github.com/pixil98/go-mud/internal/game"
)

// LookActor provides the character state needed by the look handler.
type LookActor interface {
	CommandActor
	Location() (zoneId, roomId string)
}

var _ LookActor = (*game.CharacterInstance)(nil)

// LookHandlerFactory creates handlers that display the current room.
type LookHandlerFactory struct {
	zones ZoneLocator
	pub   game.Publisher
}

// NewLookHandlerFactory creates a new LookHandlerFactory with access to world state.
func NewLookHandlerFactory(zones ZoneLocator, pub game.Publisher) *LookHandlerFactory {
	return &LookHandlerFactory{zones: zones, pub: pub}
}

func (f *LookHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer | targetTypeMobile | targetTypeObject, Required: false},
		},
	}
}

func (f *LookHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

func (f *LookHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[LookActor](f.handle), nil
}

func (f *LookHandlerFactory) handle(ctx context.Context, char LookActor, in *CommandInput) error {
	// Check if target was resolved (from targets section)
	if target := in.Targets["target"]; target != nil {
		return f.showTarget(char, target)
	}

	return f.showRoom(char)
}

// showRoom displays the current room description.
func (f *LookHandlerFactory) showRoom(char LookActor) error {
	zoneId, roomId := char.Location()

	ri := f.zones.GetZone(zoneId).GetRoom(roomId)
	if ri == nil {
		return NewUserError("You are in an invalid location.")
	}

	roomDesc := ri.Describe(char.Name())
	if f.pub != nil {
		return f.pub.Publish(game.SinglePlayer(char.Id()), nil, []byte(roomDesc))
	}

	return nil
}

// showTarget displays information about a specific target.
func (f *LookHandlerFactory) showTarget(char LookActor, target *TargetRef) error {
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
		return f.pub.Publish(game.SinglePlayer(char.Id()), nil, []byte(msg))
	}
	return nil
}
