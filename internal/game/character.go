package game

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

var (
	// ErrNoSuchSlot is returned when a character lacks the slot type entirely.
	ErrNoSuchSlot = errors.New("no such slot")
	// ErrSlotFull is returned when all slots of the given type are occupied.
	ErrSlotFull = errors.New("slot full")
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

	ActorInstance

	subs map[string]func()

	quit           bool
	combatTargetId string
	currentAP      int
	lastActivity   time.Time

	done chan struct{}

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

	// Build perk cache: race perks (own) + equipment (source).
	var racePerks []assets.Perk
	if r := c.Race.Get(); r != nil {
		racePerks = r.Perks
	}

	ci := &CharacterInstance{
		subs:      make(map[string]func()),
		msgs:      msgs,
		Character: char,
		ActorInstance: ActorInstance{
			InstanceId: char.Id(),
			inventory:  inv,
			equipment:  eq,
			level:      c.Level,
			zoneId:     zoneId,
			roomId:     roomId,
			PerkCache:  *NewPerkCache(racePerks, map[string]PerkSource{"equipment": eq}),
		},
		lastActivity: time.Now(),
		done:         make(chan struct{}),
	}

	ci.self = ci

	// Initialize resource pools from perks, then restore persisted current values.
	ci.initResources()
	for name, current := range c.Resources {
		if _, mx := ci.resource(name); mx > 0 {
			ci.setResourceCurrent(name, min(current, mx))
		}
	}

	return ci, nil
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
	for _, es := range c.Equipment {
		oi, err := SpawnObject(es.ObjectSpawn)
		if err != nil {
			return nil, nil, err
		}
		eq.equip(es.Slot, oi)
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

// Name returns the character's display name.
func (ci *CharacterInstance) Name() string {
	return ci.Character.Get().Name
}

// Asset returns the underlying character asset data.
func (ci *CharacterInstance) Asset() *assets.Character {
	return ci.Character.Get()
}

// CombatTargetId returns the InstanceId of the mob being auto-attacked, or empty.
func (ci *CharacterInstance) CombatTargetId() string {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.combatTargetId
}

// SetCombatTargetId sets the mob being auto-attacked and marks the character in combat.
func (ci *CharacterInstance) SetCombatTargetId(mobInstanceId string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.combatTargetId = mobInstanceId
	ci.inCombat = true
}

// ClearCombatTargetId clears the auto-attack target and marks the character out of combat.
func (ci *CharacterInstance) ClearCombatTargetId() {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.combatTargetId = ""
	ci.inCombat = false
}

// SpendAP deducts cost from the character's remaining action points for this tick.
// Returns false without deducting if the character has fewer than cost AP remaining.
func (ci *CharacterInstance) SpendAP(cost int) bool {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	if ci.currentAP < cost {
		return false
	}
	ci.currentAP -= cost
	return true
}

// CurrentAP returns the character's remaining action points for this tick.
func (ci *CharacterInstance) CurrentAP() int {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.currentAP
}

// ResetAP restores the character's action points to their maximum for the tick.
// Maximum is determined by the core.action_points.max modifier (typically provided by race).
func (ci *CharacterInstance) ResetAP() {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.currentAP = ci.ModifierValue(assets.PerkKeyActionPointsMax)
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

// LastActivity returns the time of the player's last activity.
func (ci *CharacterInstance) LastActivity() time.Time {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.lastActivity
}

// LinklessAt returns the time the player went linkless.
func (ci *CharacterInstance) LinklessAt() time.Time {
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

// Tick advances one game tick: expires timed perks, resets action points,
// and regenerates resources when out of combat.
func (ci *CharacterInstance) Tick() {
	ci.PerkCache.Tick()
	ci.inventory.Tick()
	ci.equipment.Tick()
	ci.ResetAP()
	if !ci.IsInCombat() {
		ci.mu.Lock()
		ci.regenTick()
		ci.mu.Unlock()
	}
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

// OnDeath handles player death. The character is healed to full HP before the session
// ends so they reconnect healthy. Location is not changed here; they will respawn
// at their stored home location on next login.
func (ci *CharacterInstance) OnDeath() []*ObjectInstance {
	if ci.msgs != nil {
		select {
		case ci.msgs <- []byte("You have been slain! Darkness consumes you..."):
		default:
		}
	}
	_, maxHP := ci.Resource(assets.ResourceHp)
	ci.SetResource(assets.ResourceHp, maxHP)
	ci.SetQuit(true)
	ci.Kick()
	return nil
}

// IsCharacter returns true for player characters.
func (ci *CharacterInstance) IsCharacter() bool { return true }

// GainXP awards experience points to the character and returns true if the
// character now has enough XP to advance to the next level.
func (ci *CharacterInstance) GainXP(xp int) bool {
	if xp <= 0 {
		return false
	}
	char := ci.Character.Get()
	char.Experience += xp
	return char.Level < MaxLevel && char.Experience >= ExpForLevel(char.Level+1)
}

// Notify sends a message directly to the character's client. Non-blocking;
// drops the message if the channel is full.
func (ci *CharacterInstance) Notify(msg string) {
	if ci.msgs == nil {
		return
	}
	select {
	case ci.msgs <- []byte(msg):
	default:
	}
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
	ci.RemoveSource("room")
	ci.AddSource("room", toRoom.Perks)
	ci.mu.Unlock()
}

// SaveCharacter persists the character's current runtime state to the character store.
// Location is not saved here; it only changes via explicit commands (e.g., sethome).
func (ci *CharacterInstance) SaveCharacter(chars storage.Storer[*assets.Character]) error {
	ci.mu.RLock()
	c := ci.Character.Get()
	c.Resources = make(map[string]int)
	for name, cur := range ci.resources {
		c.Resources[name] = cur
	}
	ci.mu.RUnlock()

	// Convert runtime inventory to spawn specs, skipping decayable items (Inventory self-locks).
	c.Inventory = nil
	ci.inventory.ForEachObj(func(_ string, oi *ObjectInstance) {
		if oi.Object.Get().Lifetime > 0 {
			return
		}
		c.Inventory = append(c.Inventory, objectInstanceToSpawn(oi))
	})

	// Convert runtime equipment to spawn specs, skipping decayable items (Equipment self-locks).
	c.Equipment = nil
	ci.equipment.ForEachSlot(func(slot EquipSlot) {
		if slot.Obj == nil || slot.Obj.Object.Get().Lifetime > 0 {
			return
		}
		c.Equipment = append(c.Equipment, assets.EquipmentSpawn{
			Slot:        slot.Slot,
			ObjectSpawn: objectInstanceToSpawn(slot.Obj),
		})
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

// EffectiveStats computes ability scores from base stats + perk modifiers.
// Equipment stat bonuses flow through the embedded PerkCache automatically.
func (ci *CharacterInstance) EffectiveStats() map[assets.StatKey]Stat {
	char := ci.Character.Get()
	stats := make(map[assets.StatKey]Stat, len(assets.AllStatKeys))
	for _, k := range assets.AllStatKeys {
		stats[k] = Stat(char.BaseStats[k])
	}
	for pk, sk := range assets.StatPerkKeys {
		if v := ci.ModifierValue(pk); v != 0 {
			stats[sk] += Stat(v)
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
	for key, args := range ci.Grants() {
		for _, arg := range args {
			label := key
			if arg != "" {
				label += ": " + arg
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
	ac := assets.ApplyModifiers(stats[assets.StatDEX].Mod(), 0, ci, assets.CombatACPrefix)
	attackMod := assets.ApplyModifiers(stats[assets.StatSTR].Mod()+char.Level/2, 0, ci, assets.CombatAttackPrefix)

	dmgParts := ci.GrantArgs(assets.PerkGrantAttack)
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

	// Modifiers section: show modifiers not already covered by stats/combat/resources.
	var modLines []StatLine
	for key, val := range ci.Modifiers() {
		if val == 0 {
			continue
		}
		if _, isStat := assets.StatPerkKeys[key]; isStat {
			continue
		}
		if strings.HasPrefix(key, assets.ResourcePrefix+".") {
			continue
		}
		if strings.HasPrefix(key, assets.CombatACPrefix+".") ||
			strings.HasPrefix(key, assets.CombatAttackPrefix+".") ||
			strings.HasPrefix(key, assets.CombatThreatPrefix+".") {
			continue
		}
		modLines = append(modLines, StatLine{Value: fmt.Sprintf("  %s: %+d", key, val)})
	}
	if len(modLines) > 0 {
		sections = append(sections, StatSection{Header: "Modifiers", Lines: modLines})
	}

	// Vitals section
	var vitalLines []StatLine
	ci.ForEachResource(func(name string, current, mx int) {
		vitalLines = append(vitalLines, StatLine{
			Value: fmt.Sprintf("  %s", ResourceLine(name, current, mx)),
		})
	})
	if len(vitalLines) > 0 {
		sections = append(sections, StatSection{Header: "Vitals", Lines: vitalLines})
	}

	return sections
}

func (ci *CharacterInstance) SetTitle(t string) {
	ci.Character.Get().Title = t
}

// Gain advances the character to the next level. Resource maxes automatically
// increase via per_level perks; all resources are restored to their new max.
func (ci *CharacterInstance) Gain() {
	char := ci.Character.Get()
	char.Level++

	ci.mu.Lock()
	ci.level = char.Level
	for name := range ci.resources {
		ci.setResourceCurrent(name, ci.resourceMax(name))
	}
	ci.mu.Unlock()
}

// Equip validates that the character has an available equipment slot of the
// given type (derived from PerkGrantWearSlot grants) and equips the object.
// Returns ErrNoSuchSlot if the character lacks that slot type entirely, or
// ErrSlotFull if all slots of that type are occupied.
func (ci *CharacterInstance) Equip(slot string, obj *ObjectInstance) error {
	maxSlots := countSlot(ci.GrantArgs(assets.PerkGrantWearSlot), slot)
	if maxSlots == 0 {
		return ErrNoSuchSlot
	}
	if ci.equipment.SlotCount(slot) >= maxSlots {
		return ErrSlotFull
	}
	ci.equipment.equip(slot, obj)
	return nil
}
