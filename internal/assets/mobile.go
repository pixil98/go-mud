package assets

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// ---------------------------------------------------------------------------
// Mobile flags
// ---------------------------------------------------------------------------

// MobileFlag defines a boolean behavior property of a mobile.
type MobileFlag int

// MobileFlag values.
const (
	MobileFlagUnknown    MobileFlag = iota
	MobileFlagSentinel               // Doesn't wander from spawn room
	MobileFlagAggressive             // Attacks players on sight
	MobileFlagWimpy                  // Flees at low HP
	MobileFlagHelper                 // Assists other mobs being attacked in same room
	MobileFlagStayZone               // Won't wander outside its zone
	MobileFlagScavenger              // Picks up valuables from the ground
	MobileFlagMemory                 // Remembers and retaliates against attackers
	MobileFlagAware                  // Cannot be backstabbed
)

func parseMobileFlag(s string) MobileFlag {
	switch strings.ToLower(s) {
	case "sentinel":
		return MobileFlagSentinel
	case "aggressive":
		return MobileFlagAggressive
	case "wimpy":
		return MobileFlagWimpy
	case "helper":
		return MobileFlagHelper
	case "stay_zone":
		return MobileFlagStayZone
	case "scavenger":
		return MobileFlagScavenger
	case "memory":
		return MobileFlagMemory
	case "aware":
		return MobileFlagAware
	default:
		return MobileFlagUnknown
	}
}

// ---------------------------------------------------------------------------
// Mobile
// ---------------------------------------------------------------------------

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
	Equipment []EquipmentSpawn `json:"equipment,omitempty"`

	Level int `json:"level,omitempty"`

	// Perks define the mobile's resources, combat stats, and other perk-driven values.
	Perks []Perk `json:"perks,omitempty"`

	// Flags are boolean behavior properties (e.g., "sentinel", "aggressive").
	Flags []string `json:"flags,omitempty"`

	// ExpReward overrides the base XP awarded when this mobile is killed.
	// If 0, base XP is calculated from the mobile's level.
	ExpReward int `json:"exp_reward,omitempty"`
}

// HasFlag returns true if the mobile has the given flag.
func (m *Mobile) HasFlag(flag MobileFlag) bool {
	for _, f := range m.Flags {
		if parseMobileFlag(f) == flag {
			return true
		}
	}
	return false
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
	for i := range m.Perks {
		if err := m.Perks[i].validate(); err != nil {
			el.Add(fmt.Errorf("perk[%d]: %w", i, err))
		}
	}
	for _, f := range m.Flags {
		if parseMobileFlag(f) == MobileFlagUnknown {
			el.Add(fmt.Errorf("unknown flag %q", f))
		}
	}
	return el.Err()
}

// Resolve resolves foreign key references on the mobile definition.
func (m *Mobile) Resolve(objs storage.Storer[*Object]) error {
	el := errors.NewErrorList()
	for i := range m.Inventory {
		el.Add(m.Inventory[i].Resolve(objs))
	}
	for i := range m.Equipment {
		el.Add(m.Equipment[i].Resolve(objs))
	}
	return el.Err()
}
