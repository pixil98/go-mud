package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// TitleActor provides the character state needed by the title handler.
type TitleActor interface {
	Id() string
	Notify(msg string)
	SetTitle(string)
}

var _ TitleActor = (*game.CharacterInstance)(nil)

// TitleHandlerFactory creates handlers that set a player's title.
type TitleHandlerFactory struct{}

// NewTitleHandlerFactory creates a new TitleHandlerFactory.
func NewTitleHandlerFactory() *TitleHandlerFactory {
	return &TitleHandlerFactory{}
}

// Spec returns the handler's target and config requirements.
func (f *TitleHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "new_title", Required: false},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *TitleHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *TitleHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[TitleActor](f.handle), nil
}

func (f *TitleHandlerFactory) handle(ctx context.Context, char TitleActor, in *CommandInput) error {
	// Read new_title from expanded config (input was templated into config)
	title := in.Config["new_title"]

	char.SetTitle(title)

	var output string
	if title == "" {
		output = "Title cleared."
	} else {
		output = fmt.Sprintf("Title set to: %s", title)
	}

	char.Notify(output)
	return nil
}
