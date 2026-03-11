package combat

import "github.com/pixil98/go-mud/internal/assets"

// PerkReader can look up modifier perk values by key.
type PerkReader interface {
	ModifierValue(key string) int
}

// CalcDamage applies all perk-based damage modifiers to a raw damage value for a
// single damage type and returns the final damage dealt and any reflected damage.
//
// Pipeline: attacker type pct bonus → target % absorb → target flat absorb → reflect cap.
func CalcDamage(raw int, dmgType string, attacker, target PerkReader) (damage, reflected int) {
	damage = max(raw, 1)

	// Attacker's damage type percentage bonus (type-specific + all).
	dmgPct := attacker.ModifierValue(assets.DamageKey(dmgType, assets.DamageAspectPct)) +
		attacker.ModifierValue(assets.DamageKey(assets.DamageTypeAll, assets.DamageAspectPct))
	if dmgPct != 0 {
		if damage = damage * (100 + dmgPct) / 100; damage < 1 {
			damage = 1
		}
	}

	// Attacker's flat damage bonus (type-specific + all).
	damage += attacker.ModifierValue(assets.DamageKey(dmgType, assets.DamageAspectFlat)) +
		attacker.ModifierValue(assets.DamageKey(assets.DamageTypeAll, assets.DamageAspectFlat))
	if damage < 1 {
		damage = 1
	}

	// Percent absorption first (type-specific + all).
	absorbPct := target.ModifierValue(assets.DefenseKey(dmgType, assets.DefenseAspectAbsorbPct)) +
		target.ModifierValue(assets.DefenseKey(assets.DamageTypeAll,assets.DefenseAspectAbsorbPct))
	if absorbPct > 0 {
		if damage = damage * (100 - absorbPct) / 100; damage < 1 {
			damage = 1
		}
	}

	// Flat absorption (type-specific + all).
	absorb := target.ModifierValue(assets.DefenseKey(dmgType, assets.DefenseAspectAbsorb)) +
		target.ModifierValue(assets.DefenseKey(assets.DamageTypeAll,assets.DefenseAspectAbsorb))
	if damage -= absorb; damage < 1 {
		damage = 1
	}

	// Reflect: capped at actual damage taken.
	reflect := target.ModifierValue(assets.DefenseKey(dmgType, assets.DefenseAspectReflect)) +
		target.ModifierValue(assets.DefenseKey(assets.DamageTypeAll,assets.DefenseAspectReflect))
	reflected = min(reflect, damage)

	return damage, reflected
}
