package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// WearActor provides the character state needed by the wear handler.
type WearActor interface {
	Id() string
	Name() string
	Notify(msg string)
	Room() *game.RoomInstance
	Inventory() *game.Inventory
	Equip(slot string, obj *game.ObjectInstance) error
}

var _ WearActor = (*game.CharacterInstance)(nil)

// WearHandlerFactory creates handlers for equipping wearable items.
// Targets:
//   - target (required): the object to wear
type WearHandlerFactory struct {
	pub Publisher
}

// NewWearHandlerFactory creates a handler factory for equipping wearable item commands.
func NewWearHandlerFactory(pub Publisher) *WearHandlerFactory {
	return &WearHandlerFactory{pub: pub}
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
	targets := in.Targets["target"]
	if len(targets) == 0 {
		return NewUserError("Wear what?")
	}

	var errs []string
	for _, target := range targets {
		if err := f.wearOne(actor, target); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return NewUserError(strings.Join(errs, "\n"))
	}
	return nil
}

func (f *WearHandlerFactory) wearOne(actor WearActor, target *TargetRef) error {
	obj := target.Obj.instance.Object.Get()

	if !obj.HasFlag(assets.ObjectFlagWearable) {
		return NewUserError(fmt.Sprintf("You can't wear %s.", obj.ShortDesc))
	}

	oi := target.Obj.source.RemoveObj(target.Obj.InstanceId)
	if oi == nil {
		return NewUserError(fmt.Sprintf("You're not carrying %s.", target.Obj.Name))
	}

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

	actor.Notify(fmt.Sprintf("You wear %s.", obj.ShortDesc))
	roomMsg := fmt.Sprintf("%s wears %s.", actor.Name(), obj.ShortDesc)
	_ = f.pub.Publish(actor.Room(), []string{actor.Id()}, []byte(roomMsg))
	return nil
}
