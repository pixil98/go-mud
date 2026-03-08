package combat

import "github.com/pixil98/go-mud/internal/game"

// Combatant is anything that can participate in combat — player or mob.
type Combatant interface {
	Id() string
	Name() string

	IsInCombat() bool
	SetInCombat(bool)
	IsAlive() bool

	Resource(name string) (current, max int)
	AdjustResource(name string, delta int)

	ModifierValue(key string) int
	GrantArgs(key string) []string

	CombatTargetId() string
	SetCombatTargetId(id string)

	Location() (zoneId, roomId string)

	OnDeath()
}

// ZoneLocator retrieves a zone instance by zone ID, for publishing room messages.
type ZoneLocator interface {
	GetZone(zoneId string) *game.ZoneInstance
}
