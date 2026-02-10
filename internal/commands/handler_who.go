package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/plugins"
	"github.com/pixil98/go-mud/internal/storage"
)

// WhoHandlerFactory creates handlers that list online players.
type WhoHandlerFactory struct {
	world    *game.WorldState
	pub      Publisher
	charInfo plugins.CharacterInfoProvider
}

// NewWhoHandlerFactory creates a new WhoHandlerFactory.
func NewWhoHandlerFactory(world *game.WorldState, pub Publisher, charInfo plugins.CharacterInfoProvider) *WhoHandlerFactory {
	return &WhoHandlerFactory{world: world, pub: pub, charInfo: charInfo}
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

		f.world.ForEachPlayer(func(charId storage.Identifier, state game.PlayerState) {
			char := f.world.Characters().Get(string(charId))
			if char == nil {
				return
			}

			info := f.charInfo.GetCharacterInfo(char, plugins.InfoStyleShort)

			// Build bracket content from plugin info
			var parts []string
			for _, v := range info {
				parts = append(parts, v)
			}
			bracket := strings.Join(parts, " ")

			lines = append(lines, fmt.Sprintf("[%s] %s %s", bracket, char.Name, char.Title))
		})

		output := "Players Online:\n" + strings.Join(lines, "\n")
		playerChannel := fmt.Sprintf("player-%s", strings.ToLower(cmdCtx.Actor.Name))
		if f.pub != nil {
			_ = f.pub.Publish(playerChannel, []byte(output))
		}

		return nil
	}, nil
}
