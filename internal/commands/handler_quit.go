package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// QuitHandlerFactory creates handlers that save and quit.
type QuitHandlerFactory struct {
	chars storage.Storer[*game.Character]
}

func NewQuitHandlerFactory(chars storage.Storer[*game.Character]) *QuitHandlerFactory {
	return &QuitHandlerFactory{chars: chars}
}

func (f *QuitHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *QuitHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *QuitHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		if err := saveCharacter(f.chars, cmdCtx.Session); err != nil {
			return fmt.Errorf("saving character on quit: %w", err)
		}

		cmdCtx.Session.Quit = true
		return nil
	}, nil
}
