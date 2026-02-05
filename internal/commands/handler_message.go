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
type MessageHandlerFactory struct {
	pub Publisher
}

// NewMessageHandlerFactory creates a new MessageHandlerFactory with a publisher.
func NewMessageHandlerFactory(pub Publisher) *MessageHandlerFactory {
	return &MessageHandlerFactory{pub: pub}
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

func (f *MessageHandlerFactory) Create(config map[string]any) (CommandFunc, error) {
	recipientChannel, _ := config["recipient_channel"].(string)
	recipientMessage, _ := config["recipient_message"].(string)
	senderChannel, _ := config["sender_channel"].(string)
	senderMessage, _ := config["sender_message"].(string)

	return func(ctx context.Context, data *TemplateData) error {
		// Send confirmation to sender if configured
		if senderChannel != "" {
			channel, err := ExpandTemplate(senderChannel, data)
			if err != nil {
				return fmt.Errorf("expanding sender channel template: %w", err)
			}

			message, err := ExpandTemplate(senderMessage, data)
			if err != nil {
				return fmt.Errorf("expanding sender message template: %w", err)
			}

			if err := f.pub.Publish(channel, []byte(message)); err != nil {
				return err
			}
		}

		// Send message to recipient if configured
		if recipientChannel != "" {
			channel, err := ExpandTemplate(recipientChannel, data)
			if err != nil {
				return fmt.Errorf("expanding recipient channel template: %w", err)
			}

			message, err := ExpandTemplate(recipientMessage, data)
			if err != nil {
				return fmt.Errorf("expanding recipient message template: %w", err)
			}

			if err := f.pub.Publish(channel, []byte(message)); err != nil {
				return err
			}
		}

		return nil
	}, nil
}
