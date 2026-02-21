package combat

import "math/rand/v2"

// RollAttack rolls a d20 and adds the attack modifier.
func RollAttack(attackMod int) int {
	return rand.IntN(20) + 1 + attackMod
}

// RollDamage rolls NdS + modifier, with a minimum result of 1.
func RollDamage(dice, sides, mod int) int {
	total := mod
	for range dice {
		total += rand.IntN(sides) + 1
	}
	if total < 1 {
		total = 1
	}
	return total
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
	{14, "hits hard"},
	{19, "pummels"},
	{24, "thrashes"},
	{30, "mauls"},
	{40, "decimates"},
	{50, "devastates"},
	{65, "obliterates"},
	{80, "annihilates"},
}

// DamageVerb returns the 3rd person verb for a damage amount.
func DamageVerb(damage int) string {
	for _, msg := range damageMessages {
		if damage <= msg.maxDamage {
			return msg.verb3rd
		}
	}
	return "does UNSPEAKABLE things to"
}
