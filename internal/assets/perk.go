package assets

import "fmt"

// Perk describes an effect granted when a node or spine node is unlocked, or
// by a race. Which fields are meaningful depends on Type.
type Perk struct {
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`    // unlock_ability: the ability id to grant
	Stat  string `json:"stat,omitempty"`  // stat_mod: the stat key to modify
	Value int    `json:"value,omitempty"` // stat_mod: amount to add per rank
}

func (p *Perk) validate() error {
	if p.Type == "" {
		return fmt.Errorf("type is required")
	}

	switch p.Type {
	case "unlock_ability":
		if p.ID == "" {
			return fmt.Errorf("unlock_ability perk requires id")
		}
	case "stat_mod":
		if p.Stat == "" {
			return fmt.Errorf("stat_mod perk requires stat")
		}
	}

	return nil
}
