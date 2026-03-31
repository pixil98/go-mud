package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// InventoryActor provides the character state needed by the inventory handler.
type InventoryActor interface {
	Id() string
	Notify(msg string)
	GetInventory() *game.Inventory
}

var _ InventoryActor = (*game.CharacterInstance)(nil)

// InventoryHandlerFactory creates handlers that list the player's inventory.
type InventoryHandlerFactory struct{}

// NewInventoryHandlerFactory creates a new InventoryHandlerFactory.
func NewInventoryHandlerFactory() *InventoryHandlerFactory {
	return &InventoryHandlerFactory{}
}

// Spec returns the handler's target and config requirements.
func (f *InventoryHandlerFactory) Spec() *HandlerSpec {
	return nil
}

// ValidateConfig performs custom validation on the command config.
func (f *InventoryHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *InventoryHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[InventoryActor](f.handle), nil
}

func (f *InventoryHandlerFactory) handle(ctx context.Context, char InventoryActor, in *CommandInput) error {
	lines := []string{"You are carrying:"}
	lines = append(lines, FormatInventoryItems(char.GetInventory())...)

	char.Notify(strings.Join(lines, "\n"))
	return nil
}

// FormatInventoryItems returns indented lines describing items in an inventory.
// Returns ["  Nothing"] if the inventory is nil or empty.
func FormatInventoryItems(inv *game.Inventory) []string {
	if inv == nil || inv.Len() == 0 {
		return []string{"  Nothing"}
	}
	var lines []string
	inv.ForEachObj(func(_ string, oi *game.ObjectInstance) {
		lines = append(lines, fmt.Sprintf("  %s", oi.Object.Get().ShortDesc))
	})
	if len(lines) == 0 {
		return []string{"  Nothing"}
	}
	return lines
}
