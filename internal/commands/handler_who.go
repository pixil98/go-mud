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

func (f *WhoHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *WhoHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *WhoHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		var lines []string

		f.players.ForEachPlayer(func(charId string, state *game.PlayerState) {
			if state.Linkless {
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
			return f.pub.Publish(game.SinglePlayer(cmdCtx.Session.Character.Id()), nil, []byte(output))
		}

		return nil
	}, nil
}
