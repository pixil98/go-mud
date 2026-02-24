package game

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/pixil98/go-mud/internal/storage"
)

// StatKey identifies an ability score.
type StatKey string

const (
	StatSTR StatKey = "str"
	StatDEX StatKey = "dex"
	StatCON StatKey = "con"
	StatINT StatKey = "int"
	StatWIS StatKey = "wis"
	StatCHA StatKey = "cha"
)

// AllStatKeys defines the canonical display order for ability scores.
var AllStatKeys = []StatKey{StatSTR, StatDEX, StatCON, StatINT, StatWIS, StatCHA}

// Stat is an ability score value (e.g., 10 for average).
type Stat int

// Mod returns the D&D-style ability modifier: (score - 10) / 2.
func (s Stat) Mod() int {
	return (int(s) - 10) / 2
}

// DefaultBaseStats returns base stats initialized to 10 for all abilities.
func DefaultBaseStats() map[StatKey]Stat {
	return map[StatKey]Stat{
		StatSTR: 10, StatDEX: 10, StatCON: 10,
		StatINT: 10, StatWIS: 10, StatCHA: 10,
	}
}

// Character represents a player character in the game.
type Character struct {
	// Name is the character's display name
	Name string `json:"name"`

	// Password is the bcrypt-hashed login credential
	Password string `json:"password"`

	// Title is displayed after the character's name (e.g., "Bob the Brave")
	Title string `json:"title,omitempty"`

	// DetailedDesc is shown when a player looks at this character
	DetailedDesc string `json:"detailed_desc"`

	// Last known location, saved on quit/save for restoring on login
	LastZone storage.Identifier `json:"last_zone,omitempty"`
	LastRoom storage.Identifier `json:"last_room,omitempty"`

	// BaseStats holds the character's base ability scores before modifiers.
	BaseStats map[StatKey]Stat `json:"base_stats,omitempty"`

	// Experience is the character's total accumulated experience points.
	Experience int `json:"experience,omitempty"`

	Actor
	ActorInstance
}

func (c *Character) UnmarshalJSON(b []byte) error {
	type Alias Character
	if err := json.Unmarshal(b, (*Alias)(c)); err != nil {
		return err
	}
	if c.Inventory == nil {
		c.Inventory = NewInventory()
	}
	if c.Equipment == nil {
		c.Equipment = NewEquipment()
	}
	// Migration: characters saved before the leveling system default to level 1.
	if c.Level == 0 {
		c.Level = 1
	}
	return nil
}

func NewCharacter(name string, pass string) *Character {
	maxHP := 20 + (1 * 8) // level 1
	return &Character{
		Name:         name,
		Password:     pass,
		Title:        "the Newbie",
		DetailedDesc: "A plain, unremarkable adventurer.",
		Actor: Actor{
			Level: 1,
		},
		ActorInstance: ActorInstance{
			Inventory: NewInventory(),
			Equipment: NewEquipment(),
			MaxHP:     maxHP,
			CurrentHP: maxHP,
		},
	}
}

// EffectiveStats computes ability scores from base stats + race modifiers + equipment bonuses.
func (c *Character) EffectiveStats() map[StatKey]Stat {
	stats := make(map[StatKey]Stat, len(AllStatKeys))
	for _, k := range AllStatKeys {
		stats[k] = c.BaseStats[k]
	}
	if c.Race.Get() != nil {
		for k, v := range c.Race.Get().StatMods {
			stats[k] += Stat(v)
		}
	}
	for k, v := range c.Equipment.StatBonuses() {
		stats[k] += Stat(v)
	}
	return stats
}

// StatSections returns the character's stat display sections.
func (c *Character) StatSections() []StatSection {
	sections := c.Actor.statSections()

	name := c.Name
	if c.Title != "" {
		name = c.Name + " " + c.Title
	}
	sections[0].Lines = append([]StatLine{{Value: name, Center: true}}, sections[0].Lines...)

	// Stats section
	stats := c.EffectiveStats()
	sections = append(sections, StatSection{
		Header: "Stats",
		Lines: []StatLine{
			{Value: fmt.Sprintf("  STR: %d (%+d)  DEX: %d (%+d)", stats[StatSTR], stats[StatSTR].Mod(), stats[StatDEX], stats[StatDEX].Mod())},
			{Value: fmt.Sprintf("  CON: %d (%+d)  INT: %d (%+d)", stats[StatCON], stats[StatCON].Mod(), stats[StatINT], stats[StatINT].Mod())},
			{Value: fmt.Sprintf("  WIS: %d (%+d)  CHA: %d (%+d)", stats[StatWIS], stats[StatWIS].Mod(), stats[StatCHA], stats[StatCHA].Mod())},
		},
	})

	// Combat section
	ac := 10 + stats[StatDEX].Mod() + c.Equipment.ACBonus()
	attackMod := stats[StatSTR].Mod() + c.Level/2

	var dmgParts []string
	for _, slot := range c.Equipment.Objs {
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
	if c.Level >= MaxLevel {
		sections = append(sections, StatSection{
			Header: "Experience",
			Lines: []StatLine{
				{Value: fmt.Sprintf("  XP: %d  (MAX LEVEL)", c.Experience)},
			},
		})
	} else {
		tnl := ExpToNextLevel(c.Level, c.Experience)
		sections = append(sections, StatSection{
			Header: "Experience",
			Lines: []StatLine{
				{Value: fmt.Sprintf("  XP: %d  TNL: %d", c.Experience, tnl)},
			},
		})
	}

	// Vitals section
	sections = append(sections, StatSection{
		Header: "Vitals",
		Lines: []StatLine{
			{Value: fmt.Sprintf("  HP: %d/%d", c.CurrentHP, c.MaxHP)},
		},
	})

	return sections
}

// Gain advances the character to the next level, increasing stats accordingly.
// The caller must check ExpToNextLevel before calling this.
func (c *Character) Gain() {
	c.Level++

	// HP gain: 1d8 + CON modifier (minimum 1)
	stats := c.EffectiveStats()
	conMod := stats[StatCON].Mod()
	hpGain := rand.IntN(8) + 1 + conMod
	if hpGain < 1 {
		hpGain = 1
	}
	c.MaxHP += hpGain
	c.CurrentHP = c.MaxHP
}

// MatchName returns true if name matches this character's name (case-insensitive).
func (c *Character) MatchName(name string) bool {
	return strings.EqualFold(c.Name, name)
}

// Resolve resolves all foreign keys on the character from the dictionary.
func (c *Character) Resolve(dict *Dictionary) error {
	if err := c.Actor.Resolve(dict); err != nil {
		return err
	}

	if c.Inventory != nil {
		for _, oi := range c.Inventory.Objs {
			if err := oi.Resolve(dict.Objects); err != nil {
				return err
			}
		}
	}
	if c.Equipment != nil {
		for _, slot := range c.Equipment.Objs {
			if slot.Obj != nil {
				if err := slot.Obj.Resolve(dict.Objects); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Validate a character definition
// TODO: We should validate some things here
func (c *Character) Validate() error {
	return nil
}
