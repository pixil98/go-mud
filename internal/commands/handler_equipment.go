package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// EquipmentActor provides the character state needed by the equipment handler.
type EquipmentActor interface {
	Id() string
	Publish(data []byte, exclude []string)
	Equipment() *game.Equipment
	WearSlots() []string
}

var _ EquipmentActor = (*game.CharacterInstance)(nil)

// EquipmentHandlerFactory creates handlers that list the player's equipped items.
type EquipmentHandlerFactory struct{}

// NewEquipmentHandlerFactory creates a handler factory for equipment listing commands.
func NewEquipmentHandlerFactory() *EquipmentHandlerFactory {
	return &EquipmentHandlerFactory{}
}

// Spec returns the handler's target and config requirements.
func (f *EquipmentHandlerFactory) Spec() *HandlerSpec {
	return nil
}

// ValidateConfig performs custom validation on the command config.
func (f *EquipmentHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *EquipmentHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[EquipmentActor](f.handle), nil
}

func (f *EquipmentHandlerFactory) handle(ctx context.Context, char EquipmentActor, in *CommandInput) error {
	// Build the slot list to display: race slots if available, otherwise equipped slots
	eq := char.Equipment()
	slots := char.WearSlots()
	if len(slots) == 0 && eq != nil {
		eq.ForEachSlot(func(item game.EquipSlot) {
			slots = append(slots, item.Slot)
		})
	}

	lines := []string{"You have equipped:"}
	lines = append(lines, FormatEquipmentSlots(eq, slots)...)

	char.Publish([]byte(strings.Join(lines, "\n")), nil)
	return nil
}

func formatSlotLine(slot string, desc string) string {
	return fmt.Sprintf("  <%s>\t%s", slot, desc)
}

// FormatEquipmentSlots returns indented lines for every slot in the list,
// showing the equipped item or "empty" for unoccupied slots.
func FormatEquipmentSlots(eq *game.Equipment, slots []string) []string {
	if len(slots) == 0 {
		return []string{"  Nothing"}
	}
	var lines []string
	slotSeen := make(map[string]int)
	for _, slot := range slots {
		slotSeen[slot]++
		desc := "empty"
		if eq != nil {
			count := 0
			eq.ForEachSlot(func(item game.EquipSlot) {
				if item.Slot == slot {
					count++
					if count == slotSeen[slot] {
						desc = item.Obj.Object.Get().ShortDesc
					}
				}
			})
		}
		lines = append(lines, formatSlotLine(slot, desc))
	}
	return lines
}

// FormatEquippedItems returns indented lines for occupied equipment slots only.
// Returns nil if nothing is equipped.
func FormatEquippedItems(eq *game.Equipment) []string {
	if eq == nil || eq.Len() == 0 {
		return nil
	}
	var lines []string
	eq.ForEachSlot(func(item game.EquipSlot) {
		lines = append(lines, formatSlotLine(item.Slot, item.Obj.Object.Get().ShortDesc))
	})
	return lines
}
