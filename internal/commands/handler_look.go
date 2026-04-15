package commands

import (
	"context"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
)

// LookActor provides the character state needed by the look handler.
type LookActor interface {
	Id() string
	Name() string
	Room() *game.RoomInstance
	Notify(msg string)
	HasGrant(key, arg string) bool
}

var _ LookActor = (*game.CharacterInstance)(nil)

// LookHandlerFactory creates handlers that display the current room.
type LookHandlerFactory struct{}

// NewLookHandlerFactory creates a new LookHandlerFactory.
func NewLookHandlerFactory() *LookHandlerFactory {
	return &LookHandlerFactory{}
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
	return Adapt(f.handle), nil
}

const darkRoomDesc = "It is pitch black..."

// CanSee returns true if the actor can see. Returns false when the actor has
// the dark grant (typically propagated from a dark room) and lacks a
// countering darkvision grant.
func CanSee(actor interface{ HasGrant(string, string) bool }) bool {
	if !actor.HasGrant(assets.PerkGrantDark, "") {
		return true
	}
	return actor.HasGrant(assets.PerkGrantDarkvision, "")
}

// DescribeRoom returns a visibility-aware room description for the actor.
func DescribeRoom(actor interface {
	Name() string
	HasGrant(string, string) bool
}, room *game.RoomInstance) string {
	if !CanSee(actor) {
		return darkRoomDesc
	}
	return room.Describe(actor.Name())
}

func (f *LookHandlerFactory) handle(ctx context.Context, actor LookActor, in *CommandInput) error {
	ri := actor.Room()
	if ri == nil {
		return NewUserError("You are in an invalid location.")
	}

	if !CanSee(actor) {
		actor.Notify(darkRoomDesc)
		return nil
	}

	if target := in.FirstTarget("target"); target != nil {
		return f.showTarget(actor, target)
	}

	if input := in.Config["target_input"]; input != "" {
		return f.showExtraDesc(actor, ri, input)
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
		msg = target.Exit.exit.Exit.Description
		if msg == "" {
			msg = "You see nothing special."
		}
	default:
		return NewUserError("You can't look at that.")
	}

	actor.Notify(msg)
	return nil
}

// showExtraDesc searches the room for an extra description matching
// the keyword on the room itself or on objects in the room.
func (f *LookHandlerFactory) showExtraDesc(actor LookActor, ri *game.RoomInstance, keyword string) error {
	ed := ri.FindExtraDesc(keyword)
	if ed == nil {
		return NewUserError("You don't see '" + display.Capitalize(keyword) + "' here.")
	}

	actor.Notify(display.Wrap(ed.Description))
	return nil
}
