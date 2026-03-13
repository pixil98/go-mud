package combat

import (
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
)

// AbilityHandler executes a compiled ability on behalf of a combatant during
// the combat tick. The combat manager iterates auto_use grants and delegates
// execution to this interface.
type AbilityHandler interface {
	ExecCombatAbility(abilityId string, actor, target shared.Actor) (roomMsg string, err error)
}

// ZoneLocator retrieves a zone instance by zone ID, for publishing room messages.
type ZoneLocator interface {
	GetZone(zoneId string) *game.ZoneInstance
}
