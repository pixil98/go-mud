package combat

import "github.com/pixil98/go-mud/internal/assets"

// CalcDamage applies all perk-based damage modifiers to a raw damage value for a
// single damage type and returns the final damage dealt and any reflected damage.
//
// Pipeline: attacker damage bonus → target absorb reduction → reflect cap.
func CalcDamage(raw int, dmgType string, attacker, target assets.PerkReader) (damage, reflected int) {
	// Attacker's damage bonus (pct then flat, type-specific + all).
	damage = assets.ApplyModifiers(max(raw, 1), 1, attacker,
		assets.BuildKey(assets.DamagePrefix, dmgType),
		assets.BuildKey(assets.DamagePrefix, assets.DamageTypeAll),
	)

	// Target's absorb reduction (pct then flat, type-specific + all).
	// Absorb subtracts, so we negate the modifiers: positive pct/flat = less damage taken.
	absorbTypePrefix := assets.BuildKey(assets.DefensePrefix, dmgType, assets.DefenseCategoryAbsorb)
	absorbAllPrefix := assets.BuildKey(assets.DefensePrefix, assets.DamageTypeAll, assets.DefenseCategoryAbsorb)

	absorbPct := target.ModifierValue(assets.BuildKey(absorbTypePrefix, assets.ModSuffixPct)) +
		target.ModifierValue(assets.BuildKey(absorbAllPrefix, assets.ModSuffixPct))
	if absorbPct > 0 {
		if damage = damage * (100 - absorbPct) / 100; damage < 1 {
			damage = 1
		}
	}

	absorbFlat := target.ModifierValue(assets.BuildKey(absorbTypePrefix, assets.ModSuffixFlat)) +
		target.ModifierValue(assets.BuildKey(absorbAllPrefix, assets.ModSuffixFlat))
	if damage -= absorbFlat; damage < 1 {
		damage = 1
	}

	// Reflect: pct then flat, capped at actual damage taken.
	reflectTypePrefix := assets.BuildKey(assets.DefensePrefix, dmgType, assets.DefenseCategoryReflect)
	reflectAllPrefix := assets.BuildKey(assets.DefensePrefix, assets.DamageTypeAll, assets.DefenseCategoryReflect)

	reflectPct := target.ModifierValue(assets.BuildKey(reflectTypePrefix, assets.ModSuffixPct)) +
		target.ModifierValue(assets.BuildKey(reflectAllPrefix, assets.ModSuffixPct))
	reflectFlat := target.ModifierValue(assets.BuildKey(reflectTypePrefix, assets.ModSuffixFlat)) +
		target.ModifierValue(assets.BuildKey(reflectAllPrefix, assets.ModSuffixFlat))
	reflected = reflectFlat
	if reflectPct > 0 {
		reflected += damage * reflectPct / 100
	}
	reflected = min(reflected, damage)

	return damage, reflected
}

// CalcThreat applies perk-based threat modifiers to a raw threat value.
// Returns the modified threat, floored at 0.
func CalcThreat(raw int, actor assets.PerkReader) int {
	return assets.ApplyModifiers(raw, 0, actor, assets.CombatThreatPrefix)
}
