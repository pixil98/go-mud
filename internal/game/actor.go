package game

import (
	"fmt"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// Actor holds properties shared between characters and mobiles.
type Actor struct {
	Pronoun   storage.Identifier `json:"pronoun,omitempty"`
	Race      storage.Identifier `json:"race,omitempty"`
	Level     int                `json:"level,omitempty"`
	Inventory *Inventory         `json:"inventory,omitempty"`
	Equipment *Equipment         `json:"equipment,omitempty"`
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
func (inv *Inventory) Add(obj *ObjectInstance) {
	if inv.Items == nil {
		inv.Items = make(map[string]*ObjectInstance)
	}
	inv.Items[obj.InstanceId] = obj
}

// Remove removes an object instance from the inventory.
// Returns the removed instance, or nil if not found.
func (inv *Inventory) Remove(instanceId string) *ObjectInstance {
	if obj, ok := inv.Items[instanceId]; ok {
		delete(inv.Items, instanceId)
		return obj
	}
	return nil
}

// Equipment holds items equipped by a character or mobile.
// Keys are slot identifiers (e.g., "head", "body", "finger-1").
type Equipment struct {
	Slots map[string]*ObjectInstance `json:"slots,omitempty"`
}

// NewEquipment creates an empty equipment set.
func NewEquipment() *Equipment {
	return &Equipment{
		Slots: make(map[string]*ObjectInstance),
	}
}

// Equip places an object instance in the given slot.
// Returns an error if the slot is already occupied.
func (eq *Equipment) Equip(slot string, obj *ObjectInstance) error {
	if eq.Slots == nil {
		eq.Slots = make(map[string]*ObjectInstance)
	}
	if _, occupied := eq.Slots[slot]; occupied {
		return fmt.Errorf("slot %q is already occupied", slot)
	}
	eq.Slots[slot] = obj
	return nil
}

// GetSlot returns the object instance in the given slot, or nil if empty.
func (eq *Equipment) GetSlot(slot string) *ObjectInstance {
	return eq.Slots[slot]
}

// Remove finds and unequips an object by instance ID.
func (eq *Equipment) Remove(instanceId string) *ObjectInstance {
	for slot, obj := range eq.Slots {
		if obj.InstanceId == instanceId {
			delete(eq.Slots, slot)
			return obj
		}
	}
	return nil
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
type Race struct {
	Name         string         `json:"name"`
	Abbreviation string         `json:"abbreviation"`
	Stats        map[string]int `json:"stats"`
	Perks        []string       `json:"perks"`
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

func (r *Race) Selector() string {
	return r.Name
}
