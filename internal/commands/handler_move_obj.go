package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

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
	rooms RoomLocator
	chars storage.Storer[*game.Character]
	pub   game.Publisher
}

func NewMoveObjHandlerFactory(rooms RoomLocator, chars storage.Storer[*game.Character], pub game.Publisher) *MoveObjHandlerFactory {
	return &MoveObjHandlerFactory{rooms: rooms, pub: pub}
}

func (f *MoveObjHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "item", Type: TargetTypeObject, Required: true},
			{Name: "destination", Type: TargetTypePlayer | TargetTypeMobile | TargetTypeObject, Required: false},
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

func (f *MoveObjHandlerFactory) ValidateConfig(config map[string]any) error {
	dest, _ := config["destination"].(string)
	if dest == "" {
		return fmt.Errorf("destination is required")
	}
	return nil
}

func (f *MoveObjHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		item := cmdCtx.Targets["item"]
		if item == nil || item.Obj == nil {
			return NewUserError("Move what?")
		}

		// Check immobile flag
		if item.Obj.instance.Object.Get().HasFlag(game.ObjectFlagImmobile) {
			return NewUserError(fmt.Sprintf("You can't seem to move %s.", item.Obj.Name))
		}

		// Check self-targeting if configured
		if noSelf := cmdCtx.Config["no_self_target"]; noSelf != "" {
			ref := cmdCtx.Targets[noSelf]
			if ref != nil && ref.Player != nil && ref.Player.CharId == cmdCtx.Session.Character.Id() {
				return NewUserError("You can't give something to yourself.")
			}
		}

		// Resolve destination to an ObjectHolder
		dest, err := f.resolveDestination(cmdCtx)
		if err != nil {
			return err
		}

		// Remove from source
		oi := item.Obj.source.RemoveObj(item.Obj.InstanceId)
		if oi == nil {
			return NewUserError(fmt.Sprintf("You don't have %s.", item.Obj.Name))
		}

		// Move
		dest.AddObj(oi)

		if f.pub != nil {
			exclude := []string{cmdCtx.Session.Character.Id()}

			if selfMsg := cmdCtx.Config["self_message"]; selfMsg != "" {
				if err := f.pub.Publish(game.SinglePlayer(cmdCtx.Session.Character.Id()), nil, []byte(selfMsg)); err != nil {
					slog.Warn("failed to publish self message", "error", err)
				}
			}

			if targetMsg := cmdCtx.Config["target_message"]; targetMsg != "" {
				if ref := cmdCtx.Targets[cmdCtx.Config["destination"]]; ref != nil && ref.Type == TargetTypePlayer {
					if err := f.pub.Publish(game.SinglePlayer(ref.Player.CharId), nil, []byte(targetMsg)); err != nil {
						slog.Warn("failed to publish target message", "error", err)
					}
					exclude = append(exclude, ref.Player.CharId)
				}
			}

			room := f.rooms.GetRoom(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId)
			if err := f.pub.Publish(room, exclude, []byte(cmdCtx.Config["room_message"])); err != nil {
				slog.Warn("failed to publish room message", "error", err)
			}
		}

		return nil
	}, nil
}

// resolveDestination maps the "destination" config to an ObjectHolder.
// Returns "inventory" → actor inventory, "room" → room holder,
// or looks up a resolved target and returns its holder.
func (f *MoveObjHandlerFactory) resolveDestination(cmdCtx *CommandContext) (ObjectHolder, error) {
	dest := cmdCtx.Config["destination"]

	switch dest {
	case "inventory":
		return cmdCtx.Actor.Inventory, nil

	case "room":
		return f.rooms.GetRoom(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId), nil

	default:
		return f.holderForTarget(cmdCtx.Targets[dest])
	}
}

// holderForTarget returns an ObjectHolder for a resolved target.
// For player targets, returns their character's inventory.
// For object targets, validates the container flag and returns contents.
// For mobile targets, returns their inventory.
func (f *MoveObjHandlerFactory) holderForTarget(ref *TargetRef) (ObjectHolder, error) {
	if ref.Player != nil {
		char := f.chars.Get(ref.Player.CharId)
		if char == nil {
			return nil, NewUserError(fmt.Sprintf("%s is no longer here.", ref.Player.Name))
		}
		return char.Inventory, nil
	}

	if ref.Mob != nil {
		return ref.Mob.instance.Inventory, nil
	}

	if ref.Obj != nil {
		if !ref.Obj.instance.Object.Get().HasFlag(game.ObjectFlagContainer) {
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

	return nil, fmt.Errorf("target has no player, mobile, or object")
}
