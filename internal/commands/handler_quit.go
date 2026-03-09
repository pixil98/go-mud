package commands

import (
	"context"

	"github.com/pixil98/go-mud/internal/game"
)

// QuitActor provides the character state needed by the quit handler.
type QuitActor interface {
	CommandActor
	IsInCombat() bool
	SetQuit(bool)
}

var _ QuitActor = (*game.CharacterInstance)(nil)

// QuitHandlerFactory creates handlers that set the quit flag.
// Character saving is handled by the player manager on session end.
type QuitHandlerFactory struct{}

func NewQuitHandlerFactory() *QuitHandlerFactory {
	return &QuitHandlerFactory{}
}

func (f *QuitHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *QuitHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

func (f *QuitHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[QuitActor](f.handle), nil
}

func (f *QuitHandlerFactory) handle(ctx context.Context, char QuitActor, in *CommandInput) error {
	if char.IsInCombat() {
		return NewUserError("You can't quit while fighting!")
	}

	char.SetQuit(true)
	return nil
}
