package commands

import (
	"context"
	"fmt"
	"strings"
)

// TitleHandlerFactory creates handlers that set a player's title.
type TitleHandlerFactory struct {
	pub Publisher
}

// NewTitleHandlerFactory creates a new TitleHandlerFactory.
func NewTitleHandlerFactory(pub Publisher) *TitleHandlerFactory {
	return &TitleHandlerFactory{pub: pub}
}

func (f *TitleHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *TitleHandlerFactory) Create(config map[string]any) (CommandFunc, error) {
	return func(ctx context.Context, data *TemplateData) error {
		title := ""
		if text, ok := data.Args["text"].(string); ok {
			title = text
		}

		data.Actor.Title = title

		var output string
		if title == "" {
			output = "Title cleared."
		} else {
			output = fmt.Sprintf("Title set to: %s", title)
		}

		playerChannel := fmt.Sprintf("player-%s", strings.ToLower(data.Actor.Name()))
		if f.pub != nil {
			_ = f.pub.Publish(playerChannel, []byte(output))
		}

		return nil
	}, nil
}
