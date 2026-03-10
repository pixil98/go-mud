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
	CommandActor
	Location() (zoneId, roomId string)
	GetInventory() *game.Inventory
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
	zones ZoneLocator
	pub   game.Publisher
}

// NewMoveObjHandlerFactory creates a handler factory for object movement commands.
func NewMoveObjHandlerFactory(zones ZoneLocator, pub game.Publisher) *MoveObjHandlerFactory {
	return &MoveObjHandlerFactory{zones: zones, pub: pub}
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
	item := in.Targets["item"]
	if item == nil || item.Obj == nil {
		return NewUserError("Move what?")
	}

	// Check immobile flag
	if item.Obj.instance.Object.Get().HasFlag(assets.ObjectFlagImmobile) {
		return NewUserError(fmt.Sprintf("You can't seem to move %s.", item.Obj.Name))
	}

	// Check self-targeting if configured
	if noSelf := in.Config["no_self_target"]; noSelf != "" {
		ref := in.Targets[noSelf]
		if ref != nil && ref.Player != nil && ref.Player.CharId == char.Id() {
			return NewUserError("You can't give something to yourself.")
		}
	}

	// Resolve destination to an ObjectHolder
	dest, err := f.resolveDestination(char, in)
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
		exclude := []string{char.Id()}

		if selfMsg := in.Config["self_message"]; selfMsg != "" {
			if err := f.pub.Publish(game.SinglePlayer(char.Id()), nil, []byte(selfMsg)); err != nil {
				slog.Warn("failed to publish self message", "error", err)
			}
		}

		if targetMsg := in.Config["target_message"]; targetMsg != "" {
			if ref := in.Targets[in.Config["destination"]]; ref != nil && ref.Type == targetTypePlayer {
				if err := f.pub.Publish(game.SinglePlayer(ref.Player.CharId), nil, []byte(targetMsg)); err != nil {
					slog.Warn("failed to publish target message", "error", err)
				}
				exclude = append(exclude, ref.Player.CharId)
			}
		}

		zoneId, roomId := char.Location()
		room := f.zones.GetZone(zoneId).GetRoom(roomId)
		if err := f.pub.Publish(room, exclude, []byte(in.Config["room_message"])); err != nil {
			slog.Warn("failed to publish room message", "error", err)
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
		return char.GetInventory(), nil

	case "room":
		zoneId, roomId := char.Location()
		return f.zones.GetZone(zoneId).GetRoom(roomId), nil

	default:
		return f.holderForTarget(in.Targets[dest])
	}
}

// holderForTarget returns an ObjectHolder for a resolved target.
// For player targets, returns their session inventory.
// For object targets, validates the container flag and returns contents.
// For mobile targets, returns their inventory.
func (f *MoveObjHandlerFactory) holderForTarget(ref *TargetRef) (ObjectHolder, error) {
	if ref.Player != nil {
		if ref.Player.session == nil {
			return nil, NewUserError(fmt.Sprintf("%s is no longer here.", ref.Player.Name))
		}
		return ref.Player.session.GetInventory(), nil
	}

	if ref.Mob != nil {
		return ref.Mob.instance.GetInventory(), nil
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

	return nil, fmt.Errorf("target has no player, mobile, or object")
}
