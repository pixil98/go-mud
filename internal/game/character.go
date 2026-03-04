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
	mu sync.RWMutex

	subscriber Subscriber
	msgs       chan []byte

	Character storage.SmartIdentifier[*assets.Character]

	// Location
	zoneId string
	roomId string

	// Runtime inventory, equipment, and HP (persisted to Character on save)
	ActorInstance

	// Subscriptions
	subs map[string]func()

	// Session state
	quit         bool
	inCombat     bool
	followingId  string // charId of the player being followed (empty = not following)
	group        *Group // current group, or nil if not grouped
	lastActivity time.Time

	// Connection management: closed to signal the active Play() goroutine to exit.
	done chan struct{}

	// Linkless state: player's connection dropped but they remain in the world.
	linkless   bool
	linklessAt time.Time
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
		zoneId:    zoneId,
		roomId:    roomId,
		ActorInstance: ActorInstance{
			inventory: inv,
			equipment: eq,
			maxHP:     c.MaxHP,
			currentHP: c.CurrentHP,
		},
		lastActivity: time.Now(),
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

// --- Connection lifecycle ---

// Done returns the channel that is closed when this session is evicted by a reconnection.
func (ci *CharacterInstance) Done() <-chan struct{} {
	return ci.done
}

// Subscribe adds a new subscription.
func (ci *CharacterInstance) Subscribe(subject string) error {
	if ci.subscriber == nil {
		return fmt.Errorf("subscriber is nil")
	}

	unsub, err := ci.subscriber.Subscribe(subject, func(data []byte) {
		ci.msgs <- data
	})

	// If we somehow are subscribing to a channel we already have, unsubscribe the old one.
	if unsub, ok := ci.subs[subject]; ok {
		unsub()
	}

	if err != nil {
		return fmt.Errorf("subscribing to channel '%s': %w", subject, err)
	}
	ci.subs[subject] = unsub
	return nil
}

// Unsubscribe removes a subscription by name.
func (ci *CharacterInstance) Unsubscribe(subject string) {
	if unsub, ok := ci.subs[subject]; ok {
		unsub()
		delete(ci.subs, subject)
	}
}

// UnsubscribeAll removes all subscriptions.
func (ci *CharacterInstance) UnsubscribeAll() {
	for name, unsub := range ci.subs {
		unsub()
		delete(ci.subs, name)
	}
}

// Kick closes the done channel, signaling the active Play() goroutine to exit.
// Safe to call multiple times; subsequent calls are no-ops.
func (ci *CharacterInstance) Kick() {
	select {
	case <-ci.done:
		// already closed
	default:
		close(ci.done)
	}
}

// Reattach swaps the msgs and done channels for a reconnecting player.
// It unsubscribes all old NATS subscriptions, clears the linkless flag,
// and creates a fresh done channel. The caller must re-subscribe to NATS after this.
func (ci *CharacterInstance) Reattach(msgs chan []byte) {
	ci.UnsubscribeAll()
	ci.msgs = msgs
	ci.done = make(chan struct{})

	ci.mu.Lock()
	ci.linkless = false
	ci.linklessAt = time.Time{}
	ci.lastActivity = time.Now()
	ci.mu.Unlock()
}

// MarkLinkless sets the player as linkless and unsubscribes all NATS subscriptions
// to prevent channel fill-up while they have no active connection.
func (ci *CharacterInstance) MarkLinkless() {
	ci.mu.Lock()
	ci.linkless = true
	ci.linklessAt = time.Now()
	ci.mu.Unlock()

	ci.UnsubscribeAll()
}

// --- Accessor methods ---

// Location returns the player's current zone and room.
func (ci *CharacterInstance) Location() (zoneId, roomId string) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.zoneId, ci.roomId
}

// IsInCombat returns whether the character is currently in combat.
func (ci *CharacterInstance) IsInCombat() bool {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.inCombat
}

// SetInCombat sets the character's combat state.
func (ci *CharacterInstance) SetInCombat(v bool) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.inCombat = v
}

