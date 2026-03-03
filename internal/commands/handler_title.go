package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// TitleHandlerFactory creates handlers that set a player's title.
type TitleHandlerFactory struct {
	pub game.Publisher
}

// NewTitleHandlerFactory creates a new TitleHandlerFactory.
func NewTitleHandlerFactory(pub game.Publisher) *TitleHandlerFactory {
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
	return func(ctx context.Context, in *CommandInput) error {
		// Read new_title from expanded config (input was templated into config)
		title := in.Config["new_title"]

		in.Char.Character.Get().Title = title

		var output string
		if title == "" {
			output = "Title cleared."
		} else {
			output = fmt.Sprintf("Title set to: %s", title)
		}

		if f.pub != nil {
			return f.pub.Publish(game.SinglePlayer(in.Char.Character.Id()), nil, []byte(output))
		}

		return nil
	}, nil
}
