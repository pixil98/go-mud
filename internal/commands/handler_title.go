package commands

import (
	"context"
	"fmt"
)

// TitleHandlerFactory creates handlers that set a player's title.
type TitleHandlerFactory struct {
	pub Publisher
}

// NewTitleHandlerFactory creates a new TitleHandlerFactory.
func NewTitleHandlerFactory(pub Publisher) *TitleHandlerFactory {
	return &TitleHandlerFactory{pub: pub}
}

func (f *TitleHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "new_title", Required: false},
		},
	}
}

func (f *TitleHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *TitleHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		// Read new_title from expanded config (input was templated into config)
		title := cmdCtx.Config["new_title"]

		cmdCtx.Actor.Title = title

		var output string
		if title == "" {
			output = "Title cleared."
		} else {
			output = fmt.Sprintf("Title set to: %s", title)
		}

		if f.pub != nil {
			return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte(output))
		}

		return nil
	}, nil
}
