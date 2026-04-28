package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
)

// MoveObjActor provides the character state needed by the move_obj handler.
type MoveObjActor interface {
	Id() string
	Publish(data []byte, exclude []string)
	Room() *game.RoomInstance
	Inventory() *game.Inventory
}

var _ MoveObjActor = (*game.CharacterInstance)(nil)

// ObjectHolder can have objects added and removed.
type ObjectHolder interface {
	ObjectRemover
	AddObj(obj *game.ObjectInstance)
}

// MoveObjHandlerFactory creates handlers that move objects between holders.
// Config:
//   - destination (required): "inventory", "room", or a target name
//   - message (required): Go template for room broadcast
//   - no_self_target (optional): target name to prevent self-targeting
type MoveObjHandlerFactory struct{}

// NewMoveObjHandlerFactory creates a handler factory for object movement commands.
func NewMoveObjHandlerFactory() *MoveObjHandlerFactory {
	return &MoveObjHandlerFactory{}
}

// Spec returns the handler's target and config requirements.
func (f *MoveObjHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "item", Type: targetTypeObject, Required: true},
			{Name: "destination", Type: targetTypePlayer | targetTypeMobile | targetTypeObject, Required: false},
		},
		Config: []ConfigRequirement{
			{Name: "destination", Required: true},
			{Name: "room_message", Required: true},
			{Name: "self_message", Required: false},
			{Name: "target_message", Required: false},
			{Name: "no_self_target", Required: false},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *MoveObjHandlerFactory) ValidateConfig(config map[string]string) error {
	dest := config["destination"]
	if dest == "" {
		return errors.New("destination is required")
	}
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *MoveObjHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[MoveObjActor](f.handle), nil
}

func (f *MoveObjHandlerFactory) handle(ctx context.Context, char MoveObjActor, in *CommandInput) error {
	items := in.Targets["item"]
	if len(items) == 0 {
		return NewUserError("Move what?")
	}

	// Check self-targeting if configured
	if noSelf := in.Config["no_self_target"]; noSelf != "" {
		ref := in.FirstTarget(noSelf)
		if ref != nil && ref.Actor != nil && ref.Actor.CharId == char.Id() {
			return NewUserError("You can't give something to yourself.")
		}
	}

	// Resolve destination to an ObjectHolder
	dest, err := f.resolveDestination(char, in)
	if err != nil {
		return err
	}

	var moved int
	for _, item := range items {
		if item.Obj.instance.Object.Get().HasFlag(assets.ObjectFlagImmobile) {
			char.Publish([]byte(fmt.Sprintf("You can't seem to move %s.", item.Obj.Name)), nil)
			continue
		}

		oi := item.Obj.source.RemoveObj(item.Obj.InstanceId)
		if oi == nil {
			continue
		}
		oi.ActivateDecay()
		dest.AddObj(oi)
		moved++
	}

	if moved == 0 {
		return nil
	}

	if selfMsg := in.Config["self_message"]; selfMsg != "" {
		char.Publish([]byte(selfMsg), nil)
	}
	// For multi-item moves, the template only names the first item.
	// TODO: build per-item or summary messages for all. targeting.

	exclude := []string{char.Id()}

	if targetMsg := in.Config["target_message"]; targetMsg != "" {
		if ref := in.FirstTarget(in.Config["destination"]); ref != nil && ref.Actor != nil {
			ref.Actor.actor.Publish([]byte(targetMsg), nil)
			exclude = append(exclude, ref.Actor.CharId)
		}
	}

	if roomMsg := in.Config["room_message"]; roomMsg != "" {
		char.Room().Publish([]byte(roomMsg), exclude)
	}

	return nil
}

// resolveDestination maps the "destination" config to an ObjectHolder.
// Returns "inventory" → session inventory, "room" → room holder,
// or looks up a resolved target and returns its holder.
func (f *MoveObjHandlerFactory) resolveDestination(char MoveObjActor, in *CommandInput) (ObjectHolder, error) {
	dest := in.Config["destination"]

	switch dest {
	case "inventory":
		return char.Inventory(), nil

	case "room":
		return char.Room(), nil

	default:
		return f.holderForTarget(in.FirstTarget(dest))
	}
}

// holderForTarget returns an ObjectHolder for a resolved target.
// For actor targets (player/mob), returns their inventory.
// For object targets, validates the container flag and returns contents.
func (f *MoveObjHandlerFactory) holderForTarget(ref *TargetRef) (ObjectHolder, error) {
	if ref.Actor != nil {
		return ref.Actor.actor.Inventory(), nil
	}

	if ref.Obj != nil {
		if !ref.Obj.instance.Object.Get().HasFlag(assets.ObjectFlagContainer) {
			return nil, NewUserError(fmt.Sprintf("%s is not a container.", display.Capitalize(ref.Obj.Name)))
		}
		if ref.Obj.instance.Locked {
			return nil, NewUserError(fmt.Sprintf("%s is locked.", ref.Obj.ClosureName()))
		}
		if ref.Obj.instance.Closed {
			return nil, NewUserError(fmt.Sprintf("%s is closed.", ref.Obj.ClosureName()))
		}
		return ref.Obj.instance.Contents, nil
	}

	return nil, errors.New("target has no actor or object")
}
