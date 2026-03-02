package assets

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// Mobile defines a type of mobile entity loaded from asset files.
// Multiple instances can be spawned from one definition.
// Mobile IDs follow the convention <zone>-<name> (e.g., "millbrook-guard").
type Mobile struct {
	// Aliases are keywords players can use to target this mobile (e.g., ["guard", "town"])
	Aliases []string `json:"aliases"`

	// ShortDesc is used in action messages (e.g., "The town guard hits you.")
	ShortDesc string `json:"short_desc"`

	// LongDesc is shown when the mobile is in its default position in a room
	// (e.g., "A burly guard in chain mail keeps watch over the square.")
	LongDesc string `json:"long_desc"`

	// DetailedDesc is shown when a player looks at the mobile
	DetailedDesc string `json:"detailed_desc"`

	// Inventory is the mobile's starting inventory
	Inventory []ObjectSpawn `json:"inventory,omitempty"`

	// Equipment is the mobile's starting equipment
	Equipment map[string]ObjectSpawn `json:"equipment,omitempty"`

	Level int `json:"level,omitempty"`

	// Combat template values used to initialize MobileInstance CombatStats on spawn.
	MaxHP       int `json:"max_hp,omitempty"`
	AC          int `json:"ac,omitempty"`
	AttackMod   int `json:"attack_mod,omitempty"`
	DamageDice  int `json:"damage_dice,omitempty"`
	DamageSides int `json:"damage_sides,omitempty"`
	DamageMod   int `json:"damage_mod,omitempty"`

	// ExpReward overrides the base XP awarded when this mobile is killed.
	// If 0, base XP is calculated from the mobile's level.
	ExpReward int `json:"exp_reward,omitempty"`
}

// MatchName returns true if name matches any of this mobile's aliases (case-insensitive).
func (m *Mobile) MatchName(name string) bool {
	nameLower := strings.ToLower(name)
	for _, alias := range m.Aliases {
		if strings.ToLower(alias) == nameLower {
			return true
		}
	}
	return false
}

// Validate returns an error if the mobile definition is invalid.
func (m *Mobile) Validate() error {
	el := errors.NewErrorList()
	if len(m.Aliases) < 1 {
		el.Add(fmt.Errorf("mobile alias is required"))
	}
	if m.ShortDesc == "" {
		el.Add(fmt.Errorf("mobile short description is required"))
	}
	if m.MaxHP <= 0 {
		el.Add(fmt.Errorf("mobile max_hp is required and must be positive"))
	}
	return el.Err()
}

// Resolve resolves foreign key references on the mobile definition.
func (m *Mobile) Resolve(objs storage.Storer[*Object]) error {
	el := errors.NewErrorList()
	for i := range m.Inventory {
		el.Add(m.Inventory[i].Resolve(objs))
	}
	for k := range m.Equipment {
		eq := m.Equipment[k]
		el.Add(eq.Resolve(objs))
		m.Equipment[k] = eq
	}
	return el.Err()
}
