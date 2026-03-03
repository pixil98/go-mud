package game

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// Stat is an ability score value (e.g., 10 for average).
type Stat int

// Mod returns the D&D-style ability modifier: (score - 10) / 2.
func (s Stat) Mod() int {
	return (int(s) - 10) / 2
}

// CharacterInstance holds all mutable state for an active player.
type CharacterInstance struct {
	subscriber Subscriber
	msgs       chan []byte

	Character storage.SmartIdentifier[*assets.Character]

	// Location
	ZoneId string
	RoomId string

	// Runtime inventory, equipment, and HP (persisted to Character on save)
	ActorInstance

	// Subscriptions
	subs map[string]func()

	// Session state
	Quit         bool
	InCombat     bool
	FollowingId  string // charId of the player being followed (empty = not following)
	Group        *Group // current group, or nil if not grouped
	LastActivity time.Time

	// Connection management: closed to signal the active Play() goroutine to exit.
	done chan struct{}

	// Linkless state: player's connection dropped but they remain in the world.
	Linkless   bool
	LinklessAt time.Time
}

// NewCharacterInstance creates a CharacterInstance from a saved character spec,
// materializing inventory and equipment into runtime instances.
func NewCharacterInstance(char storage.SmartIdentifier[*assets.Character], msgs chan []byte, zoneId, roomId string) (*CharacterInstance, error) {
	c := char.Get()
	inv, eq, err := materializeInventoryEquipment(c)
	if err != nil {
		return nil, fmt.Errorf("materializing inventory for %q: %w", char.Id(), err)
	}
	return &CharacterInstance{
		subs:      make(map[string]func()),
		msgs:      msgs,
		Character: char,
		ZoneId:    zoneId,
		RoomId:    roomId,
		ActorInstance: ActorInstance{
			Inventory: inv,
			Equipment: eq,
			MaxHP:     c.MaxHP,
			CurrentHP: c.CurrentHP,
		},
		LastActivity: time.Now(),
		done:         make(chan struct{}),
	}, nil
}

// materializeInventoryEquipment converts persistent spawn specs into runtime instances.
func materializeInventoryEquipment(c *assets.Character) (*Inventory, *Equipment, error) {
	inv := NewInventory()
	for _, spawn := range c.Inventory {
		oi, err := SpawnObject(spawn)
		if err != nil {
			return nil, nil, err
		}
		inv.AddObj(oi)
	}

	eq := NewEquipment()
	for slot, spawn := range c.Equipment {
		oi, err := SpawnObject(spawn)
		if err != nil {
			return nil, nil, err
		}
		if err := eq.Equip(slot, 0, oi); err != nil {
			return nil, nil, err
		}
	}

	return inv, eq, nil
}

// Flags returns display labels for the player's current state.
func (p *CharacterInstance) Flags() []string {
	var flags []string
	if p.InCombat {
		flags = append(flags, "fighting")
	}
	if p.Linkless {
		flags = append(flags, "linkless")
	}
	return flags
}

// Done returns the channel that is closed when this session is evicted by a reconnection.
func (p *CharacterInstance) Done() <-chan struct{} {
	return p.done
}

// Location returns the player's current zone and room.
func (p *CharacterInstance) Location() (zoneId, roomId string) {
	return p.ZoneId, p.RoomId
}

// Move updates the player's location and room instance player lists.
func (p *CharacterInstance) Move(fromRoom, toRoom *RoomInstance) {
	toZoneId := toRoom.Room.Get().Zone.Id()
	toRoomId := toRoom.Room.Id()

	fromRoom.RemovePlayer(p.Character.Id())
	toRoom.AddPlayer(p.Character.Id(), p)

	p.ZoneId = toZoneId
	p.RoomId = toRoomId
}

// Subscribe adds a new subscription.
func (p *CharacterInstance) Subscribe(subject string) error {
	if p.subscriber == nil {
		return fmt.Errorf("subscriber is nil")
	}

	unsub, err := p.subscriber.Subscribe(subject, func(data []byte) {
		p.msgs <- data
	})

	// If we somehow are subscribing to a channel we already have, unsubscribe the old one.
	if unsub, ok := p.subs[subject]; ok {
		unsub()
	}

	if err != nil {
		return fmt.Errorf("subscribing to channel '%s': %w", subject, err)
	}
	p.subs[subject] = unsub
	return nil
}

// Unsubscribe removes a subscription by name.
func (p *CharacterInstance) Unsubscribe(subject string) {
	if unsub, ok := p.subs[subject]; ok {
		unsub()
		delete(p.subs, subject)
	}
}

