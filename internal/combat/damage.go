package combat

import "math/rand/v2"

// RollAttack rolls a d20 and adds the attack modifier.
func RollAttack(attackMod int) int {
	return rand.IntN(20) + 1 + attackMod
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
