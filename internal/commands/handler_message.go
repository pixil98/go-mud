package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// MessageHandlerFactory creates handlers that publish messages to scoped groups.
// Config:
//   - scope (required): "room", "zone", "world", or "player"
//   - recipient_message (required): template for message sent to scope targets
//   - sender_message (optional): template for 2nd-person message sent to actor
type MessageHandlerFactory struct {
	world WorldView
	pub   game.Publisher
}

// NewMessageHandlerFactory creates a new MessageHandlerFactory with a publisher.
func NewMessageHandlerFactory(world WorldView, pub game.Publisher) *MessageHandlerFactory {
	return &MessageHandlerFactory{world: world, pub: pub}
}

func (f *MessageHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "scope", Required: true},
			{Name: "recipient_message", Required: true},
			{Name: "sender_message", Required: false},
		},
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypePlayer, Required: false},
		},
	}
}

func (f *MessageHandlerFactory) ValidateConfig(config map[string]any) error {
	scope, _ := config["scope"].(string)
	switch scope {
	case "room", "zone", "world", "player":
		// valid
	default:
		return fmt.Errorf("scope must be room, zone, world, or player (got %q)", scope)
	}

	return nil
}

func (f *MessageHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		scope := cmdCtx.Config["scope"]
		recipientMessage := cmdCtx.Config["recipient_message"]
		senderMessage := cmdCtx.Config["sender_message"]

		// Send 2nd-person message to actor if configured
		if senderMessage != "" {
			if err := f.pub.Publish(game.SinglePlayer(cmdCtx.Session.Character.Id()), nil, []byte(senderMessage)); err != nil {
				return err
			}
		}

		// Send message to scope targets, excluding actor only if they got a sender_message
		zoneId, roomId := cmdCtx.Session.Location()
		var exclude []string
		if senderMessage != "" {
			exclude = []string{cmdCtx.Session.Character.Id()}
		}

		switch scope {
		case "room":
			room := f.world.GetRoom(zoneId, roomId)
			return f.pub.Publish(room, exclude, []byte(recipientMessage))

		case "zone":
			zone := f.world.GetZone(zoneId)
			return f.pub.Publish(zone, exclude, []byte(recipientMessage))

		case "world":
			return f.pub.Publish(f.world, exclude, []byte(recipientMessage))

		case "player":
			target := cmdCtx.Targets["target"]
			if target == nil || target.Player == nil {
				return NewUserError("They're not here.")
			}
			return f.pub.Publish(game.SinglePlayer(target.Player.CharId), nil, []byte(recipientMessage))
		}

		return nil
	}, nil
}
