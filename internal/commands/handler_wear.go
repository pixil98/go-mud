package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// WearActor provides the character state needed by the wear handler.
type WearActor interface {
	Id() string
	Notify(msg string)
	Location() (zoneId, roomId string)
	Inventory() *game.Inventory
	Equip(slot string, obj *game.ObjectInstance) error
	Asset() *assets.Character
}

var _ WearActor = (*game.CharacterInstance)(nil)

// WearHandlerFactory creates handlers for equipping wearable items.
// Targets:
//   - target (required): the object to wear
type WearHandlerFactory struct {
	zones ZoneLocator
	pub   game.Publisher
}

// NewWearHandlerFactory creates a handler factory for equipping wearable item commands.
func NewWearHandlerFactory(zones ZoneLocator, pub game.Publisher) *WearHandlerFactory {
	return &WearHandlerFactory{zones: zones, pub: pub}
}

// Spec returns the handler's target and config requirements.
func (f *WearHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeObject, Required: true},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *WearHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *WearHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[WearActor](f.handle), nil
}

func (f *WearHandlerFactory) handle(ctx context.Context, actor WearActor, in *CommandInput) error {
	target := in.Targets["target"]
	if target == nil || target.Obj == nil {
		return NewUserError("Wear what?")
	}

	// Use resolved object definition
	obj := target.Obj.instance.Object.Get()

	// Check if the item is wearable
	if !obj.HasFlag(assets.ObjectFlagWearable) {
		return NewUserError(fmt.Sprintf("You can't wear %s.", obj.ShortDesc))
	}

	// Remove from source
	oi := target.Obj.source.RemoveObj(target.Obj.InstanceId)
	if oi == nil {
		return NewUserError(fmt.Sprintf("You're not carrying %s.", target.Obj.Name))
	}

	// Try each slot the item supports until one succeeds
	var slot string
	var slotFull bool
	for _, s := range obj.WearSlots {
		err := actor.Equip(s, oi)
		if err == nil {
			slot = s
			break
		}
		if errors.Is(err, game.ErrSlotFull) {
			slotFull = true
		}
	}
	if slot == "" {
		actor.Inventory().AddObj(oi)
		if slotFull {
			return NewUserError("You're already wearing something in that slot.")
		}
		return NewUserError(fmt.Sprintf("You have nowhere to wear %s.", obj.ShortDesc))
	}

	// Send self message
	actor.Notify(fmt.Sprintf("You wear %s.", obj.ShortDesc))

	// Broadcast to room
	roomMsg := fmt.Sprintf("%s wears %s.", actor.Asset().Name, obj.ShortDesc)
	zoneId, roomId := actor.Location()
	room := f.zones.GetZone(zoneId).GetRoom(roomId)
	return f.pub.Publish(room, []string{actor.Id()}, []byte(roomMsg))
}
