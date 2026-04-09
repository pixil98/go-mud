package combat

import (
	"github.com/pixil98/go-mud/internal/shared"
)

// CombatAbilityResult holds the messages produced by an auto-use combat ability.
type CombatAbilityResult struct {
	RoomMsg   string // 3rd-person message for other players in the room
	TargetMsg string // 2nd-person message for the target player
	TargetId  string // charId of the target player; empty if target is not a player
}

// AbilityHandler executes a compiled ability on behalf of a combatant during
// the combat tick. The combat manager iterates auto_use grants and delegates
// execution to this interface.
type AbilityHandler interface {
	ExecCombatAbility(abilityId string, actor, target shared.Actor) (CombatAbilityResult, error)
}
