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
	Name         string   `json:"name"`
	Abbreviation string   `json:"abbreviation"`
	Perks        []Perk   `json:"perks"`
	WearSlots    []string `json:"wear_slots,omitempty"`
}

// Validate checks that all perks on the race are valid.
func (r *Race) Validate() error {
	el := errors.NewErrorList()

	for i, p := range r.Perks {
		if err := p.validate(); err != nil {
			el.Add(fmt.Errorf("perks[%d]: %w", i, err))
		}
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

// Selector returns the race name for use in interactive selection prompts.
func (r *Race) Selector() string {
	return r.Name
}
