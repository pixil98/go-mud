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

	Mobile   storage.SmartIdentifier[*assets.Mobile]
	inCombat bool
	zoneId   string
	roomId   string

	ActorInstance
}

// NewMobileInstance constructs a fully initialized MobileInstance from a mob
// definition. The caller is responsible for placing it in a room via RoomInstance.AddMob.
func NewMobileInstance(mob storage.SmartIdentifier[*assets.Mobile]) (*MobileInstance, error) {
	def := mob.Get()
	eq := NewEquipment()
	mi := &MobileInstance{
		Mobile: mob,
		ActorInstance: ActorInstance{
			InstanceId: uuid.New().String(),
			inventory:  NewInventory(),
			equipment: eq,
			level:     def.Level,
			PerkCache: *NewPerkCache(def.Perks, map[string]PerkSource{"equipment": eq}),
		},
	}
	mi.initResources()
	for _, spawn := range def.Inventory {
		oi, err := SpawnObject(spawn)
		if err != nil {
			return nil, fmt.Errorf("spawning inventory for %q: %w", mob.Id(), err)
		}
		mi.inventory.AddObj(oi)
	}
	for _, es := range def.Equipment {
		oi, err := SpawnObject(es.ObjectSpawn)
		if err != nil {
			return nil, fmt.Errorf("spawning equipment for %q: %w", mob.Id(), err)
		}
		if err := mi.equipment.Equip(es.Slot, 0, oi); err != nil {
			return nil, fmt.Errorf("equipping %q on %q: %w", es.Slot, mob.Id(), err)
		}
	}
	return mi, nil
}

// --- Accessor methods ---

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
func (mi *MobileInstance) Resource(name string) (current, maximum int) {
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
// When overfill is true the max clamp is skipped, allowing values above maximum.
func (mi *MobileInstance) AdjustResource(name string, delta int, overfill bool) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	mi.adjustResource(name, delta, overfill)
}

// Tick advances one game tick: expires timed perks and regenerates
// resources when out of combat.
func (mi *MobileInstance) Tick() {
	mi.PerkCache.Tick()
	mi.inventory.Tick()
	mi.equipment.Tick()
	if !mi.IsInCombat() {
		mi.mu.Lock()
		mi.regenTick()
		mi.mu.Unlock()
	}
}

// Inventory returns the mobile's inventory.
// Inventory is self-locking; its methods are safe for concurrent use.
func (mi *MobileInstance) Inventory() *Inventory {
	return mi.inventory
}

// Equipment returns the mobile's equipment.
// Equipment is self-locking; its methods are safe for concurrent use.
func (mi *MobileInstance) Equipment() *Equipment {
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

// IsCharacter returns false for mobs.
func (mi *MobileInstance) IsCharacter() bool { return false }

// Notify is a no-op for mobs since they have no client connection.
func (mi *MobileInstance) Notify(_ string) {}

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
	si := storage.NewResolvedSmartIdentifier("corpse-"+mi.Id(), corpseObj)
	corpse := &ObjectInstance{
		InstanceId: uuid.New().String(),
		Object:     si,
		Contents:   NewInventory(),
	}
	for _, oi := range mi.inventory.Drain() {
		oi.ActivateDecay()
		corpse.Contents.AddObj(oi)
	}
	for _, oi := range mi.equipment.Drain() {
		oi.ActivateDecay()
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

// Asset returns a synthetic Character for template expansion in the ability system.
// Mobs don't have a full Character spec, so only the name is populated.
func (mi *MobileInstance) Asset() *assets.Character {
	return &assets.Character{Name: mi.Name()}
}

// SpendAP always succeeds for mobs — they have no action point budget.
func (mi *MobileInstance) SpendAP(_ int) bool { return true }

// Group returns nil; mobs are never in a player group.
func (mi *MobileInstance) Group() *Group { return nil }

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