// GetFollowingId returns the charId of the player being followed, or empty.
func (ci *CharacterInstance) GetFollowingId() string {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.followingId
}

// SetFollowingId sets the charId of the player being followed.
func (ci *CharacterInstance) SetFollowingId(id string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.followingId = id
}

// GetGroup returns the character's current group, or nil.
func (ci *CharacterInstance) GetGroup() *Group {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.group
}

// SetGroup sets the character's current group.
func (ci *CharacterInstance) SetGroup(g *Group) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.group = g
}

// IsQuit returns whether the quit flag is set.
func (ci *CharacterInstance) IsQuit() bool {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.quit
}

// SetQuit sets the quit flag.
func (ci *CharacterInstance) SetQuit(v bool) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.quit = v
}

// IsLinkless returns whether the player's connection has dropped.
func (ci *CharacterInstance) IsLinkless() bool {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.linkless
}

// GetLastActivity returns the time of the player's last activity.
func (ci *CharacterInstance) GetLastActivity() time.Time {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.lastActivity
}

// GetLinklessAt returns the time the player went linkless.
func (ci *CharacterInstance) GetLinklessAt() time.Time {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.linklessAt
}

// MarkActive resets the player's idle timer to now.
func (ci *CharacterInstance) MarkActive() {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.lastActivity = time.Now()
}

// HP returns the current and max hit points.
func (ci *CharacterInstance) HP() (current, max int) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.currentHP, ci.maxHP
}

// SetHP sets the current and max hit points.
func (ci *CharacterInstance) SetHP(current, max int) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.currentHP = current
	ci.maxHP = max
}

// AdjustHP changes current HP by delta (positive = heal, negative = damage),
// clamping the result to [0, maxHP].
func (ci *CharacterInstance) AdjustHP(delta int) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.ActorInstance.adjustHP(delta)
}

// GetInventory returns the character's inventory.
// Inventory is self-locking; its methods are safe for concurrent use.
func (ci *CharacterInstance) GetInventory() *Inventory {
	return ci.inventory
}

// GetEquipment returns the character's equipment.
// Equipment is self-locking; its methods are safe for concurrent use.
func (ci *CharacterInstance) GetEquipment() *Equipment {
	return ci.equipment
}

// Flags returns display labels for the player's current state.
func (ci *CharacterInstance) Flags() []string {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	var flags []string
	if ci.inCombat {
		flags = append(flags, "fighting")
	}
	if ci.linkless {
		flags = append(flags, "linkless")
	}
	return flags
}

// --- Game logic ---

// Move updates the player's location and room instance player lists.
// Room locks are acquired first (for map safety), then instance lock (for field update).
func (ci *CharacterInstance) Move(fromRoom, toRoom *RoomInstance) {
	toZoneId := toRoom.Room.Get().Zone.Id()
	toRoomId := toRoom.Room.Id()

	fromRoom.RemovePlayer(ci.Character.Id())
	toRoom.AddPlayer(ci.Character.Id(), ci)

	ci.mu.Lock()
	ci.zoneId = toZoneId
	ci.roomId = toRoomId
	ci.mu.Unlock()
}

// SaveCharacter persists the character's current runtime state to the character store.
func (ci *CharacterInstance) SaveCharacter(chars storage.Storer[*assets.Character]) error {
	ci.mu.RLock()
	c := ci.Character.Get()
	c.LastZone = ci.zoneId
	c.LastRoom = ci.roomId
	c.MaxHP = ci.maxHP
	c.CurrentHP = ci.currentHP
	ci.mu.RUnlock()

	// Convert runtime inventory to spawn specs (Inventory self-locks)
	c.Inventory = nil
	ci.inventory.ForEachObj(func(_ string, oi *ObjectInstance) {
		c.Inventory = append(c.Inventory, objectInstanceToSpawn(oi))
	})

	// Convert runtime equipment to spawn specs (Equipment self-locks)
	c.Equipment = make(map[string]assets.ObjectSpawn)
	ci.equipment.ForEachSlot(func(slot EquipSlot) {
		if slot.Obj != nil {
			c.Equipment[slot.Slot] = objectInstanceToSpawn(slot.Obj)
		}
	})

	return chars.Save(ci.Character.Id(), c)
}

