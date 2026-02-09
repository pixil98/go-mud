package commands

import "context"

// QuitHandlerFactory creates handlers that signal the player wants to quit.
type QuitHandlerFactory struct{}

func (f *QuitHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *QuitHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *QuitHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		if cmdCtx.Session != nil {
			cmdCtx.Session.Quit = true
		}
		return nil
	}, nil
}
