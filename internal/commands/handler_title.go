package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// TitleActor provides the character state needed by the title handler.
type TitleActor interface {
	CommandActor
	Asset() *assets.Character
}

var _ TitleActor = (*game.CharacterInstance)(nil)

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

func (f *TitleHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

func (f *TitleHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[TitleActor](f.handle), nil
}

func (f *TitleHandlerFactory) handle(ctx context.Context, char TitleActor, in *CommandInput) error {
	// Read new_title from expanded config (input was templated into config)
	title := in.Config["new_title"]

	char.Asset().Title = title

	var output string
	if title == "" {
		output = "Title cleared."
	} else {
		output = fmt.Sprintf("Title set to: %s", title)
	}

	if f.pub != nil {
		return f.pub.Publish(game.SinglePlayer(char.Id()), nil, []byte(output))
	}

	return nil
}
