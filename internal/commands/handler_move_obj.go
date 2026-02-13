package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// MoveObjHandlerFactory creates handlers that move objects between holders.
// Config:
//   - destination (required): "inventory", "room", or a target name
//   - message (required): Go template for room broadcast
//   - no_self_target (optional): target name to prevent self-targeting
type MoveObjHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

func NewMoveObjHandlerFactory(world *game.WorldState, pub Publisher) *MoveObjHandlerFactory {
	return &MoveObjHandlerFactory{world: world, pub: pub}
}

func (f *MoveObjHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "item", Type: TargetTypeObject, Required: true},
			{Name: "container", Type: TargetTypeObject, Required: false},
			{Name: "recipient", Type: TargetTypePlayer, Required: false},
		},
		Config: []ConfigRequirement{
			{Name: "destination", Required: true},
			{Name: "message", Required: true},
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
		obj := f.world.Objects().Get(string(item.Obj.ObjectId))
		if obj != nil && obj.HasFlag(game.ObjectFlagImmobile) {
			return NewUserError(fmt.Sprintf("You can't seem to move %s.", item.Obj.Name))
		}

		// Check self-targeting if configured
		if noSelf := cmdCtx.Config["no_self_target"]; noSelf != "" {
			ref := cmdCtx.Targets[noSelf]
			if ref != nil && ref.Player != nil && ref.Player.CharId == cmdCtx.Session.CharId {
				return NewUserError("You can't give something to yourself.")
			}
		}

		// Resolve destination to an ObjectHolder
		dest, err := f.resolveDestination(cmdCtx)
		if err != nil {
			return err
		}

		// Remove from source
		oi := item.Obj.Source.Remove(item.Obj.InstanceId)
		if oi == nil {
			return NewUserError(fmt.Sprintf("You don't have %s.", item.Obj.Name))
		}

		// Move
		dest.Add(oi)

		// Broadcast to room
		if f.pub != nil {
			msg := cmdCtx.Config["message"]
			return f.pub.PublishToRoom(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId, []byte(msg))
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
		if cmdCtx.Actor.Inventory == nil {
			cmdCtx.Actor.Inventory = game.NewInventory()
		}
		return cmdCtx.Actor.Inventory, nil

	case "room":
		return f.world.RoomHolder(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId), nil

	default:
		ref := cmdCtx.Targets[dest]
		if ref == nil {
			return nil, NewUserError(fmt.Sprintf("Target %q not found.", dest))
		}
		return f.holderForTarget(ref)
	}
}

// holderForTarget returns an ObjectHolder for a resolved target.
// For player targets, returns their character's inventory.
// For object targets, validates the container flag and returns contents.
// For mobile targets, returns their inventory (once supported).
func (f *MoveObjHandlerFactory) holderForTarget(ref *TargetRef) (ObjectHolder, error) {
	if ref.Player != nil {
		char := f.world.Characters().Get(string(ref.Player.CharId))
		if char == nil {
			return nil, NewUserError(fmt.Sprintf("%s is no longer here.", ref.Player.Name))
		}
		if char.Inventory == nil {
			char.Inventory = game.NewInventory()
		}
		return char.Inventory, nil
	}

	if ref.Mob != nil {
		// TODO: MobileInstance needs an Inventory field to support this
		return nil, NewUserError("You can't do that.")
	}

	if ref.Obj != nil {
		objDef := f.world.Objects().Get(string(ref.Obj.ObjectId))
		if objDef == nil || !objDef.HasFlag(game.ObjectFlagContainer) {
			name := strings.ToUpper(ref.Obj.Name[:1]) + ref.Obj.Name[1:]
			return nil, NewUserError(fmt.Sprintf("%s is not a container.", name))
		}
		if ref.Obj.Instance.Contents == nil {
			ref.Obj.Instance.Contents = game.NewInventory()
		}
		return ref.Obj.Instance.Contents, nil
	}

	return nil, fmt.Errorf("target has no player, mobile, or object")
}
