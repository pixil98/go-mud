package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// EquipmentHandlerFactory creates handlers that list the player's equipped items.
type EquipmentHandlerFactory struct {
	pub Publisher
}

func NewEquipmentHandlerFactory(pub Publisher) *EquipmentHandlerFactory {
	return &EquipmentHandlerFactory{pub: pub}
}

func (f *EquipmentHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *EquipmentHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *EquipmentHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		// Build the slot list to display: race slots if available, otherwise equipped slots
		slots := cmdCtx.Actor.Race.Id().WearSlots
		if len(slots) == 0 && cmdCtx.Actor.Equipment != nil {
			for _, item := range cmdCtx.Actor.Equipment.Objs {
				slots = append(slots, item.Slot)
			}
		}

		lines := []string{"You are wearing:"}
		lines = append(lines, FormatEquipmentSlots(cmdCtx.Actor.Equipment, slots)...)

		output := strings.Join(lines, "\n")
		if f.pub != nil {
			return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte(output))
		}

		return nil
	}, nil
}

func formatSlotLine(slot string, desc string) string {
	return fmt.Sprintf("  [%s] %s", slot, desc)
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
			for _, item := range eq.Objs {
				if item.Slot == slot {
					count++
					if count == slotSeen[slot] {
						desc = item.Obj.Object.Id().ShortDesc
						break
					}
				}
			}
		}
		lines = append(lines, formatSlotLine(slot, desc))
	}
	return lines
}

// FormatEquippedItems returns indented lines for occupied equipment slots only.
// Returns nil if nothing is equipped.
func FormatEquippedItems(eq *game.Equipment) []string {
	if eq == nil || len(eq.Objs) == 0 {
		return nil
	}
	var lines []string
	for _, item := range eq.Objs {
		lines = append(lines, formatSlotLine(item.Slot, item.Obj.Object.Id().ShortDesc))
	}
	return lines
}
