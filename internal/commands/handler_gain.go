package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// GainActor provides the character state needed by the gain handler.
type GainActor interface {
	Id() string
	Notify(msg string)
	IsInCombat() bool
	Asset() *assets.Character
	Gain()
}

var _ GainActor = (*game.CharacterInstance)(nil)

// GainHandlerFactory creates handlers for the gain (level up) command.
type GainHandlerFactory struct{}

// NewGainHandlerFactory creates a handler factory for gain (level up) commands.
func NewGainHandlerFactory() *GainHandlerFactory {
	return &GainHandlerFactory{}
}

// Spec returns the handler's target and config requirements.
func (f *GainHandlerFactory) Spec() *HandlerSpec {
	return nil
}

// ValidateConfig performs custom validation on the command config.
func (f *GainHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *GainHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[GainActor](f.handle), nil
}

func (f *GainHandlerFactory) handle(ctx context.Context, char GainActor, in *CommandInput) error {
	if char.IsInCombat() {
		return NewUserError("You can't train while fighting!")
	}

	actor := char.Asset()

	if actor.Level >= game.MaxLevel {
		return NewUserError("You have reached the maximum level.")
	}

	needed := game.ExpToNextLevel(actor.Level, actor.Experience)
	if needed > 0 {
		return NewUserError(fmt.Sprintf(
			"You need %d more experience to reach level %d.",
			needed, actor.Level+1))
	}

	char.Gain()

	char.Notify(fmt.Sprintf("Congratulations! You have advanced to level %d!", actor.Level))
	return nil
}
