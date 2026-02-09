package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// GiveHandlerFactory creates handlers for giving objects to other players.
// Targets:
//   - item (required): the object to give (from inventory)
//   - recipient (required): the player to give it to (in room)
type GiveHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

func NewGiveHandlerFactory(world *game.WorldState, pub Publisher) *GiveHandlerFactory {
	return &GiveHandlerFactory{world: world, pub: pub}
}

func (f *GiveHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "item", Type: TargetTypeObject, Required: true},
			{Name: "recipient", Type: TargetTypePlayer, Required: true},
		},
	}
}

func (f *GiveHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *GiveHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		item := cmdCtx.Targets["item"]
		if item == nil || item.Obj == nil {
			return NewUserError("Give what?")
		}

		recipient := cmdCtx.Targets["recipient"]
		if recipient == nil || recipient.Player == nil {
			return NewUserError("Give to whom?")
		}

		// Can't give to yourself
		// TODO: Consider adding CharId to PlayerState (similar to MobileInstance/ObjectInstance)
		// so we can compare by ID instead of name
		if recipient.Player.Name == cmdCtx.Actor.Name {
			return NewUserError("You can't give something to yourself.")
		}

		// Remove from actor's inventory
		if cmdCtx.Actor.Inventory == nil {
			return NewUserError(fmt.Sprintf("You're not carrying %s.", item.Name))
		}

		oi := cmdCtx.Actor.Inventory.Remove(item.Obj.InstanceId)
		if oi == nil {
			return NewUserError(fmt.Sprintf("You're not carrying %s.", item.Name))
		}

		// Add to recipient's inventory
		recipientChar := f.world.Characters().Get(string(recipient.Player.CharId))
		if recipientChar == nil {
			// Shouldn't happen if resolver worked, but handle gracefully
			cmdCtx.Actor.Inventory.Add(oi) // Put it back
			return NewUserError(fmt.Sprintf("%s is no longer here.", recipient.Name))
		}

		if recipientChar.Inventory == nil {
			recipientChar.Inventory = game.NewInventory()
		}
		recipientChar.Inventory.Add(oi)

		// Broadcast to room
		if f.pub != nil {
			obj := f.world.Objects().Get(string(oi.ObjectId))
			msg := fmt.Sprintf("%s gives %s to %s.", cmdCtx.Actor.Name, obj.ShortDesc, recipientChar.Name)
			roomChannel := fmt.Sprintf("zone-%s-room-%s", cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId)
			_ = f.pub.Publish(roomChannel, []byte(msg))
		}

		return nil
	}, nil
}
