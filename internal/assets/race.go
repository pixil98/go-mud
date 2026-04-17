package assets

import (
	"errors"
	"fmt"
)

// Race defines a playable race loaded from asset files.
type Race struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	Perks        []Perk `json:"perks"`
}

// Validate checks that all perks on the race are valid.
func (r *Race) Validate() error {
	var errs []error

	for i, p := range r.Perks {
		if err := p.validate(); err != nil {
			errs = append(errs, fmt.Errorf("perks[%d]: %w", i, err))
		}
	}

	return errors.Join(errs...)
}

// Selector returns the race name for use in interactive selection prompts.
func (r *Race) Selector() string {
	return r.Name
}
