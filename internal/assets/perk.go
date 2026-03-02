package assets

import "fmt"

// PerkKey identifies a well-known key for key_mod perks. Asset-defined keys
// (e.g. "evocation.fire.damage_pct") don't need consts — these are provided
// for engine-interpreted values so they're discoverable and consistent.
type PerkKey = string

const (
	PerkKeySTR     PerkKey = "core.stats.str"
	PerkKeyDEX     PerkKey = "core.stats.dex"
	PerkKeyCON     PerkKey = "core.stats.con"
	PerkKeyINT     PerkKey = "core.stats.int"
	PerkKeyWIS     PerkKey = "core.stats.wis"
	PerkKeyCHA     PerkKey = "core.stats.cha"
	PerkKeyMaxHp   PerkKey = "core.hp.max"
	PerkKeyMaxMana PerkKey = "core.mana.max"
)

// StatPerkKeys maps stat-related PerkKey consts to their corresponding StatKey.
// Use this to extract ability score modifiers from key_mod perks without
// hardcoding the key strings at the call site.
var StatPerkKeys = map[PerkKey]StatKey{
	PerkKeySTR: StatSTR,
	PerkKeyDEX: StatDEX,
	PerkKeyCON: StatCON,
	PerkKeyINT: StatINT,
	PerkKeyWIS: StatWIS,
	PerkKeyCHA: StatCHA,
}

// Perk describes an effect granted when a node or spine node is unlocked, or
// by a race. Which fields are meaningful depends on Type.
type Perk struct {
	Type  string `json:"type"`
	Id    string `json:"id,omitempty"`    // unlock_ability: the ability id to grant
	Key   string `json:"key,omitempty"`   // key_mod: the key to modify
	Value int    `json:"value,omitempty"` // key_mod: amount to add per rank
	Tag   string `json:"tag,omitempty"`   // tag: the keyword flag to grant
}

func (p *Perk) validate() error {
	if p.Type == "" {
		return fmt.Errorf("type is required")
	}

	switch p.Type {
	case "unlock_ability":
		if p.Id == "" {
			return fmt.Errorf("unlock_ability perk requires id")
		}
	case "key_mod":
		if p.Key == "" {
			return fmt.Errorf("key_mod perk requires key")
		}
	case "tag":
		if p.Tag == "" {
			return fmt.Errorf("tag perk requires tag")
		}
	default:
		return fmt.Errorf("unknown perk type: %q", p.Type)
	}

	return nil
}
