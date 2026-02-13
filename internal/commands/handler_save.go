package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// saveCharacter persists the character's current session state (location, inventory, etc.)
// to the character store. Shared by save and quit handlers.
func saveCharacter(world *game.WorldState, cmdCtx *CommandContext) error {
	zoneId, roomId := cmdCtx.Session.Location()
	cmdCtx.Actor.LastZone = zoneId
	cmdCtx.Actor.LastRoom = roomId

	return world.Characters().Save(strings.ToLower(cmdCtx.Actor.Name), cmdCtx.Actor)
}

// SaveHandlerFactory creates handlers that persist the player's character.
type SaveHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

func NewSaveHandlerFactory(world *game.WorldState, pub Publisher) *SaveHandlerFactory {
	return &SaveHandlerFactory{world: world, pub: pub}
}

func (f *SaveHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *SaveHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *SaveHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		if err := saveCharacter(f.world, cmdCtx); err != nil {
			return fmt.Errorf("saving character: %w", err)
		}

		if f.pub != nil {
			return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte("Character saved."))
		}

		return nil
	}, nil
}
