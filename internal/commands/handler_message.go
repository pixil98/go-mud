package commands

import (
	"context"
	"fmt"
)

// MessageHandlerFactory creates handlers that publish messages to channels.
// Config:
//   - recipient_channel (optional): template for the recipient's channel
//   - recipient_message (required if recipient_channel set): template for message to recipient
//   - sender_channel (optional): template for sender's channel (for confirmation)
//   - sender_message (required if sender_channel set): template for sender confirmation
//
// TODO: This handler uses raw Publish with config-driven channel names. Evaluate whether
// it can use typed Publisher methods now that plugins won't define custom channels.
type MessageHandlerFactory struct {
	pub Publisher
}

// NewMessageHandlerFactory creates a new MessageHandlerFactory with a publisher.
func NewMessageHandlerFactory(pub Publisher) *MessageHandlerFactory {
	return &MessageHandlerFactory{pub: pub}
}

func (f *MessageHandlerFactory) Spec() *HandlerSpec {
	// Conditional requirements (e.g., recipient_message required when recipient_channel set)
	// are handled by ValidateConfig below.
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "recipient_channel", Required: false},
			{Name: "recipient_message", Required: false},
			{Name: "sender_channel", Required: false},
			{Name: "sender_message", Required: false},
		},
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypePlayer, Required: false},
		},
	}
}

func (f *MessageHandlerFactory) ValidateConfig(config map[string]any) error {
	recipientChannel, _ := config["recipient_channel"].(string)
	recipientMessage, _ := config["recipient_message"].(string)
	if recipientChannel != "" && recipientMessage == "" {
		return fmt.Errorf("recipient_message is required when recipient_channel is set")
	}

	senderChannel, _ := config["sender_channel"].(string)
	senderMessage, _ := config["sender_message"].(string)
	if senderChannel != "" && senderMessage == "" {
		return fmt.Errorf("sender_message is required when sender_channel is set")
	}

	if recipientChannel == "" && senderChannel == "" {
		return fmt.Errorf("at least one of recipient_channel or sender_channel is required")
	}

	return nil
}

func (f *MessageHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		// Config values are already expanded by the framework
		recipientChannel := cmdCtx.Config["recipient_channel"]
		recipientMessage := cmdCtx.Config["recipient_message"]
		senderChannel := cmdCtx.Config["sender_channel"]
		senderMessage := cmdCtx.Config["sender_message"]

		// Send confirmation to sender if configured
		if senderChannel != "" {
			if err := f.pub.Publish(senderChannel, []byte(senderMessage)); err != nil {
				return err
			}
		}

		// Send message to recipient if configured
		if recipientChannel != "" {
			if err := f.pub.Publish(recipientChannel, []byte(recipientMessage)); err != nil {
				return err
			}
		}

		return nil
	}, nil
}
