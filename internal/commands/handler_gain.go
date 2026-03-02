package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// GainHandlerFactory creates handlers for the gain (level up) command.
type GainHandlerFactory struct {
	pub game.Publisher
}

func NewGainHandlerFactory(pub game.Publisher) *GainHandlerFactory {
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
		if cmdCtx.Session.InCombat {
			return NewUserError("You can't train while fighting!")
		}

		if cmdCtx.Actor.Level >= game.MaxLevel {
			return NewUserError("You have reached the maximum level.")
		}

		needed := game.ExpToNextLevel(cmdCtx.Actor.Level, cmdCtx.Actor.Experience)
		if needed > 0 {
			return NewUserError(fmt.Sprintf(
				"You need %d more experience to reach level %d.",
				needed, cmdCtx.Actor.Level+1))
		}

		cmdCtx.Session.Gain()

		msg := fmt.Sprintf("Congratulations! You have advanced to level %d!", cmdCtx.Actor.Level)

		if f.pub != nil {
			return f.pub.Publish(game.SinglePlayer(cmdCtx.Session.Character.Id()), nil, []byte(msg))
		}
		return nil
	}, nil
}
