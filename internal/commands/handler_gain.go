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
	return func(ctx context.Context, in *CommandInput) error {
		if in.Char.IsInCombat() {
			return NewUserError("You can't train while fighting!")
		}

		actor := in.Char.Character.Get()

		if actor.Level >= game.MaxLevel {
			return NewUserError("You have reached the maximum level.")
		}

		needed := game.ExpToNextLevel(actor.Level, actor.Experience)
		if needed > 0 {
			return NewUserError(fmt.Sprintf(
				"You need %d more experience to reach level %d.",
				needed, actor.Level+1))
		}

		in.Char.Gain()

		msg := fmt.Sprintf("Congratulations! You have advanced to level %d!", actor.Level)

		if f.pub != nil {
			return f.pub.Publish(game.SinglePlayer(in.Char.Character.Id()), nil, []byte(msg))
		}
		return nil
	}, nil
}
