package assets

import (
	"fmt"

	"github.com/pixil98/go-errors"
)

// Race defines a playable race loaded from asset files.
// WearSlots lists the equipment slot types available to this race. Duplicate
// entries indicate multiple slots of the same type (e.g., two "finger" entries
// means two ring slots). The list order defines the display order for the
// equipment command.
type Race struct {
	Name         string         `json:"name"`
	Abbreviation string         `json:"abbreviation"`
	StatMods     map[StatKey]int `json:"stat_mods"`
	Perks        []string       `json:"perks"`
	WearSlots    []string       `json:"wear_slots,omitempty"`
}

func (r *Race) Validate() error {
	el := errors.NewErrorList()

	for _, p := range r.Perks {
		el.Add(func() error {
			switch p {
			case "darkvision":
				return nil
			default:
				return fmt.Errorf("unknown perk: %s", p)
			}
		}())
	}

	return el.Err()
}

// SlotCount returns how many slots of the given type this race has.
func (r *Race) SlotCount(slot string) int {
	count := 0
	for _, s := range r.WearSlots {
		if s == slot {
			count++
		}
	}
	return count
}

func (r *Race) Selector() string {
	return r.Name
}
