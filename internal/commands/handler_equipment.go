package commands

import (
	"context"
	"fmt"
	"sort"
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
		lines := []string{"You are wearing:"}

		if cmdCtx.Actor.Equipment == nil || len(cmdCtx.Actor.Equipment.Slots) == 0 {
			lines = append(lines, "  Nothing")
		} else {
			// Sort slots for consistent output
			slots := make([]string, 0, len(cmdCtx.Actor.Equipment.Slots))
			for slot := range cmdCtx.Actor.Equipment.Slots {
				slots = append(slots, slot)
			}
			sort.Strings(slots)

			for _, slot := range slots {
				oi := cmdCtx.Actor.Equipment.Slots[slot]
				obj := f.world.Objects().Get(string(oi.ObjectId))
				if obj == nil {
					continue
				}
				lines = append(lines, fmt.Sprintf("  [%s] %s", slot, obj.ShortDesc))
			}
		}

		output := strings.Join(lines, "\n")
		if f.pub != nil {
			return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte(output))
		}

		return nil
	}, nil
}
