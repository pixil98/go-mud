package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// GainHandlerFactory creates handlers for the gain (level up) command.
type GainHandlerFactory struct {
	pub Publisher
}

func NewGainHandlerFactory(pub Publisher) *GainHandlerFactory {
	return &GainHandlerFactory{pub: pub}
}

func (f *GainHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *GainHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *GainHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		char := cmdCtx.Actor

		if cmdCtx.Session.InCombat {
			return NewUserError("You can't train while fighting!")
		}

		if char.Level >= game.MaxLevel {
			return NewUserError("You have reached the maximum level.")
		}

		needed := game.ExpToNextLevel(char.Level, char.Experience)
		if needed > 0 {
			return NewUserError(fmt.Sprintf(
				"You need %d more experience to reach level %d.",
				needed, char.Level+1))
		}

		char.Gain()

		msg := fmt.Sprintf("Congratulations! You have advanced to level %d!", char.Level)

		if f.pub != nil {
			return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte(msg))
		}
		return nil
	}, nil
}