// UnsubscribeAll removes all subscriptions.
func (p *CharacterInstance) UnsubscribeAll() {
	for name, unsub := range p.subs {
		unsub()
		delete(p.subs, name)
	}
}

// Kick closes the done channel, signaling the active Play() goroutine to exit.
// Safe to call multiple times; subsequent calls are no-ops.
func (p *CharacterInstance) Kick() {
	select {
	case <-p.done:
		// already closed
	default:
		close(p.done)
	}
}

// Reattach swaps the msgs and done channels for a reconnecting player.
// It unsubscribes all old NATS subscriptions, clears the linkless flag,
// and creates a fresh done channel. The caller must re-subscribe to NATS after this.
func (p *CharacterInstance) Reattach(msgs chan []byte) {
	p.UnsubscribeAll()
	p.msgs = msgs
	p.done = make(chan struct{})
	p.Linkless = false
	p.LinklessAt = time.Time{}
	p.LastActivity = time.Now()
}

// MarkLinkless sets the player as linkless and unsubscribes all NATS subscriptions
// to prevent channel fill-up while they have no active connection.
func (p *CharacterInstance) MarkLinkless() {
	p.Linkless = true
	p.LinklessAt = time.Now()
	p.UnsubscribeAll()
}

// SaveCharacter persists the character's current runtime state to the character store.
func (ci *CharacterInstance) SaveCharacter(chars storage.Storer[*assets.Character]) error {
	c := ci.Character.Get()
	c.LastZone = ci.ZoneId
	c.LastRoom = ci.RoomId
	c.MaxHP = ci.MaxHP
	c.CurrentHP = ci.CurrentHP

	// Convert runtime inventory to spawn specs
	c.Inventory = nil
	for _, oi := range ci.Inventory.Objs {
		c.Inventory = append(c.Inventory, objectInstanceToSpawn(oi))
	}

	// Convert runtime equipment to spawn specs
	c.Equipment = make(map[string]assets.ObjectSpawn)
	for _, slot := range ci.Equipment.Objs {
		if slot.Obj != nil {
			c.Equipment[slot.Slot] = objectInstanceToSpawn(slot.Obj)
		}
	}

	return chars.Save(ci.Character.Id(), c)
}

// objectInstanceToSpawn converts a runtime ObjectInstance back to a persistent ObjectSpawn.
func objectInstanceToSpawn(oi *ObjectInstance) assets.ObjectSpawn {
	spawn := assets.ObjectSpawn{
		Object: oi.Object,
	}
	if oi.Contents != nil {
		for _, ci := range oi.Contents.Objs {
			spawn.Contents = append(spawn.Contents, objectInstanceToSpawn(ci))
		}
	}
	return spawn
}

// Perks returns the aggregated perks from all sources (race, tree nodes, etc.).
func (ci *CharacterInstance) Perks() []assets.Perk {
	var perks []assets.Perk
	if r := ci.Character.Get().Race.Get(); r != nil {
		perks = append(perks, r.Perks...)
	}
	return perks
}

// HasAbility returns true if any of the character's aggregated perks include
// an unlock_ability perk matching the given ability ID.
func (ci *CharacterInstance) HasAbility(abilityId string) bool {
	for _, p := range ci.Perks() {
		if p.Type == assets.PerkTypeUnlockAbility && p.Id == abilityId {
			return true
		}
	}
	return false
}

// EffectiveStats computes ability scores from base stats + perk modifiers + equipment bonuses.
func (ci *CharacterInstance) EffectiveStats() map[assets.StatKey]Stat {
	char := ci.Character.Get()
	stats := make(map[assets.StatKey]Stat, len(assets.AllStatKeys))
	for _, k := range assets.AllStatKeys {
		stats[k] = Stat(char.BaseStats[k])
	}
	for _, p := range ci.Perks() {
		if p.Type != assets.PerkTypeKeyMod {
			continue
		}
		if sk, ok := assets.StatPerkKeys[p.Key]; ok {
			stats[sk] += Stat(p.Value)
		}
	}
	for k, v := range ci.Equipment.StatBonuses() {
		stats[k] += Stat(v)
	}
	return stats
}

