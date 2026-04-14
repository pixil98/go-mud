package game

import "fmt"

// processDeath handles an actor's death: creates drops, removes the actor
// from the room, places drops, and distributes XP to player contributors.
// Caller must have already verified ClaimDeath() returned true.
func processDeath(dead Actor, room *RoomInstance) {
	drops := dead.OnDeath()
	room.RemoveMob(dead.Id())
	for _, obj := range drops {
		room.AddObj(obj)
	}

	deathMsg := fmt.Sprintf("%s is DEAD!  R.I.P.", dead.Name())
	room.ForEachPlayer(func(_ string, ci *CharacterInstance) {
		ci.QueueTickMsg(deathMsg)
	})

	snap := dead.ThreatSnapshot()
	if len(snap) == 0 {
		return
	}
	mobLevel := dead.Level()
	baseXP := BaseExpForLevel(mobLevel)
	world := room.Zone().World()
	for actorId := range snap {
		ci := world.GetPlayer(actorId)
		if ci == nil {
			continue
		}
		xp := int(float64(baseXP) * LevelDiffMultiplier(ci.Character.Get().Level, mobLevel))
		canAdvance := ci.GainXP(xp)
		msg := fmt.Sprintf("You receive %d experience points.", xp)
		if canAdvance {
			msg += "\nYou feel ready to advance to the next level!"
		}
		ci.QueueTickMsg(msg)
	}
}
