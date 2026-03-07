package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// SaveHandlerFactory creates handlers that persist the player's character.
type SaveHandlerFactory struct {
	chars storage.Storer[*assets.Character]
	pub   game.Publisher
}

func NewSaveHandlerFactory(chars storage.Storer[*assets.Character], pub game.Publisher) *SaveHandlerFactory {
	return &SaveHandlerFactory{chars: chars, pub: pub}
}

func (f *SaveHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *SaveHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *SaveHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, in *CommandInput) error {
		if err := in.Char.SaveCharacter(f.chars); err != nil {
			return fmt.Errorf("saving character: %w", err)
		}

		if f.pub != nil {
			return f.pub.Publish(game.SinglePlayer(in.Char.Id()), nil, []byte("Character saved."))
		}

		return nil
	}, nil
}
