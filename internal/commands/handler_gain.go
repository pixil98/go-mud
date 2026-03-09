package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// GainActor provides the character state needed by the gain handler.
type GainActor interface {
	CommandActor
	IsInCombat() bool
	Asset() *assets.Character
	Gain()
}

var _ GainActor = (*game.CharacterInstance)(nil)

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

	msg := fmt.Sprintf("Congratulations! You have advanced to level %d!", actor.Level)

	if f.pub != nil {
		return f.pub.Publish(game.SinglePlayer(char.Id()), nil, []byte(msg))
	}
	return nil
}
