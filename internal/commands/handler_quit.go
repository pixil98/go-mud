package commands

import "context"

// QuitHandlerFactory creates handlers that signal the player wants to quit.
type QuitHandlerFactory struct{}

func (f *QuitHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *QuitHandlerFactory) Create(config map[string]any) (CommandFunc, error) {
	return func(ctx context.Context, data *TemplateData) error {
		if data.State != nil {
			data.State.Quit = true
		}
		return nil
	}, nil
}
