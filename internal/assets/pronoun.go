package assets

import "fmt"

type pronounPossessive struct {
	Adjective string `json:"adjective"`
	Pronoun   string `json:"pronoun"`
}

// Pronoun defines a set of pronouns loaded from asset files.
type Pronoun struct {
	Subjective string            `json:"subjective"`
	Objective  string            `json:"objective"`
	Possessive pronounPossessive `json:"possessive"`
	Reflexive  string            `json:"reflexive"`
}

// Validate checks the pronoun fields (currently a no-op placeholder).
func (p *Pronoun) Validate() error {
	return nil
}

// Selector returns the subjective/objective pronoun pair as a display string.
func (p *Pronoun) Selector() string {
	return fmt.Sprintf("%s/%s", p.Subjective, p.Objective)
}
