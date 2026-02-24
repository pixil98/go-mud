package commands

import (
	"context"
)

// QuitHandlerFactory creates handlers that set the quit flag.
// Character saving is handled by the player manager on session end.
type QuitHandlerFactory struct{}

func NewQuitHandlerFactory() *QuitHandlerFactory {
	return &QuitHandlerFactory{}
}

func (f *QuitHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *QuitHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *QuitHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		cmdCtx.Session.Quit = true
		return nil
	}, nil
}
