package game

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"

	"github.com/google/uuid"
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

const (
	// wanderChance is the 1-in-N chance a mob attempts to wander each tick.
	wanderChance = 20
	// scavengeChance is the 1-in-N chance a scavenger mob picks up an item each tick.
	scavengeChance = 10
)

// MobCommander executes a command on behalf of a mob.
type MobCommander interface {
	ExecMobCommand(ctx context.Context, mob *MobileInstance, cmd string, args ...string) error
}

// MobileInstance represents a single spawned instance of a Mobile definition.
// Location is set at spawn time and tracks which zone/room contains this mob.
type MobileInstance struct {
	Mobile    storage.SmartIdentifier[*assets.Mobile]
	Commander MobCommander // if set, the mob executes commands autonomously on tick

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

// Tick advances one game tick: expires timed perks, regenerates resources,
// and runs autonomous behavior (wandering, scavenging) when not in combat.
func (mi *MobileInstance) Tick() {
	if mi.Commander == nil {
		slog.Error("mob ticking without commander", "mob", mi.Mobile.Id())
		return
	}

	mi.PerkCache.Tick()
	mi.inventory.Tick()
	mi.equipment.Tick()
	if !mi.IsInCombat() {
		mi.mu.Lock()
		mi.regenTick()
		mi.mu.Unlock()
	}

	// Autonomous behavior requires no combat.
	if mi.IsInCombat() {
		return
	}
	mi.tryWander()
	mi.tryScavenge()
}

// tryWander gives the mob a chance to move to a random adjacent room.
func (mi *MobileInstance) tryWander() {
	if mi.Mobile.Get().HasFlag(assets.MobileFlagSentinel) {
		return
	}
	if rand.IntN(wanderChance) != 0 {
		return
	}

	from := mi.Room()
	if len(from.exits) == 0 {
		return
	}

	stayZone := mi.Mobile.Get().HasFlag(assets.MobileFlagStayZone)

	var directions []string
	for dir, re := range from.exits {
		if re.closed || re.Dest == nil {
			continue
		}
		destDef := re.Dest.Room.Get()
		if destDef.HasFlag(assets.RoomFlagNoMob) || destDef.HasFlag(assets.RoomFlagDeath) {
			continue
		}
		if stayZone && re.Dest.zone != from.zone {
			continue
		}
		directions = append(directions, dir)
	}

	if len(directions) == 0 {
		return
	}

	dir := directions[rand.IntN(len(directions))]
	if err := mi.Commander.ExecMobCommand(context.Background(), mi, dir); err != nil {
		slog.Debug("mob wander failed", "mob", mi.Mobile.Id(), "direction", dir, "error", err)
	}
}

// tryScavenge gives a scavenger mob a chance to pick up an item from its room.
func (mi *MobileInstance) tryScavenge() {
	if !mi.Mobile.Get().HasFlag(assets.MobileFlagScavenger) {
		return
	}
	if rand.IntN(scavengeChance) != 0 {
		return
	}

	from := mi.Room()
	if from.objects.Len() == 0 {
		return
	}

	var alias string
	from.objects.ForEachObj(func(_ string, oi *ObjectInstance) {
		if alias != "" {
			return
		}
		def := oi.Object.Get()
		if def.HasFlag(assets.ObjectFlagImmobile) {
			return
		}
		if len(def.Aliases) > 0 {
			alias = def.Aliases[0]
		}
	})
	if alias == "" {
		return
	}
	if err := mi.Commander.ExecMobCommand(context.Background(), mi, "get", alias); err != nil {
		slog.Debug("mob scavenge failed", "mob", mi.Mobile.Id(), "item", alias, "error", err)
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
