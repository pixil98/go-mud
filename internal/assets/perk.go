package assets

import (
	"errors"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Perk types
// ---------------------------------------------------------------------------

const (
	PerkTypeModifier = "modifier"
	PerkTypeGrant    = "grant"
)

// ---------------------------------------------------------------------------
// Modifier suffixes (shared across all key builders that use flat/pct scaling)
// ---------------------------------------------------------------------------

const (
	ModSuffixFlat = "flat" // flat additive bonus
	ModSuffixPct  = "pct"  // percent scaling bonus
)

// ---------------------------------------------------------------------------
// Stat keys — core.stats.<stat>
// Individual keys with no flat/pct split.
// ---------------------------------------------------------------------------

// PerkKey identifies a well-known key for modifier perks. Asset-defined keys
// (e.g. "evocation.fire.damage_pct") don't need consts — these are provided
// for engine-interpreted values so they're discoverable and consistent.
type PerkKey = string

const (
	PerkKeySTR PerkKey = "core.stats.str"
	PerkKeyDEX PerkKey = "core.stats.dex"
	PerkKeyCON PerkKey = "core.stats.con"
	PerkKeyINT PerkKey = "core.stats.int"
	PerkKeyWIS PerkKey = "core.stats.wis"
	PerkKeyCHA PerkKey = "core.stats.cha"
)

// StatPerkKeys maps stat-related PerkKey consts to their corresponding StatKey.
var StatPerkKeys = map[PerkKey]StatKey{
	PerkKeySTR: StatSTR,
	PerkKeyDEX: StatDEX,
	PerkKeyCON: StatCON,
	PerkKeyINT: StatINT,
	PerkKeyWIS: StatWIS,
	PerkKeyCHA: StatCHA,
}

// ---------------------------------------------------------------------------
// Action points — core.action_points.max (individual key, no flat/pct)
// ---------------------------------------------------------------------------

const PerkKeyActionPointsMax PerkKey = "core.action_points.max"

// ---------------------------------------------------------------------------
// Combat prefixes — core.combat.<property>.<suffix>
// All combat modifiers use the flat/pct pattern via ApplyModifiers.
// ---------------------------------------------------------------------------

const (
	CombatACPrefix     = "core.combat.ac"     // armor class
	CombatAttackPrefix = "core.combat.attack" // attack roll bonus
	CombatThreatPrefix = "core.combat.threat" // threat generation scaling
)

// ---------------------------------------------------------------------------
// Key builder — joins parts with "." to form perk keys
// ---------------------------------------------------------------------------

// BuildKey joins the parts with "." to form a perk key.
// (e.g. BuildKey(DamagePrefix, "fire", ModSuffixPct) -> "core.damage.fire.pct").
func BuildKey(parts ...string) string {
	return strings.Join(parts, ".")
}

// ---------------------------------------------------------------------------
// Damage keys — core.damage.<type>.<suffix>
// ---------------------------------------------------------------------------

const DamagePrefix = "core.damage"

// Well-known damage type strings.
const (
	DamageTypeUntyped = "untyped"
	DamageTypeAll     = "all"
)

// ---------------------------------------------------------------------------
// Defense keys — core.defense.<type>.<category>.<suffix>
// ---------------------------------------------------------------------------

const DefensePrefix = "core.defense"

// Defense category constants.
const (
	DefenseCategoryAbsorb  = "absorb"  // damage reduction after a hit lands
	DefenseCategoryReflect = "reflect" // damage reflected back to the attacker
)

// ---------------------------------------------------------------------------
// Resource keys — core.resource.<name>.<aspect>
// ---------------------------------------------------------------------------

const ResourceHp = "hp"

const ResourcePrefix = "core.resource"

// Resource aspect constants.
const (
	ResourceAspectMax      = "max"
	ResourceAspectPerLevel = "per_level"
	ResourceAspectRegen    = "regen"
)

// ---------------------------------------------------------------------------
// Grant keys
// ---------------------------------------------------------------------------

const (
	// PerkGrantUnlockAbility grants access to an ability. Arg is the ability id.
	PerkGrantUnlockAbility = "unlock_ability"
	// PerkGrantAttack grants an attack with the given dice expression (e.g. "2d6").
	PerkGrantAttack = "attack"
	// PerkGrantAutoUse enables automatic ability use each combat tick.
	// Arg format: "ability_id:cooldown_ticks" (e.g. "attack:1", "fireball:3").
	PerkGrantAutoUse = "auto_use"
	// PerkGrantPeaceful prevents the holder from initiating combat.
	PerkGrantPeaceful = "peaceful"
	// PerkGrantWearSlot grants one equipment slot of the type given in Arg
	// (e.g. "head", "finger"). Duplicate grants of the same type create
	// multiple slots (e.g. two "finger" grants = two ring slots).
	PerkGrantWearSlot = "wear_slot"
	// PerkGrantDark makes the holder unable to see as if in a dark room.
	// When granted by a room, all occupants are affected unless they have
	// a countering darkvision effect.
	PerkGrantDark = "dark"
	// PerkGrantDarkvision lets the holder see in dark rooms. Granted by
	// racial traits, spells, or equipment like torches and lanterns.
	PerkGrantDarkvision = "darkvision"
	// PerkGrantNoMagic prevents the holder from casting spells.
	PerkGrantNoMagic = "nomagic"
)

// ---------------------------------------------------------------------------
// CircleMUD affection grants — effects from spells, items, or mob definitions
// ---------------------------------------------------------------------------

const (
	PerkGrantInvisible   = "invisible"
	PerkGrantDetectInvis = "detect_invis"
	PerkGrantSenseLife   = "sense_life"
	PerkGrantWaterwalk   = "waterwalk"
	PerkGrantSneak       = "sneak"
	PerkGrantHide        = "hide"
	PerkGrantNoCharm     = "nocharm"
	PerkGrantNoSummon    = "nosummon"
	PerkGrantNoSleep     = "nosleep"
	PerkGrantNoBash      = "nobash"
	PerkGrantNoBlind     = "noblind"
	PerkGrantProtectEvil = "protect_evil"
	PerkGrantProtectGood = "protect_good"
	PerkGrantNoTrack     = "notrack"
)

// ---------------------------------------------------------------------------
// Perk struct
// ---------------------------------------------------------------------------

// Perk describes an effect granted by a race, tree node, or equipped object.
// Which fields are meaningful depends on Type.
//
// modifier perks use Key and Value; their values are summed across all sources.
// grant perks use Key and an optional Arg string for parameterized effects.
type Perk struct {
	Type  string `json:"type"`
	Key   string `json:"key,omitempty"`   // modifier: the key to modify; grant: the keyword to grant
	Value int    `json:"value,omitempty"` // modifier: amount to add per rank
	Arg   string `json:"arg,omitempty"`   // grant: optional argument (e.g. ability id, dice expression)
}

// validatePerks runs Perk.validate on each element and joins any errors, prefixing
// each with its index. Shared by the various asset types that embed perk lists.
func validatePerks(perks []Perk) error {
	var errs []error
	for i := range perks {
		if err := perks[i].validate(); err != nil {
			errs = append(errs, fmt.Errorf("perks[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

// Validate checks a perk's fields for correctness.
func (p *Perk) validate() error {
	if p.Type == "" {
		return errors.New("type is required")
	}
	switch p.Type {
	case PerkTypeModifier:
		if p.Key == "" {
			return errors.New("modifier perk requires key")
		}
		if p.Value == 0 {
			return errors.New("modifier perk requires non-zero value")
		}
	case PerkTypeGrant:
		if p.Key == "" {
			return errors.New("grant perk requires key")
		}
	default:
		return fmt.Errorf("unknown perk type: %q", p.Type)
	}
	return nil
}

// ---------------------------------------------------------------------------
// PerkReader + ApplyModifiers — shared modifier evaluation
// ---------------------------------------------------------------------------

// PerkReader can look up modifier perk values by key.
type PerkReader interface {
	ModifierValue(key string) int
}

// ApplyModifiers applies flat and percent perk modifiers to a raw value.
// It looks up prefix+ModSuffixPct and prefix+ModSuffixFlat from the reader,
// applies percent scaling first, then adds the flat bonus.
// The result is floored at minVal.
//
// Multiple prefixes can be provided; their flat and pct values are summed
// before applying (e.g. type-specific + "all" prefixes for damage).
func ApplyModifiers(raw, minVal int, reader PerkReader, prefixes ...string) int {
	var totalFlat, totalPct int
	for _, prefix := range prefixes {
		totalPct += reader.ModifierValue(BuildKey(prefix, ModSuffixPct))
		totalFlat += reader.ModifierValue(BuildKey(prefix, ModSuffixFlat))
	}

	result := raw
	if totalPct != 0 {
		result = result * (100 + totalPct) / 100
	}
	result += totalFlat

	if result < minVal {
		result = minVal
	}
	return result
}
