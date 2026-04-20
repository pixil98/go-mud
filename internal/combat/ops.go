package combat

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// StartCombat registers mutual threat between attacker and target.
// Idempotent: re-entering after flee preserves existing threat entries.
func StartCombat(attacker, target game.Actor) error {
	if !attacker.IsAlive() {
		return fmt.Errorf("%s is not alive", attacker.Name())
	}
	if !target.IsAlive() {
		return fmt.Errorf("%s is not alive", target.Name())
	}
	attacker.EnsureThreat(target.Id(), target)
	target.EnsureThreat(attacker.Id(), attacker)
	return nil
}

// AddThreat increases the threat that source has generated toward target.
// Threat modifiers are applied via CalcThreat.
func AddThreat(source, target game.Actor, amount int) {
	target.AddThreatFrom(source.Id(), CalcThreat(amount, source))
}

// SetThreat sets the threat that source has on target's threat table to an
// absolute value, ignoring the threat modifier.
func SetThreat(source, target game.Actor, amount int) {
	target.SetThreatFrom(source.Id(), amount)
}

// TopThreat sets source's threat on target to one more than the current
// highest entry, guaranteeing source becomes the top-threat enemy.
func TopThreat(source, target game.Actor) {
	target.TopThreatFrom(source.Id())
}

// NotifyHeal adds heal-generated threat from healer to every actor in
// roomOccupants whose threat table contains target. The caller assembles the
// occupant list from the room.
func NotifyHeal(healer, target game.Actor, amount int, roomOccupants []game.Actor) {
	modified := CalcThreat(amount, healer)
	if modified <= 0 {
		return
	}
	healerId := healer.Id()
	targetId := target.Id()
	for _, occupant := range roomOccupants {
		if occupant.Id() == healerId {
			continue
		}
		if !occupant.HasThreatFrom(targetId) {
			continue
		}
		occupant.AddThreatFrom(healerId, modified)
		healer.EnsureThreat(occupant.Id(), occupant)
	}
}
