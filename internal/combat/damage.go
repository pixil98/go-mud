package combat

import (
	"math/rand/v2"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
)

// ParseAttackArg extracts the damage type and dice expression from an attack grant arg.
// Supports "<type>:<dice>" (e.g. "fire:2d6+3") or plain "<dice>" (defaults to "untyped").
func ParseAttackArg(arg string) (dmgType, diceExpr string) {
	if i := strings.IndexByte(arg, ':'); i >= 0 {
		return arg[:i], arg[i+1:]
	}
	return assets.DamageTypeUntyped, arg
}

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
	verb2nd   string // "You {verb} {target}!"
	verb3rd   string // "{attacker} {verb} {target}!"
}{
	{0, "miss", "misses"},
	{2, "barely scratch", "barely scratches"},
	{4, "tickle", "tickles"},
	{6, "barely hurt", "barely hurts"},
	{10, "hit", "hits"},
	{14, "strike", "strikes"},
	{19, "pummel", "pummels"},
	{24, "thrash", "thrashes"},
	{30, "maul", "mauls"},
	{40, "decimate", "decimates"},
	{50, "devastate", "devastates"},
	{65, "obliterate", "obliterates"},
	{80, "annihilate", "annihilates"},
}

// damageVerb2nd returns the 2nd person verb for a damage amount ("miss", "hit").
func damageVerb2nd(damage int) string {
	for _, msg := range damageMessages {
		if damage <= msg.maxDamage {
			return msg.verb2nd
		}
	}
	return "do UNSPEAKABLE things to"
}

// damageVerb3rd returns the 3rd person verb for a damage amount ("misses", "hits").
func damageVerb3rd(damage int) string {
	for _, msg := range damageMessages {
		if damage <= msg.maxDamage {
			return msg.verb3rd
		}
	}
	return "does UNSPEAKABLE things to"
}

// HitMsgActor returns a 2nd-person message: "You pummel Goblin!" or "You miss Goblin!"
func HitMsgActor(targetName string, damage int) string {
	return "You " + damageVerb2nd(damage) + " " + targetName + "!"
}

// HitMsgTarget returns a message for the target: "Player pummels you!" or "Player misses you!"
func HitMsgTarget(actorName string, damage int) string {
	return actorName + " " + damageVerb3rd(damage) + " you!"
}

// HitMsgRoom returns a 3rd-person message: "Player pummels Goblin!" or "Player misses Goblin!"
func HitMsgRoom(actorName, targetName string, damage int) string {
	return actorName + " " + damageVerb3rd(damage) + " " + targetName + "!"
}
