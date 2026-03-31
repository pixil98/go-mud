package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// SaveActor provides the character state needed by the save handler.
type SaveActor interface {
	Id() string
	Notify(msg string)
	SaveCharacter(storage.Storer[*assets.Character]) error
}

var _ SaveActor = (*game.CharacterInstance)(nil)

// SaveHandlerFactory creates handlers that persist the player's character.
type SaveHandlerFactory struct {
	chars storage.Storer[*assets.Character]
}

// NewSaveHandlerFactory creates a handler factory for character save commands.
func NewSaveHandlerFactory(chars storage.Storer[*assets.Character]) *SaveHandlerFactory {
	return &SaveHandlerFactory{chars: chars}
}

// Spec returns the handler's target and config requirements.
func (f *SaveHandlerFactory) Spec() *HandlerSpec {
	return nil
}

// ValidateConfig performs custom validation on the command config.
func (f *SaveHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *SaveHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[SaveActor](f.handle), nil
}

func (f *SaveHandlerFactory) handle(ctx context.Context, char SaveActor, in *CommandInput) error {
	if err := char.SaveCharacter(f.chars); err != nil {
		return fmt.Errorf("saving character: %w", err)
	}

	char.Notify("Character saved.")
	return nil
}
