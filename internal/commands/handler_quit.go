package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// QuitHandlerFactory creates handlers that save and quit.
type QuitHandlerFactory struct {
	world *game.WorldState
}

func NewQuitHandlerFactory(world *game.WorldState) *QuitHandlerFactory {
	return &QuitHandlerFactory{world: world}
}

func (f *QuitHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *QuitHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *QuitHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		if err := saveCharacter(f.world, cmdCtx); err != nil {
			return fmt.Errorf("saving character on quit: %w", err)
		}

		cmdCtx.Session.Quit = true
		return nil
	}, nil
}
