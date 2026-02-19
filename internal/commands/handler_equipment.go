package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// EquipmentHandlerFactory creates handlers that list the player's equipped items.
type EquipmentHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

func NewEquipmentHandlerFactory(world *game.WorldState, pub Publisher) *EquipmentHandlerFactory {
	return &EquipmentHandlerFactory{world: world, pub: pub}
}

func (f *EquipmentHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *EquipmentHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *EquipmentHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		eq := cmdCtx.Actor.Equipment

		// Build the slot list to display: race slots if available, otherwise equipped slots
		slots := cmdCtx.Actor.Race.WearSlots
		if len(slots) == 0 && eq != nil {
			for _, item := range eq.Items {
				slots = append(slots, item.Slot)
			}
		}

		lines := []string{"You are wearing:"}
		if len(slots) == 0 {
			lines = append(lines, "  Nothing")
		} else {
			slotSeen := make(map[string]int)
			for _, slot := range slots {
				slotSeen[slot]++
				desc := "empty"
				if eq != nil {
					count := 0
					for _, item := range eq.Items {
						if item.Slot == slot {
							count++
							if count == slotSeen[slot] {
								desc = item.Obj.Definition.ShortDesc
								break
							}
						}
					}
				}
				lines = append(lines, fmt.Sprintf("  [%s] %s", slot, desc))
			}
		}

		output := strings.Join(lines, "\n")
		if f.pub != nil {
			return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte(output))
		}

		return nil
	}, nil
}
