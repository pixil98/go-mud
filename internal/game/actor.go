package game

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

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

// Actor holds properties shared between characters and mobiles.
type Actor struct {
	PronounId storage.Identifier `json:"pronoun,omitempty"`
	RaceId    storage.Identifier `json:"race,omitempty"`
	Level     int                `json:"level,omitempty"`

	// Resolved references (populated post-load, not serialized)
	Race    *Race    `json:"-"`
	Pronoun *Pronoun `json:"-"`
}

// Resolve resolves foreign keys from the dictionary.
// Empty identifiers are skipped (valid for characters that haven't selected yet).
// Returns an error if a non-empty identifier doesn't resolve to a record.
func (a *Actor) Resolve(dict *Dictionary) error {
	if a.RaceId != "" {
		a.Race = dict.Races.Get(string(a.RaceId))
		if a.Race == nil {
			return fmt.Errorf("race %q not found", a.RaceId)
		}
	}
	if a.PronounId != "" {
		a.Pronoun = dict.Pronouns.Get(string(a.PronounId))
		if a.Pronoun == nil {
			return fmt.Errorf("pronoun %q not found", a.PronounId)
		}
	}
	return nil
}

// statSections returns the shared stat display: an identity subtitle
// (race, level, pronouns), stats, and perks. Character and Mobile
// prepend their own name line to the first section.
func (a *Actor) statSections() []StatSection {
	var parts []string
	if a.Race != nil {
		parts = append(parts, a.Race.Name)
	}
	parts = append(parts, fmt.Sprintf("Level %d", a.Level))
	if a.Pronoun != nil {
		parts = append(parts, a.Pronoun.Selector())
	}

	sections := []StatSection{
		{Lines: []StatLine{{Value: strings.Join(parts, " | "), Center: true}}},
	}

	if a.Race != nil && len(a.Race.Stats) > 0 {
		keys := make([]string, 0, len(a.Race.Stats))
		for k := range a.Race.Stats {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var statParts []string
		for _, k := range keys {
			statParts = append(statParts, fmt.Sprintf("%s: %d", strings.ToUpper(k), a.Race.Stats[k]))
		}
		sections = append(sections, StatSection{
			Header: "Stats",
			Lines:  []StatLine{{Value: "  " + strings.Join(statParts, "  ")}},
		})
	}

	if a.Race != nil && len(a.Race.Perks) > 0 {
		var lines []StatLine
		for _, p := range a.Race.Perks {
			lines = append(lines, StatLine{Value: "  " + p})
		}
		sections = append(sections, StatSection{Header: "Perks", Lines: lines})
	}

	return sections
}

type pronounPossessive struct {
	Adjective string `json:"adjective"`
	Pronoun   string `json:"pronoun"`
}

// Pronoun defines a set of pronouns loaded from asset files.
type Pronoun struct {
	Subjective string            `json:"subjective"`
	Objective  string            `json:"objective"`
	Possessive pronounPossessive `json:"possessive"`
	Reflexive  string            `json:"reflexive"`
}

func (p *Pronoun) Validate() error {
	return nil
}

func (p *Pronoun) Selector() string {
	return fmt.Sprintf("%s/%s", p.Subjective, p.Objective)
}

// Race defines a playable race loaded from asset files.
// WearSlots lists the equipment slot types available to this race. Duplicate
// entries indicate multiple slots of the same type (e.g., two "finger" entries
// means two ring slots). The list order defines the display order for the
// equipment command.
type Race struct {
	Name         string         `json:"name"`
	Abbreviation string         `json:"abbreviation"`
	Stats        map[string]int `json:"stats"`
	Perks        []string       `json:"perks"`
	WearSlots    []string       `json:"wear_slots,omitempty"`
}

func (r *Race) Validate() error {
	el := errors.NewErrorList()

	for _, p := range r.Perks {
		el.Add(func() error {
			switch p {
			case "darkvision":
				return nil
			default:
				return fmt.Errorf("unknown perk: %s", p)
			}
		}())
	}

	return el.Err()
}

// SlotCount returns how many slots of the given type this race has.
func (r *Race) SlotCount(slot string) int {
	count := 0
	for _, s := range r.WearSlots {
		if s == slot {
			count++
		}
	}
	return count
}

func (r *Race) Selector() string {
	return r.Name
}

// ActorInstance holds properties shared between Characters and MobileInstances
type ActorInstance struct {
	Inventory *Inventory `json:"inventory,omitempty"`
	Equipment *Equipment `json:"equipment,omitempty"`
}

// Inventory holds object instances carried by a character or mobile.
// All methods are safe for concurrent use.
// TODO: Add stackable item support (keyed by ObjectId with count) for commodities.
type Inventory struct {
	mu sync.RWMutex
	// Items maps instance IDs to object instances
	Items map[string]*ObjectInstance `json:"items,omitempty"`
}

// NewInventory creates an empty inventory.
func NewInventory() *Inventory {
	return &Inventory{
		Items: make(map[string]*ObjectInstance),
	}
}

// AddObj adds an object instance to the inventory.
func (inv *Inventory) AddObj(obj *ObjectInstance) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if inv.Items == nil {
		inv.Items = make(map[string]*ObjectInstance)
	}
	inv.Items[obj.InstanceId] = obj
}

// RemoveObj removes an object instance from the inventory.
// Returns the removed instance, or nil if not found.
func (inv *Inventory) RemoveObj(instanceId string) *ObjectInstance {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	if obj, ok := inv.Items[instanceId]; ok {
		delete(inv.Items, instanceId)
		return obj
	}
	return nil
}

// FindObj searches inventory items for one whose definition matches the given alias.
// Returns nil if not found.
func (inv *Inventory) FindObj(name string) *ObjectInstance {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	for _, oi := range inv.Items {
		if oi.Definition.MatchName(name) {
			return oi
		}
	}
	return nil
}

// Clear removes all items.
func (inv *Inventory) Clear() {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	inv.Items = make(map[string]*ObjectInstance)
}

// EquipSlot pairs a slot type name with the equipped object instance.
type EquipSlot struct {
	Slot string          `json:"slot"`
	Obj  *ObjectInstance `json:"obj"`
}

// Equipment holds items equipped by a character or mobile.
// Multiple items may share the same slot type (e.g., two rings in "finger").
// All methods are safe for concurrent use.
type Equipment struct {
	mu    sync.RWMutex
	Items []EquipSlot `json:"items,omitempty"`
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
	eq.Items = append(eq.Items, EquipSlot{Slot: slot, Obj: obj})
	return nil
}

// slotCount returns how many items are equipped in the given slot type.
// Caller must hold at least a read lock.
func (eq *Equipment) slotCount(slot string) int {
	count := 0
	for _, item := range eq.Items {
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

	for _, slot := range eq.Items {
		if slot.Obj == nil {
			continue
		}
		if slot.Obj.Definition.MatchName(name) {
			return slot.Obj
		}
	}
	return nil
}

// RemoveObj finds and unequips an object by instance ID.
func (eq *Equipment) RemoveObj(instanceId string) *ObjectInstance {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	for i, item := range eq.Items {
		if item.Obj.InstanceId == instanceId {
			eq.Items = append(eq.Items[:i], eq.Items[i+1:]...)
			return item.Obj
		}
	}
	return nil
}
