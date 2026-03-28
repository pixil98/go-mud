package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// WhoHandlerFactory creates handlers that list online players.
type WhoHandlerFactory struct {
	players game.PlayerGroup
	pub     game.Publisher
}

// NewWhoHandlerFactory creates a new WhoHandlerFactory.
func NewWhoHandlerFactory(players game.PlayerGroup, pub game.Publisher) *WhoHandlerFactory {
	return &WhoHandlerFactory{players: players, pub: pub}
}

// Spec returns the handler's target and config requirements.
func (f *WhoHandlerFactory) Spec() *HandlerSpec {
	return nil
}

// ValidateConfig performs custom validation on the command config.
func (f *WhoHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *WhoHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, in *CommandInput) error {
		var lines []string

		f.players.ForEachPlayer(func(charId string, state *game.CharacterInstance) {
			if state.IsLinkless() {
				return
			}
			char := state.Character.Get()
			parts := []string{char.Race.Get().Abbreviation}
			parts = append(parts, strconv.Itoa(char.Level))
			bracket := strings.Join(parts, " ")

			lines = append(lines, fmt.Sprintf("[%s] %s %s", bracket, char.Name, char.Title))
		})

		output := "Players Online:\n" + strings.Join(lines, "\n")
		if f.pub != nil {
			return f.pub.Publish(game.SinglePlayer(in.Actor.Id()), nil, []byte(output))
		}

		return nil
	}, nil
}
