package commands

import (
	"context"

	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
)

// LookActor provides the character state needed by the look handler.
type LookActor interface {
	Id() string
	Name() string
	Location() (string, string)
	Notify(msg string)
}

var _ LookActor = (*game.CharacterInstance)(nil)

// LookHandlerFactory creates handlers that display the current room.
type LookHandlerFactory struct {
	zones ZoneLocator
}

// NewLookHandlerFactory creates a new LookHandlerFactory with access to world state.
func NewLookHandlerFactory(zones ZoneLocator) *LookHandlerFactory {
	return &LookHandlerFactory{zones: zones}
}

// Spec returns the handler's target and config requirements.
func (f *LookHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer | targetTypeMobile | targetTypeObject | targetTypeExit, Required: false},
		},
		Config: []ConfigRequirement{
			{Name: "target_input", Required: false},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *LookHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *LookHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[LookActor](f.handle), nil
}

func (f *LookHandlerFactory) handle(ctx context.Context, char LookActor, in *CommandInput) error {
	if target := in.Targets["target"]; target != nil {
		return f.showTarget(char, target)
	}

	// Target didn't resolve — check for extra desc fallback.
	if input := in.Config["target_input"]; input != "" {
		return f.showExtraDesc(char, input)
	}

	return f.showRoom(char)
}

// showRoom displays the current room description.
func (f *LookHandlerFactory) showRoom(actor LookActor) error {
	zoneId, roomId := actor.Location()

	ri := f.zones.GetZone(zoneId).GetRoom(roomId)
	if ri == nil {
		return NewUserError("You are in an invalid location.")
	}

	actor.Notify(ri.Describe(actor.Name()))
	return nil
}

// showTarget displays information about a specific target.
func (f *LookHandlerFactory) showTarget(actor LookActor, target *TargetRef) error {
	var msg string
	switch target.Type {
	case targetTypeActor:
		msg = target.Actor.Describe()
	case targetTypeObject:
		msg = target.Obj.Describe()
	case targetTypeExit:
		msg = target.Exit.exit.Description
		if msg == "" {
			msg = "You see nothing special."
		}
	default:
		return NewUserError("You can't look at that.")
	}

	actor.Notify(msg)
	return nil
}

// showExtraDesc searches the current room for an extra description matching
// the keyword on the room itself or on objects in the room.
func (f *LookHandlerFactory) showExtraDesc(actor LookActor, keyword string) error {
	zoneId, roomId := actor.Location()
	ri := f.zones.GetZone(zoneId).GetRoom(roomId)
	if ri == nil {
		return NewUserError("You are in an invalid location.")
	}

	ed := ri.FindExtraDesc(keyword)
	if ed == nil {
		return NewUserError("You don't see '" + display.Capitalize(keyword) + "' here.")
	}

	actor.Notify(display.Wrap(ed.Description))
	return nil
}