// StatSections returns the character's stat display sections.
func (ci *CharacterInstance) StatSections() []StatSection {
	char := ci.Character.Get()

	// Identity subtitle: race, level, pronouns
	var parts []string
	if char.Race.Get() != nil {
		parts = append(parts, char.Race.Get().Name)
	}
	parts = append(parts, fmt.Sprintf("Level %d", char.Level))
	if char.Pronoun.Get() != nil {
		parts = append(parts, char.Pronoun.Get().Selector())
	}

	sections := []StatSection{
		{Lines: []StatLine{{Value: strings.Join(parts, " | "), Center: true}}},
	}

	var perkLines []StatLine
	for _, p := range ci.Perks() {
		if p.Type == assets.PerkTypeTag {
			perkLines = append(perkLines, StatLine{Value: "  " + p.Tag})
		}
	}
	if len(perkLines) > 0 {
		sections = append(sections, StatSection{Header: "Perks", Lines: perkLines})
	}

	// Prepend name line
	name := char.Name
	if char.Title != "" {
		name = char.Name + " " + char.Title
	}
	sections[0].Lines = append([]StatLine{{Value: name, Center: true}}, sections[0].Lines...)

	// Stats section
	stats := ci.EffectiveStats()
	sections = append(sections, StatSection{
		Header: "Stats",
		Lines: []StatLine{
			{Value: fmt.Sprintf("  STR: %d (%+d)  DEX: %d (%+d)", stats[assets.StatSTR], stats[assets.StatSTR].Mod(), stats[assets.StatDEX], stats[assets.StatDEX].Mod())},
			{Value: fmt.Sprintf("  CON: %d (%+d)  INT: %d (%+d)", stats[assets.StatCON], stats[assets.StatCON].Mod(), stats[assets.StatINT], stats[assets.StatINT].Mod())},
			{Value: fmt.Sprintf("  WIS: %d (%+d)  CHA: %d (%+d)", stats[assets.StatWIS], stats[assets.StatWIS].Mod(), stats[assets.StatCHA], stats[assets.StatCHA].Mod())},
		},
	})

	// Combat section
	ac := 10 + stats[assets.StatDEX].Mod() + ci.Equipment.ACBonus()
	attackMod := stats[assets.StatSTR].Mod() + char.Level/2

	var dmgParts []string
	for _, slot := range ci.Equipment.Objs {
		if slot.Slot != "wield" || slot.Obj == nil {
			continue
		}
		def := slot.Obj.Object.Get()
		dice, sides := def.DamageDice, def.DamageSides
		if dice == 0 {
			dice = 1
		}
		if sides == 0 {
			sides = 4
		}
		dmgParts = append(dmgParts, fmt.Sprintf("%dd%d", dice, sides))
	}
	if len(dmgParts) == 0 {
		dmgParts = append(dmgParts, "1d4")
	}

	sections = append(sections, StatSection{
		Header: "Combat",
		Lines: []StatLine{
			{Value: fmt.Sprintf("  AC: %d  Attack: %+d  Dmg: %s", ac, attackMod, strings.Join(dmgParts, ", "))},
		},
	})

	// Experience section
	if char.Level >= MaxLevel {
		sections = append(sections, StatSection{
			Header: "Experience",
			Lines: []StatLine{
				{Value: fmt.Sprintf("  XP: %d  (MAX LEVEL)", char.Experience)},
			},
		})
	} else {
		tnl := ExpToNextLevel(char.Level, char.Experience)
		sections = append(sections, StatSection{
			Header: "Experience",
			Lines: []StatLine{
				{Value: fmt.Sprintf("  XP: %d  TNL: %d", char.Experience, tnl)},
			},
		})
	}

	// Vitals section
	sections = append(sections, StatSection{
		Header: "Vitals",
		Lines: []StatLine{
			{Value: fmt.Sprintf("  HP: %d/%d", ci.CurrentHP, ci.MaxHP)},
		},
	})

	return sections
}

// Gain advances the character to the next level, increasing MaxHP.
func (ci *CharacterInstance) Gain() {
	char := ci.Character.Get()
	char.Level++

	// HP gain: 1d8 + CON modifier (minimum 1)
	stats := ci.EffectiveStats()
	conMod := stats[assets.StatCON].Mod()
	hpGain := rand.IntN(8) + 1 + conMod
	if hpGain < 1 {
		hpGain = 1
	}
	ci.MaxHP += hpGain
	ci.CurrentHP = ci.MaxHP
}

// --- Inventory ---

// Inventory holds object instances carried by a character or mobile.
// All methods are safe for concurrent use.
type Inventory struct {
	mu sync.RWMutex
	// Objs maps instance IDs to object instances
	Objs map[string]*ObjectInstance `json:"objects,omitempty"`
}

