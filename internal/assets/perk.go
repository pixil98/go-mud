package assets

import "fmt"

// PerkKey identifies a well-known key for modifier perks. Asset-defined keys
// (e.g. "evocation.fire.damage_pct") don't need consts — these are provided
// for engine-interpreted values so they're discoverable and consistent.
type PerkKey = string

// PerkKey constants for engine-interpreted modifier and grant keys.
const (
	PerkKeySTR             PerkKey = "core.stats.str"
	PerkKeyDEX             PerkKey = "core.stats.dex"
	PerkKeyCON             PerkKey = "core.stats.con"
	PerkKeyINT             PerkKey = "core.stats.int"
	PerkKeyWIS             PerkKey = "core.stats.wis"
	PerkKeyCHA             PerkKey = "core.stats.cha"
	PerkKeyActionPointsMax PerkKey = "core.action_points.max"
	PerkKeyCombatAC        PerkKey = "core.combat.ac"
	PerkKeyCombatAttackMod PerkKey = "core.combat.attack_mod"
	PerkKeyCombatThreatMod PerkKey = "core.combat.threat_mod"
)

// ResourceHp is the well-known resource name for hit points.
const ResourceHp = "hp"

// ResourceKeyPrefix is the perk key prefix for resource modifiers.
// Resource perk keys follow: core.resource.<name>.<aspect>
const ResourceKeyPrefix = "core.resource."

// ResourceAspect identifies a specific aspect of a perk-driven resource pool.
type ResourceAspect string

// ResourceAspect constants.
const (
	ResourceAspectMax      ResourceAspect = "max"
	ResourceAspectPerLevel ResourceAspect = "per_level"
	ResourceAspectRegen    ResourceAspect = "regen"
)

// ResourceKey builds a perk key for a resource aspect
// (e.g. ResourceKey("hp", ResourceAspectMax) -> "core.resource.hp.max").
func ResourceKey(resource string, aspect ResourceAspect) string {
	return ResourceKeyPrefix + resource + "." + string(aspect)
}

// StatPerkKeys maps stat-related PerkKey consts to their corresponding StatKey.
// Use this to extract ability score modifiers from modifier perks without
// hardcoding the key strings at the call site.
var StatPerkKeys = map[PerkKey]StatKey{
	PerkKeySTR: StatSTR,
	PerkKeyDEX: StatDEX,
	PerkKeyCON: StatCON,
	PerkKeyINT: StatINT,
	PerkKeyWIS: StatWIS,
	PerkKeyCHA: StatCHA,
}

// DamageKeyPrefix is the perk key prefix for damage type modifiers.
// Damage perk keys follow: core.damage.<type>.<aspect>
const DamageKeyPrefix = "core.damage."

// Well-known damage type strings.
const (
	// DamageTypeUntyped is the default damage type when no type is configured.
	DamageTypeUntyped = "untyped"
	// DamageTypeAll is used in defense keys to apply mitigation to all damage
	// regardless of type (e.g. DefenseKey(DamageTypeAll, DefenseAspectAbsorb)).
	DamageTypeAll = "all"
)

// DamageAspect identifies a specific aspect of damage type scaling.
type DamageAspect string

// DamageAspect constants.
const (
	DamageAspectPct  DamageAspect = "pct"  // percent damage bonus
	DamageAspectFlat DamageAspect = "flat" // flat damage bonus
)

// DamageKey builds a perk key for a damage type aspect
// (e.g. DamageKey("fire", DamageAspectPct) -> "core.damage.fire.pct").
func DamageKey(damageType string, aspect DamageAspect) string {
	return DamageKeyPrefix + damageType + "." + string(aspect)
}

// DefenseKeyPrefix is the perk key prefix for defense type modifiers.
// Defense perk keys follow: core.defense.<type>.<aspect>
// Use DamageTypeAll as the type to apply to all incoming damage regardless of type.
const DefenseKeyPrefix = "core.defense."

// DefenseAspect identifies a specific aspect of damage mitigation.
type DefenseAspect string

// DefenseAspect constants.
const (
	DefenseAspectAbsorb    DefenseAspect = "absorb"     // flat damage reduction after a hit lands
	DefenseAspectAbsorbPct DefenseAspect = "absorb_pct" // percent damage reduction after a hit lands
	DefenseAspectReflect   DefenseAspect = "reflect"    // flat damage reflected back to the attacker
)

// DefenseKey builds a perk key for a defense type aspect
// (e.g. DefenseKey("fire", DefenseAspectAbsorb) -> "core.defense.fire.absorb",
// DefenseKey("all", DefenseAspectAbsorb) -> "core.defense.all.absorb").
func DefenseKey(damageType string, aspect DefenseAspect) string {
	return DefenseKeyPrefix + damageType + "." + string(aspect)
}

// Perk type constants.
const (
	PerkTypeModifier = "modifier"
	PerkTypeGrant    = "grant"
)

// Well-known grant keys for engine-interpreted grant perks.
const (
	// PerkGrantUnlockAbility grants access to an ability. Arg is the ability id.
	PerkGrantUnlockAbility = "unlock_ability"
	// PerkGrantAttack grants an attack with the given dice expression (e.g. "2d6").
	PerkGrantAttack = "attack"
	// PerkGrantAutoUse enables automatic ability use each combat tick.
	// Arg format: "ability_id:cooldown_ticks" (e.g. "attack:1", "fireball:3").
	PerkGrantAutoUse = "auto_use"
	// PerkGrantPeaceful prevents the holder from initiating combat. Can be granted
	// by a room, zone, world, or equipped item.
	PerkGrantPeaceful = "peaceful"
)

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

func (p *Perk) validate() error {
	if p.Type == "" {
		return fmt.Errorf("type is required")
	}
	switch p.Type {
	case PerkTypeModifier:
		if p.Key == "" {
			return fmt.Errorf("modifier perk requires key")
		}
		if p.Value == 0 {
			return fmt.Errorf("modifier perk requires non-zero value")
		}
	case PerkTypeGrant:
		if p.Key == "" {
			return fmt.Errorf("grant perk requires key")
		}
	default:
		return fmt.Errorf("unknown perk type: %q", p.Type)
	}
	return nil
}
