package game

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/storage"
)

// PlayerCombatant adapts a PlayerState for the combat system.
type PlayerCombatant struct {
	CharId    storage.Identifier
	Player    *PlayerState
	Character *Character
}

func (c *PlayerCombatant) CombatID() string       { return fmt.Sprintf("player:%s", c.CharId) }
func (c *PlayerCombatant) CombatName() string      { return c.Character.Name }
func (c *PlayerCombatant) CombatSide() combat.Side { return combat.SidePlayer }
func (c *PlayerCombatant) IsAlive() bool           { return c.Character.CurrentHP > 0 }
func (c *PlayerCombatant) AC() int                 { return 10 }
func (c *PlayerCombatant) AttackMod() int          { return c.Character.Level / 2 }
func (c *PlayerCombatant) DamageDice() int         { return 1 }
func (c *PlayerCombatant) DamageSides() int        { return 4 }
func (c *PlayerCombatant) DamageMod() int          { return 0 }

func (c *PlayerCombatant) ApplyDamage(dmg int) {
	c.Character.CurrentHP -= dmg
	if c.Character.CurrentHP < 0 {
		c.Character.CurrentHP = 0
	}
}

func (c *PlayerCombatant) SetInCombat(v bool) {
	c.Player.InCombat = v
}

// MobCombatant adapts a MobileInstance for the combat system.
type MobCombatant struct {
	Instance *MobileInstance
}

func (c *MobCombatant) CombatID() string       { return fmt.Sprintf("mob:%s", c.Instance.InstanceId) }
func (c *MobCombatant) CombatName() string      { return c.Instance.Mobile.Get().ShortDesc }
func (c *MobCombatant) CombatSide() combat.Side { return combat.SideMob }
func (c *MobCombatant) IsAlive() bool           { return c.Instance.CurrentHP > 0 }

func (c *MobCombatant) AC() int          { return c.Instance.Mobile.Get().AC }
func (c *MobCombatant) AttackMod() int   { return c.Instance.Mobile.Get().AttackMod }
func (c *MobCombatant) DamageDice() int  { return c.Instance.Mobile.Get().DamageDice }
func (c *MobCombatant) DamageSides() int { return c.Instance.Mobile.Get().DamageSides }
func (c *MobCombatant) DamageMod() int   { return c.Instance.Mobile.Get().DamageMod }

func (c *MobCombatant) ApplyDamage(dmg int) {
	c.Instance.CurrentHP -= dmg
	if c.Instance.CurrentHP < 0 {
		c.Instance.CurrentHP = 0
	}
}

func (c *MobCombatant) SetInCombat(v bool) {
	c.Instance.InCombat = v
}
