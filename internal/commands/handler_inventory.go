package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// InventoryHandlerFactory creates handlers that list the player's inventory.
type InventoryHandlerFactory struct {
	pub Publisher
}

// NewInventoryHandlerFactory creates a new InventoryHandlerFactory.
func NewInventoryHandlerFactory(pub Publisher) *InventoryHandlerFactory {
	return &InventoryHandlerFactory{pub: pub}
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
		lines = append(lines, FormatInventoryItems(cmdCtx.Actor.Inventory)...)

		output := strings.Join(lines, "\n")
		if f.pub != nil {
			return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte(output))
		}

		return nil
	}, nil
}

// FormatInventoryItems returns indented lines describing items in an inventory.
// Returns ["  Nothing"] if the inventory is nil or empty.
func FormatInventoryItems(inv *game.Inventory) []string {
	if inv == nil || len(inv.Objs) == 0 {
		return []string{"  Nothing"}
	}
	var lines []string
	for _, oi := range inv.Objs {
		lines = append(lines, fmt.Sprintf("  %s", oi.Object.Id().ShortDesc))
	}
	if len(lines) == 0 {
		return []string{"  Nothing"}
	}
	return lines
}
