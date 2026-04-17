package assets

// Race defines a playable race loaded from asset files.
type Race struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	Perks        []Perk `json:"perks"`
}

// Validate checks that all perks on the race are valid.
func (r *Race) Validate() error {
	return validatePerks(r.Perks)
}

// Selector returns the race name for use in interactive selection prompts.
func (r *Race) Selector() string {
	return r.Name
}
