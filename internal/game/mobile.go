package game

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// MobileInstance represents a single spawned instance of a Mobile definition.
// Location is set at spawn time and tracks which zone/room contains this mob.
type MobileInstance struct {
	Mobile storage.SmartIdentifier[*assets.Mobile]

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
	mi.self = mi

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
		mi.equipment.equip(es.Slot, oi)
	}
	return mi, nil
}

// --- Accessor methods ---

// Name returns the mobile's display name.
func (mi *MobileInstance) Name() string {
	return mi.Mobile.Get().ShortDesc
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

// Move updates the mob's location between rooms.
func (mi *MobileInstance) Move(fromRoom, toRoom *RoomInstance) {
	fromRoom.RemoveMob(mi.Id())
	toRoom.AddMob(mi)
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
