package game

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// MobileInstance represents a single spawned instance of a Mobile definition.
// Location is set at spawn time and tracks which zone/room contains this mob.
type MobileInstance struct {
	mu sync.RWMutex

	InstanceId string
	Mobile     storage.SmartIdentifier[*assets.Mobile]
	inCombat   bool
	zoneId     string
	roomId     string

	ActorInstance
}

// --- Accessor methods ---

// Id returns the mobile instance's unique identifier.
func (mi *MobileInstance) Id() string {
	return mi.InstanceId
}

// Name returns the mobile's display name.
func (mi *MobileInstance) Name() string {
	return mi.Mobile.Get().ShortDesc
}

// IsInCombat returns whether the mobile is currently in combat.
func (mi *MobileInstance) IsInCombat() bool {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	return mi.inCombat
}

// SetInCombat sets the mobile's combat state.
func (mi *MobileInstance) SetInCombat(v bool) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.inCombat = v
}

// Resource returns the current and max for a named resource.
func (mi *MobileInstance) Resource(name string) (current, max int) {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	return mi.resource(name)
}

// SetResource sets the current value for a named resource, clamped to [0, max].
func (mi *MobileInstance) SetResource(name string, current int) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mx := mi.resourceMax(name)
	mi.setResourceCurrent(name, max(0, min(current, mx)))
}

// AdjustResource changes a resource's current value by delta, clamping to [0, max].
func (mi *MobileInstance) AdjustResource(name string, delta int) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.adjustResource(name, delta)
}

// RegenTick regenerates all resources based on perk-driven regen values.
func (mi *MobileInstance) RegenTick() {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.regenTick()
}

// GetInventory returns the mobile's inventory.
// Inventory is self-locking; its methods are safe for concurrent use.
func (mi *MobileInstance) GetInventory() *Inventory {
	return mi.inventory
}

// GetEquipment returns the mobile's equipment.
// Equipment is self-locking; its methods are safe for concurrent use.
func (mi *MobileInstance) GetEquipment() *Equipment {
	return mi.equipment
}

// IsAlive returns whether the mobile has more than zero hit points.
func (mi *MobileInstance) IsAlive() bool {
	cur, _ := mi.Resource(assets.ResourceHp)
	return cur > 0
}

// Flags returns display labels for the mobile's current state.
func (mi *MobileInstance) Flags() []string {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	var flags []string
	if mi.inCombat {
		flags = append(flags, "fighting")
	}
	return flags
}

// OnDeath creates a corpse containing all of the mob's inventory and equipped items.
// The combat manager places the returned objects in the room after calling this.
func (mi *MobileInstance) OnDeath() []*ObjectInstance {
	return []*ObjectInstance{newCorpse(mi)}
}

// newCorpse creates a container ObjectInstance holding all of the mob's loot.
func newCorpse(mi *MobileInstance) *ObjectInstance {
	name := mi.Name()
	corpseObj := &assets.Object{
		Aliases:      []string{"corpse", name},
		ShortDesc:    fmt.Sprintf("the corpse of %s", name),
		LongDesc:     fmt.Sprintf("The corpse of %s lies here.", name),
		DetailedDesc: fmt.Sprintf("The lifeless body of %s. It may still be carrying some belongings.", name),
		Flags:        []string{"container"},
	}
	si := storage.NewResolvedSmartIdentifier("corpse-"+mi.InstanceId, corpseObj)
	corpse := &ObjectInstance{
		InstanceId: uuid.New().String(),
		Object:     si,
		Contents:   NewInventory(),
	}
	for _, oi := range mi.inventory.Drain() {
		corpse.Contents.AddObj(oi)
	}
	for _, oi := range mi.equipment.Drain() {
		corpse.Contents.AddObj(oi)
	}
	return corpse
}

// Location returns the zone and room ID where this mob is spawned.
func (mi *MobileInstance) Location() (zoneId, roomId string) {
	mi.mu.RLock()
	defer mi.mu.RUnlock()
	return mi.zoneId, mi.roomId
}

// CombatTargetId returns an empty string; mobs select targets via their threat table.
func (mi *MobileInstance) CombatTargetId() string {
	return ""
}

// SetCombatTargetId is a no-op for mobs; their target is resolved from the threat table.
func (mi *MobileInstance) SetCombatTargetId(_ string) {}

// StatSections returns the mobile's stat display sections.
func (mi *MobileInstance) StatSections() []StatSection {
	mob := mi.Mobile.Get()
	return []StatSection{
		{Lines: []StatLine{
			{Value: mob.ShortDesc, Center: true},
			{Value: fmt.Sprintf("Level %d", mob.Level), Center: true},
		}},
	}
}
