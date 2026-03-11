package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// MessageActor provides the character state needed by the message handler.
type MessageActor interface {
	Id() string
	Location() (string, string)
	GetGroup() *game.Group
}

var _ MessageActor = (*game.CharacterInstance)(nil)

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

// Spec returns the handler's target and config requirements.
func (f *MessageHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "scope", Required: true},
			{Name: "recipient_message", Required: true},
			{Name: "sender_message", Required: false},
		},
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer, Required: false},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *MessageHandlerFactory) ValidateConfig(config map[string]string) error {
	scope := config["scope"]
	switch scope {
	case "room", "zone", "world", "player", "group":
		// valid
	default:
		return fmt.Errorf("scope must be room, zone, world, player, or group (got %q)", scope)
	}

	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *MessageHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[MessageActor](f.handle), nil
}

func (f *MessageHandlerFactory) handle(ctx context.Context, char MessageActor, in *CommandInput) error {
	scope := in.Config["scope"]
	recipientMessage := in.Config["recipient_message"]
	senderMessage := in.Config["sender_message"]

	// Send 2nd-person message to actor if configured
	if senderMessage != "" {
		if err := f.pub.Publish(game.SinglePlayer(char.Id()), nil, []byte(senderMessage)); err != nil {
			return err
		}
	}

	// Send message to scope targets, excluding actor only if they got a sender_message
	zoneId, roomId := char.Location()
	var exclude []string
	if senderMessage != "" {
		exclude = []string{char.Id()}
	}

	switch scope {
	case "room":
		room := f.world.GetZone(zoneId).GetRoom(roomId)
		return f.pub.Publish(room, exclude, []byte(recipientMessage))

	case "zone":
		zone := f.world.GetZone(zoneId)
		return f.pub.Publish(zone, exclude, []byte(recipientMessage))

	case "world":
		return f.pub.Publish(f.world, exclude, []byte(recipientMessage))

	case "player":
		target := in.Targets["target"]
		if target == nil || target.Actor == nil {
			return NewUserError("They're not here.")
		}
		return f.pub.Publish(game.SinglePlayer(target.Actor.CharId), nil, []byte(recipientMessage))

	case "group":
		grp := char.GetGroup()
		if grp == nil {
			return NewUserError("You are not in a group.")
		}
		return f.pub.Publish(grp, exclude, []byte(recipientMessage))
	}

	return nil
}
