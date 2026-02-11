package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// InventoryHandlerFactory creates handlers that list the player's inventory.
type InventoryHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

// NewInventoryHandlerFactory creates a new InventoryHandlerFactory.
func NewInventoryHandlerFactory(world *game.WorldState, pub Publisher) *InventoryHandlerFactory {
	return &InventoryHandlerFactory{world: world, pub: pub}
}

func (f *InventoryHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *InventoryHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *InventoryHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		lines := []string{"You are carrying:"}

		if cmdCtx.Actor.Inventory == nil || len(cmdCtx.Actor.Inventory.Items) == 0 {
			lines = append(lines, "  Nothing")
		} else {
			for _, oi := range cmdCtx.Actor.Inventory.Items {
				obj := f.world.Objects().Get(string(oi.ObjectId))
				if obj == nil {
					continue
				}
				lines = append(lines, fmt.Sprintf("  %s", obj.ShortDesc))
			}
		}

		output := strings.Join(lines, "\n")
		if f.pub != nil {
			return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte(output))
		}

		return nil
	}, nil
}
