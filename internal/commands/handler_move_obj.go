package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// MoveObjActor provides the character state needed by the move_obj handler.
type MoveObjActor interface {
	Id() string
	Notify(msg string)
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
type MoveObjHandlerFactory struct {
	pub Publisher
}

// NewMoveObjHandlerFactory creates a handler factory for object movement commands.
func NewMoveObjHandlerFactory(pub Publisher) *MoveObjHandlerFactory {
	return &MoveObjHandlerFactory{pub: pub}
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
		return fmt.Errorf("destination is required")
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
			char.Notify(fmt.Sprintf("You can't seem to move %s.", item.Obj.Name))
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
		char.Notify(selfMsg)
	}
	// For multi-item moves, the template only names the first item.
	// TODO: build per-item or summary messages for all. targeting.

	if f.pub != nil {
		exclude := []string{char.Id()}

		if targetMsg := in.Config["target_message"]; targetMsg != "" {
			if ref := in.FirstTarget(in.Config["destination"]); ref != nil && ref.Actor != nil && ref.Actor.CharId != "" {
				if err := f.pub.Publish(game.SinglePlayer(ref.Actor.CharId), nil, []byte(targetMsg)); err != nil {
					slog.Warn("failed to publish target message", "error", err)
				}
				exclude = append(exclude, ref.Actor.CharId)
			}
		}

		if roomMsg := in.Config["room_message"]; roomMsg != "" {
			if err := f.pub.Publish(char.Room(), exclude, []byte(roomMsg)); err != nil {
				slog.Warn("failed to publish room message", "error", err)
			}
		}
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
			name := strings.ToUpper(ref.Obj.Name[:1]) + ref.Obj.Name[1:]
			return nil, NewUserError(fmt.Sprintf("%s is not a container.", name))
		}
		if ref.Obj.instance.Locked {
			return nil, NewUserError(fmt.Sprintf("%s is locked.", ref.Obj.ClosureName()))
		}
		if ref.Obj.instance.Closed {
			return nil, NewUserError(fmt.Sprintf("%s is closed.", ref.Obj.ClosureName()))
		}
		return ref.Obj.instance.Contents, nil
	}

	return nil, fmt.Errorf("target has no actor or object")
}