// objectInstanceToSpawn converts a runtime ObjectInstance back to a persistent ObjectSpawn.
func objectInstanceToSpawn(oi *ObjectInstance) assets.ObjectSpawn {
	spawn := assets.ObjectSpawn{
		Object: oi.Object,
	}
	if oi.Contents != nil {
		oi.Contents.ForEachObj(func(_ string, ci *ObjectInstance) {
			spawn.Contents = append(spawn.Contents, objectInstanceToSpawn(ci))
		})
	}
	return spawn
}

// Perks returns the aggregated perks from all sources (race, equipment, etc.).
func (ci *CharacterInstance) Perks() []assets.Perk {
	var perks []assets.Perk
	if r := ci.Character.Get().Race.Get(); r != nil {
		perks = append(perks, r.Perks...)
	}
	perks = append(perks, ci.equipment.Perks()...)
	return perks
}

// ModifierValue sums all modifier perk values matching key across all sources.
func (ci *CharacterInstance) ModifierValue(key assets.PerkKey) int {
	total := 0
	for _, p := range ci.Perks() {
		if p.Type == assets.PerkTypeModifier && p.Key == key {
			total += p.Value
		}
	}
	return total
}

// Grants returns the args of all grant perks with the given key.
func (ci *CharacterInstance) Grants(key string) []string {
	var args []string
	for _, p := range ci.Perks() {
		if p.Type == assets.PerkTypeGrant && p.Key == key {
			args = append(args, p.Arg)
		}
	}
	return args
}

// HasGrant returns true if any grant perk matches both key and arg.
func (ci *CharacterInstance) HasGrant(key, arg string) bool {
	for _, p := range ci.Perks() {
		if p.Type == assets.PerkTypeGrant && p.Key == key && p.Arg == arg {
			return true
		}
	}
	return false
}

// HasAbility returns true if any of the character's aggregated perks grant the given ability ID.
func (ci *CharacterInstance) HasAbility(abilityId string) bool {
	return ci.HasGrant(assets.PerkGrantUnlockAbility, abilityId)
}

