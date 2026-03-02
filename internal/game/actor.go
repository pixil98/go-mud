package game

// StatLine is a single line in a stat section.
type StatLine struct {
	Value  string
	Center bool
}

// StatSection is a labeled group of stat lines.
type StatSection struct {
	Header string
	Lines  []StatLine
}

// ActorInstance holds HP, inventory, and equipment shared between CharacterInstance and MobileInstance.
type ActorInstance struct {
	Inventory *Inventory
	Equipment *Equipment
	MaxHP     int
	CurrentHP int
}

// Regenerate heals the actor by amount, capped at MaxHP.
func (a *ActorInstance) Regenerate(amount int) {
	a.CurrentHP = min(a.CurrentHP+amount, a.MaxHP)
}
