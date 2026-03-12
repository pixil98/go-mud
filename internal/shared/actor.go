package shared

import (
	"github.com/pixil98/go-mud/internal/assets"
)

// Actor is the shared interface satisfied by both CharacterInstance and
// MobileInstance. It provides everything the ability and combat systems
// need to interact with an entity without depending on concrete game types.
type Actor interface {
	Id() string
	Name() string
	Location() (zoneId, roomId string)
	IsInCombat() bool
	IsAlive() bool
	Level() int
	Resource(name string) (current, max int)
	AdjustResource(name string, delta int, overfill bool)
	SpendAP(cost int) bool
	HasGrant(key, arg string) bool
	ModifierValue(key string) int
	GrantArgs(key string) []string
	AddTimedPerks(name string, perks []assets.Perk, ticks int)
	SetInCombat(bool)
	CombatTargetId() string
	SetCombatTargetId(id string)
	OnDeath() []any
	IsCharacter() bool
}
