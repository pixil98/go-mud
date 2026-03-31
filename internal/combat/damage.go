package combat

import (
	"math/rand/v2"

	"github.com/pixil98/go-mud/internal/assets"
)

// RollAttack rolls a d20 and adds the attack modifier.
func RollAttack(attackMod int) int {
	return rand.IntN(20) + 1 + attackMod
}

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

var damageMessages = []struct {
	maxDamage int
	verb3rd   string // "{attacker} {verb} {target}!"
}{
	{0, "misses"},
	{2, "barely scratches"},
	{4, "tickles"},
	{6, "barely hurts"},
	{10, "hits"},
	{14, "strikes"},
	{19, "pummels"},
	{24, "thrashes"},
	{30, "mauls"},
	{40, "decimates"},
	{50, "devastates"},
	{65, "obliterates"},
	{80, "annihilates"},
}

// damageVerb returns the 3rd person verb for a damage amount.
// A damage of 0 returns "misses".
func damageVerb(damage int) string {
	for _, msg := range damageMessages {
		if damage <= msg.maxDamage {
			return msg.verb3rd
		}
	}
	return "does UNSPEAKABLE things to"
}

// HitMsgActor returns a 2nd-person message: "You pummel Goblin!" or "You miss Goblin!"
func HitMsgActor(targetName string, damage int) string {
	return "You " + damageVerb(damage) + " " + targetName + "!"
}

// HitMsgTarget returns a message for the target: "Player pummels you!" or "Player misses you!"
func HitMsgTarget(actorName string, damage int) string {
	return actorName + " " + damageVerb(damage) + " you!"
}

// HitMsgRoom returns a 3rd-person message: "Player pummels Goblin!" or "Player misses Goblin!"
func HitMsgRoom(actorName, targetName string, damage int) string {
	return actorName + " " + damageVerb(damage) + " " + targetName + "!"
}