// EffectiveStats computes ability scores from base stats + perk modifiers.
// Equipment stat bonuses flow through Perks() automatically.
func (ci *CharacterInstance) EffectiveStats() map[assets.StatKey]Stat {
	char := ci.Character.Get()
	stats := make(map[assets.StatKey]Stat, len(assets.AllStatKeys))
	for _, k := range assets.AllStatKeys {
		stats[k] = Stat(char.BaseStats[k])
	}
	for _, p := range ci.Perks() {
		if p.Type != assets.PerkTypeModifier {
			continue
		}
		if sk, ok := assets.StatPerkKeys[p.Key]; ok {
			stats[sk] += Stat(p.Value)
		}
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
		if p.Type == assets.PerkTypeGrant {
			label := p.Key
			if p.Arg != "" {
				label += ": " + p.Arg
			}
			perkLines = append(perkLines, StatLine{Value: "  " + label})
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
	ac := 10 + stats[assets.StatDEX].Mod() + ci.ModifierValue(assets.PerkKeyCombatAC)
	attackMod := stats[assets.StatSTR].Mod() + char.Level/2

	dmgParts := ci.Grants(assets.PerkGrantAttack)
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
	currentHP, maxHP := ci.HP()
	sections = append(sections, StatSection{
		Header: "Vitals",
		Lines: []StatLine{
			{Value: fmt.Sprintf("  HP: %d/%d", currentHP, maxHP)},
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

	ci.mu.Lock()
	ci.maxHP += hpGain
	ci.currentHP = ci.maxHP
	ci.mu.Unlock()
}

// --- Inventory ---

// Inventory holds object instances carried by a character or mobile.
// All methods are safe for concurrent use.
type Inventory struct {
	mu   sync.RWMutex
	objs map[string]*ObjectInstance
}

// NewInventory creates an empty inventory.
func NewInventory() *Inventory {
	return &Inventory{
		objs: make(map[string]*ObjectInstance),
	}
}

// AddObj adds an object instance to the inventory.
func (inv *Inventory) AddObj(obj *ObjectInstance) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if inv.objs == nil {
		inv.objs = make(map[string]*ObjectInstance)
	}
	inv.objs[obj.InstanceId] = obj
}

// RemoveObj removes an object instance from the inventory.
// Returns the removed instance, or nil if not found.
func (inv *Inventory) RemoveObj(instanceId string) *ObjectInstance {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if obj, ok := inv.objs[instanceId]; ok {
		delete(inv.objs, instanceId)
		return obj
	}
	return nil
}

// FindObj searches inventory items for one whose definition matches the given alias.
// Returns nil if not found.
func (inv *Inventory) FindObj(name string) *ObjectInstance {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	for _, oi := range inv.objs {
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

	for _, oi := range inv.objs {
		if oi.Object.Id() == defId {
			return oi
		}
	}
	return nil
}

// ForEachObj calls fn for each object in the inventory while holding the read lock.
func (inv *Inventory) ForEachObj(fn func(string, *ObjectInstance)) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	for id, oi := range inv.objs {
		fn(id, oi)
	}
}

// Len returns the number of items in the inventory.
func (inv *Inventory) Len() int {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return len(inv.objs)
}

// Clear removes all items.
func (inv *Inventory) Clear() {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	inv.objs = make(map[string]*ObjectInstance)
}

// Drain atomically removes and returns all items.
func (inv *Inventory) Drain() []*ObjectInstance {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	items := make([]*ObjectInstance, 0, len(inv.objs))
	for _, obj := range inv.objs {
		items = append(items, obj)
	}
	inv.objs = make(map[string]*ObjectInstance)
	return items
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
	objs []EquipSlot
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
	eq.objs = append(eq.objs, EquipSlot{Slot: slot, Obj: obj})
	return nil
}

// slotCount returns how many items are equipped in the given slot type.
// Caller must hold at least a read lock.
func (eq *Equipment) slotCount(slot string) int {
	count := 0
	for _, item := range eq.objs {
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

	for _, slot := range eq.objs {
		if slot.Obj == nil {
			continue
		}
		if slot.Obj.Object.Get().MatchName(name) {
			return slot.Obj
		}
	}
	return nil
}

// ForEachSlot calls fn for each equipment slot while holding the read lock.
func (eq *Equipment) ForEachSlot(fn func(EquipSlot)) {
	eq.mu.RLock()
	defer eq.mu.RUnlock()
	for _, slot := range eq.objs {
		fn(slot)
	}
}

// Drain atomically removes and returns all equipped objects.
func (eq *Equipment) Drain() []*ObjectInstance {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	var items []*ObjectInstance
	for _, slot := range eq.objs {
		if slot.Obj != nil {
			items = append(items, slot.Obj)
		}
	}
	eq.objs = []EquipSlot{}
	return items
}

// Len returns the number of equipped items.
func (eq *Equipment) Len() int {
	eq.mu.RLock()
	defer eq.mu.RUnlock()
	return len(eq.objs)
}

// Perks returns the aggregated perks from all equipped items.
func (eq *Equipment) Perks() []assets.Perk {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	var perks []assets.Perk
	for _, slot := range eq.objs {
		if slot.Obj != nil {
			perks = append(perks, slot.Obj.Object.Get().Perks...)
		}
	}
	return perks
}

// RemoveObj finds and unequips an object by instance ID.
func (eq *Equipment) RemoveObj(instanceId string) *ObjectInstance {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	for i, item := range eq.objs {
		if item.Obj.InstanceId == instanceId {
			eq.objs = append(eq.objs[:i], eq.objs[i+1:]...)
			return item.Obj
		}
	}
	return nil
}
