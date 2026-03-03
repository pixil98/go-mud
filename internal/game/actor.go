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
	inventory *Inventory
	equipment *Equipment
	maxHP     int
	currentHP int
}

// adjustHP changes currentHP by delta (positive = heal, negative = damage),
// clamping the result to [0, maxHP]. Caller must hold the owning type's write lock.
func (a *ActorInstance) adjustHP(delta int) {
	a.currentHP = max(0, min(a.currentHP+delta, a.maxHP))
}
