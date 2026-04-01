package assets

import (
	"fmt"

	"github.com/pixil98/go-errors"
)

// Race defines a playable race loaded from asset files.
type Race struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	Perks        []Perk `json:"perks"`
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

// Selector returns the race name for use in interactive selection prompts.
func (r *Race) Selector() string {
	return r.Name
}
