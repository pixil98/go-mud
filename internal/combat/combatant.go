package combat

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

	OnDeath()
}
