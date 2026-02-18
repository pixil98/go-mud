package game

import (
	"fmt"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// Actor holds properties shared between characters and mobiles.
type Actor struct {
	Pronoun storage.Identifier `json:"pronoun,omitempty"`
	Race    storage.Identifier `json:"race,omitempty"`
	Level   int                `json:"level,omitempty"`
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
// TODO: Add stackable item support (keyed by ObjectId with count) for commodities.
type Inventory struct {
	// Items maps instance IDs to object instances
	Items map[string]*ObjectInstance `json:"items,omitempty"`
}

// NewInventory creates an empty inventory.
func NewInventory() *Inventory {
	return &Inventory{
		Items: make(map[string]*ObjectInstance),
	}
}

// Add adds an object instance to the inventory.
func (inv *Inventory) AddObj(obj *ObjectInstance) {
	if inv.Items == nil {
		inv.Items = make(map[string]*ObjectInstance)
	}
	inv.Items[obj.InstanceId] = obj
}

// Remove removes an object instance from the inventory.
// Returns the removed instance, or nil if not found.
func (inv *Inventory) RemoveObj(instanceId string) *ObjectInstance {
	if obj, ok := inv.Items[instanceId]; ok {
		delete(inv.Items, instanceId)
		return obj
	}
	return nil
}

// FindObj searches inventory items for one whose definition matches the given alias.
// Returns nil if not found.
func (inv *Inventory) FindObj(name string) *ObjectInstance {
	for _, oi := range inv.Items {
		if oi.Definition.MatchName(name) {
			return oi
		}
	}
	return nil
}

// EquipSlot pairs a slot type name with the equipped object instance.
type EquipSlot struct {
	Slot string          `json:"slot"`
	Obj  *ObjectInstance `json:"obj"`
}

// Equipment holds items equipped by a character or mobile.
// Multiple items may share the same slot type (e.g., two rings in "finger").
type Equipment struct {
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
	if maxSlots > 0 && eq.SlotCount(slot) >= maxSlots {
		return fmt.Errorf("no available %q slot", slot)
	}
	eq.Items = append(eq.Items, EquipSlot{Slot: slot, Obj: obj})
	return nil
}

// SlotCount returns how many items are equipped in the given slot type.
func (eq *Equipment) SlotCount(slot string) int {
	count := 0
	for _, item := range eq.Items {
		if item.Slot == slot {
			count++
		}
	}
	return count
}

// FindObj searches equipped items for one whose definition matches the given alias.
// Returns nil if not found.
func (eq *Equipment) FindObj(name string) *ObjectInstance {
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

// Remove finds and unequips an object by instance ID.
func (eq *Equipment) RemoveObj(instanceId string) *ObjectInstance {
	for i, item := range eq.Items {
		if item.Obj.InstanceId == instanceId {
			eq.Items = append(eq.Items[:i], eq.Items[i+1:]...)
			return item.Obj
		}
	}
	return nil
}
