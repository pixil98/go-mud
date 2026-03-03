package combat

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// PlayerCombatant adapts a CharacterInstance for the combat system.
type PlayerCombatant struct {
	Character storage.SmartIdentifier[*assets.Character]
	Player    *game.CharacterInstance
}

func (c *PlayerCombatant) CombatID() string   { return fmt.Sprintf("player:%s", c.Character.Id()) }
func (c *PlayerCombatant) CombatName() string { return c.Character.Get().Name }

func (c *PlayerCombatant) IsAlive() bool {
	cur, _ := c.Player.HP()
	return cur > 0
}

func (c *PlayerCombatant) AC() int {
	stats := c.Player.EffectiveStats()
	return 10 + stats[assets.StatDEX].Mod() + c.Player.PerkValue(assets.PerkKeyCombatAC)
}

func (c *PlayerCombatant) Attacks() []Attack {
	char := c.Character.Get()
	stats := c.Player.EffectiveStats()
	strMod := stats[assets.StatSTR].Mod()
	attackMod := strMod + char.Level/2
	dmgMod := strMod + c.Player.PerkValue(assets.PerkKeyCombatDmgMod)

	var attacks []Attack
	c.Player.GetEquipment().ForEachSlot(func(slot game.EquipSlot) {
		if slot.Slot != "wield" || slot.Obj == nil {
			return
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
			DamageMod:   dmgMod,
		})
	})

	// Unarmed fallback
	if len(attacks) == 0 {
		attacks = append(attacks, Attack{
			Mod:         attackMod,
			DamageDice:  1,
			DamageSides: 4,
			DamageMod:   dmgMod,
		})
	}
	return attacks
}

func (c *PlayerCombatant) AdjustHP(delta int) { c.Player.AdjustHP(delta) }
func (c *PlayerCombatant) SetInCombat(v bool)   { c.Player.SetInCombat(v) }
func (c *PlayerCombatant) Level() int           { return c.Character.Get().Level }

// AwardXP adds amount to the player's experience and returns the message to
// send them. Returns empty string if the award is zero.
func (c *PlayerCombatant) AwardXP(amount int) string {
	if amount <= 0 {
		return ""
	}
	char := c.Character.Get()
	char.Experience += amount
	msg := fmt.Sprintf("You receive %d experience points.", amount)
	if game.ExpToNextLevel(char.Level, char.Experience) <= 0 {
		msg += " You feel ready to advance!"
	}
	return msg
}

// MobCombatant adapts a MobileInstance for the combat system.
type MobCombatant struct {
	Instance *game.MobileInstance
}

func (c *MobCombatant) CombatID() string   { return fmt.Sprintf("mob:%s", c.Instance.InstanceId) }
func (c *MobCombatant) CombatName() string { return c.Instance.Mobile.Get().ShortDesc }

func (c *MobCombatant) IsAlive() bool {
	cur, _ := c.Instance.HP()
	return cur > 0
}

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

func (c *MobCombatant) AdjustHP(delta int) { c.Instance.AdjustHP(delta) }
func (c *MobCombatant) SetInCombat(v bool)   { c.Instance.SetInCombat(v) }
func (c *MobCombatant) Level() int           { return c.Instance.Mobile.Get().Level }

// ExpReward returns the XP value of killing this mob. If the mob has no
// explicit exp_reward, falls back to the level-based formula.
func (c *MobCombatant) ExpReward() int {
	def := c.Instance.Mobile.Get()
	if def.ExpReward > 0 {
		return def.ExpReward
	}
	return game.BaseExpForLevel(def.Level)
}
