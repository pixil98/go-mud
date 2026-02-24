package combat

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// PlayerCombatant adapts a PlayerState for the combat system.
type PlayerCombatant struct {
	CharId    storage.Identifier
	Player    *game.PlayerState
	Character *game.Character
}

func (c *PlayerCombatant) CombatID() string   { return fmt.Sprintf("player:%s", c.CharId) }
func (c *PlayerCombatant) CombatName() string { return c.Character.Name }
func (c *PlayerCombatant) CombatSide() Side   { return SidePlayer }
func (c *PlayerCombatant) IsAlive() bool      { return c.Character.CurrentHP > 0 }

func (c *PlayerCombatant) AC() int {
	stats := c.Character.EffectiveStats()
	return 10 + stats[game.StatDEX].Mod() + c.Character.Equipment.ACBonus()
}

func (c *PlayerCombatant) Attacks() []Attack {
	stats := c.Character.EffectiveStats()
	strMod := stats[game.StatSTR].Mod()
	attackMod := strMod + c.Character.Level/2

	var attacks []Attack
	for _, slot := range c.Character.Equipment.Objs {
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
		attacks = append(attacks, Attack{
			Mod:         attackMod,
			DamageDice:  dice,
			DamageSides: sides,
			DamageMod:   strMod + def.DamageMod,
		})
	}

	// Unarmed fallback
	if len(attacks) == 0 {
		attacks = append(attacks, Attack{
			Mod:         attackMod,
			DamageDice:  1,
			DamageSides: 4,
			DamageMod:   strMod,
		})
	}
	return attacks
}

func (c *PlayerCombatant) ApplyDamage(dmg int) {
	c.Character.CurrentHP -= dmg
	if c.Character.CurrentHP < 0 {
		c.Character.CurrentHP = 0
	}
}

func (c *PlayerCombatant) SetInCombat(v bool) {
	c.Player.InCombat = v
}

func (c *PlayerCombatant) Level() int { return c.Character.Level }

// MobCombatant adapts a MobileInstance for the combat system.
type MobCombatant struct {
	Instance *game.MobileInstance
}

func (c *MobCombatant) CombatID() string   { return fmt.Sprintf("mob:%s", c.Instance.InstanceId) }
func (c *MobCombatant) CombatName() string { return c.Instance.Mobile.Get().ShortDesc }
func (c *MobCombatant) CombatSide() Side   { return SideMob }
func (c *MobCombatant) IsAlive() bool      { return c.Instance.CurrentHP > 0 }

func (c *MobCombatant) AC() int { return c.Instance.Mobile.Get().AC }

func (c *MobCombatant) Attacks() []Attack {
	def := c.Instance.Mobile.Get()
	return []Attack{{
		Mod:         def.AttackMod,
		DamageDice:  def.DamageDice,
		DamageSides: def.DamageSides,
		DamageMod:   def.DamageMod,
	}}
}

func (c *MobCombatant) ApplyDamage(dmg int) {
	c.Instance.CurrentHP -= dmg
	if c.Instance.CurrentHP < 0 {
		c.Instance.CurrentHP = 0
	}
}

func (c *MobCombatant) SetInCombat(v bool) {
	c.Instance.InCombat = v
}

func (c *MobCombatant) Level() int { return c.Instance.Mobile.Get().Level }