// NewInventory creates an empty inventory.
func NewInventory() *Inventory {
	return &Inventory{
		Objs: make(map[string]*ObjectInstance),
	}
}

// AddObj adds an object instance to the inventory.
func (inv *Inventory) AddObj(obj *ObjectInstance) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if inv.Objs == nil {
		inv.Objs = make(map[string]*ObjectInstance)
	}
	inv.Objs[obj.InstanceId] = obj
}

// RemoveObj removes an object instance from the inventory.
// Returns the removed instance, or nil if not found.
func (inv *Inventory) RemoveObj(instanceId string) *ObjectInstance {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if obj, ok := inv.Objs[instanceId]; ok {
		delete(inv.Objs, instanceId)
		return obj
	}
	return nil
}

// FindObj searches inventory items for one whose definition matches the given alias.
// Returns nil if not found.
func (inv *Inventory) FindObj(name string) *ObjectInstance {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	for _, oi := range inv.Objs {
		if oi.Object.Get().MatchName(name) {
			return oi
		}
	}
	return nil
}

// FindObjByDef searches for an object whose definition ID matches defId.
// Returns nil if not found.
func (inv *Inventory) FindObjByDef(defId string) *ObjectInstance {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	for _, oi := range inv.Objs {
		if oi.Object.Id() == defId {
			return oi
		}
	}
	return nil
}

// Clear removes all items.
func (inv *Inventory) Clear() {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	inv.Objs = make(map[string]*ObjectInstance)
}

// --- Equipment ---

// EquipSlot pairs a slot type name with the equipped object instance.
type EquipSlot struct {
	Slot string          `json:"slot"`
	Obj  *ObjectInstance `json:"obj"`
}

// Equipment holds items equipped by a character or mobile.
// Multiple items may share the same slot type (e.g., two rings in "finger").
// All methods are safe for concurrent use.
type Equipment struct {
	mu   sync.RWMutex
	Objs []EquipSlot `json:"slots,omitempty"`
}

// NewEquipment creates an empty equipment set.
func NewEquipment() *Equipment {
	return &Equipment{}
}

// Equip adds an object to the given slot type. maxSlots limits how many items
// can occupy that slot type (0 means no limit). Returns an error if the slot
// is already at capacity.
func (eq *Equipment) Equip(slot string, maxSlots int, obj *ObjectInstance) error {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if maxSlots > 0 && eq.slotCount(slot) >= maxSlots {
		return fmt.Errorf("no available %q slot", slot)
	}
	eq.Objs = append(eq.Objs, EquipSlot{Slot: slot, Obj: obj})
	return nil
}

// slotCount returns how many items are equipped in the given slot type.
// Caller must hold at least a read lock.
func (eq *Equipment) slotCount(slot string) int {
	count := 0
	for _, item := range eq.Objs {
		if item.Slot == slot {
			count++
		}
	}
	return count
}

// SlotCount returns how many items are equipped in the given slot type.
func (eq *Equipment) SlotCount(slot string) int {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	return eq.slotCount(slot)
}

// FindObj searches equipped items for one whose definition matches the given alias.
// Returns nil if not found.
func (eq *Equipment) FindObj(name string) *ObjectInstance {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	for _, slot := range eq.Objs {
		if slot.Obj == nil {
			continue
		}
		if slot.Obj.Object.Get().MatchName(name) {
			return slot.Obj
		}
	}
	return nil
}

// ACBonus returns the total AC bonus from all equipped items.
func (eq *Equipment) ACBonus() int {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	total := 0
	for _, slot := range eq.Objs {
		if slot.Obj != nil {
			total += slot.Obj.Object.Get().ACBonus
		}
	}
	return total
}

// StatBonuses returns the combined stat modifiers from all equipped items.
func (eq *Equipment) StatBonuses() map[assets.StatKey]int {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	bonuses := make(map[assets.StatKey]int)
	for _, slot := range eq.Objs {
		if slot.Obj != nil {
			for k, v := range slot.Obj.Object.Get().StatMods {
				bonuses[k] += v
			}
		}
	}
	return bonuses
}

// RemoveObj finds and unequips an object by instance ID.
func (eq *Equipment) RemoveObj(instanceId string) *ObjectInstance {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	for i, item := range eq.Objs {
		if item.Obj.InstanceId == instanceId {
			eq.Objs = append(eq.Objs[:i], eq.Objs[i+1:]...)
			return item.Obj
		}
	}
	return nil
}
