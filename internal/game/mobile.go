package game

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

	Actor
}

// StatSections returns the mobile's stat display sections.
func (m *Mobile) StatSections() []StatSection {
	sections := m.Actor.statSections()
	sections[0].Lines = append([]StatLine{{Value: m.ShortDesc, Center: true}}, sections[0].Lines...)
	return sections
}

// Resolve resolves foreign keys from the dictionary.
func (m *Mobile) Resolve(dict *Dictionary) error {
	el := errors.NewErrorList()
	for i := range m.Inventory {
		el.Add(m.Inventory[i].Resolve(dict.Objects))
	}
	for k := range m.Equipment {
		eq := m.Equipment[k]
		el.Add(eq.Resolve(dict.Objects))
		m.Equipment[k] = eq
	}
	el.Add(m.Actor.Resolve(dict))
	return el.Err()
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

// Validate satisfies storage.ValidatingSpec
func (m *Mobile) Validate() error {
	el := errors.NewErrorList()
	if len(m.Aliases) < 1 {
		el.Add(fmt.Errorf("mobile alias is required"))
	}
	if m.ShortDesc == "" {
		el.Add(fmt.Errorf("mobile short description is required"))
	}
	return el.Err()
}

// MobileInstance represents a single spawned instance of a Mobile definition.
// Location is tracked by the containing structure (room map).
type MobileInstance struct {
	InstanceId string
	Mobile     storage.SmartIdentifier[*Mobile]

	ActorInstance
}
