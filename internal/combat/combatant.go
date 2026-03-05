package combat

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// combatActor is the subset of game.CharacterInstance / game.MobileInstance
// methods used by the combat system. Both types satisfy this via ActorInstance
// embedding.
type combatActor interface {
	Resource(name string) (current, max int)
	AdjustResource(name string, delta int)
	SetInCombat(v bool)
	ModifierValue(key string) int
	GrantArgs(key string) []string
}

// baseCombatant implements the Combatant methods that are identical for
// players and mobs. Concrete types embed this and provide identity, AC,
// Attacks, and Level.
type baseCombatant struct {
	actor combatActor
}

func (b *baseCombatant) IsAlive() bool {
	cur, _ := b.actor.Resource(assets.ResourceHp)
	return cur > 0
}

func (b *baseCombatant) AdjustHP(delta int) {
	b.actor.AdjustResource(assets.ResourceHp, delta)
}

func (b *baseCombatant) SetInCombat(v bool) {
	b.actor.SetInCombat(v)
}

// buildAttacks constructs the attack list from grant perks, reading attackMod
// and dmgMod from perk keys.
func (b *baseCombatant) buildAttacks() []Attack {
	attackMod := b.actor.ModifierValue(assets.PerkKeyCombatAttackMod)
	dmgMod := b.actor.ModifierValue(assets.PerkKeyCombatDmgMod)

	var attacks []Attack
	for _, expr := range b.actor.GrantArgs(assets.PerkGrantAttack) {
		dice, sides, bonus, ok := parseAttackDice(expr)
		if !ok {
			continue
		}
		attacks = append(attacks, Attack{
			Mod:         attackMod,
			DamageDice:  dice,
			DamageSides: sides,
			DamageMod:   dmgMod + bonus,
		})
	}
	return attacks
}

// parseAttackDice parses a dice expression like "2d6" or "2d6+3" into
// (dice, sides, bonus, ok). Returns ok=false for any unrecognized format.
func parseAttackDice(expr string) (dice, sides, bonus int, ok bool) {
	// Try NdN+N first, then NdN.
	n, err := fmt.Sscanf(expr, "%dd%d+%d", &dice, &sides, &bonus)
	if err == nil && n == 3 && dice >= 1 && sides >= 1 {
		return dice, sides, bonus, true
	}
	n, err = fmt.Sscanf(expr, "%dd%d", &dice, &sides)
	if err == nil && n == 2 && dice >= 1 && sides >= 1 {
		return dice, sides, 0, true
	}
	return 0, 0, 0, false
}

// PlayerCombatant adapts a CharacterInstance for the combat system.
type PlayerCombatant struct {
	baseCombatant
	Character storage.SmartIdentifier[*assets.Character]
	Player    *game.CharacterInstance
}

// NewPlayerCombatant creates a PlayerCombatant for the given character.
func NewPlayerCombatant(char storage.SmartIdentifier[*assets.Character], player *game.CharacterInstance) *PlayerCombatant {
	return &PlayerCombatant{
		baseCombatant: baseCombatant{actor: player},
		Character:     char,
		Player:        player,
	}
}

func (c *PlayerCombatant) CombatID() string   { return fmt.Sprintf("player:%s", c.Character.Id()) }
func (c *PlayerCombatant) CombatName() string { return c.Character.Get().Name }
func (c *PlayerCombatant) Level() int         { return c.Character.Get().Level }

func (c *PlayerCombatant) AC() int {
	stats := c.Player.EffectiveStats()
	return stats[assets.StatDEX].Mod() + c.actor.ModifierValue(assets.PerkKeyCombatAC)
}

func (c *PlayerCombatant) Attacks() []Attack {
	attacks := c.buildAttacks()
	// Unarmed fallback: players always have at least a 1d4 punch.
	if len(attacks) == 0 {
		attacks = append(attacks, Attack{
			Mod:         c.actor.ModifierValue(assets.PerkKeyCombatAttackMod),
			DamageDice:  1,
			DamageSides: 4,
			DamageMod:   c.actor.ModifierValue(assets.PerkKeyCombatDmgMod),
		})
	}
	// Add stat-based bonuses.
	char := c.Character.Get()
	stats := c.Player.EffectiveStats()
	strMod := stats[assets.StatSTR].Mod()
	for i := range attacks {
		attacks[i].Mod += strMod + char.Level/2
		attacks[i].DamageMod += strMod
	}
	return attacks
}

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
	baseCombatant
	Instance *game.MobileInstance
}

// NewMobCombatant creates a MobCombatant for the given mobile instance.
func NewMobCombatant(instance *game.MobileInstance) *MobCombatant {
	return &MobCombatant{
		baseCombatant: baseCombatant{actor: instance},
		Instance:      instance,
	}
}

func (c *MobCombatant) CombatID() string   { return fmt.Sprintf("mob:%s", c.Instance.InstanceId) }
func (c *MobCombatant) CombatName() string { return c.Instance.Mobile.Get().ShortDesc }
func (c *MobCombatant) Level() int         { return c.Instance.Mobile.Get().Level }

func (c *MobCombatant) AC() int {
	return c.actor.ModifierValue(assets.PerkKeyCombatAC)
}

func (c *MobCombatant) Attacks() []Attack {
	return c.buildAttacks()
}

// ExpReward returns the XP value of killing this mob. If the mob has no
// explicit exp_reward, falls back to the level-based formula.
func (c *MobCombatant) ExpReward() int {
	def := c.Instance.Mobile.Get()
	if def.ExpReward > 0 {
		return def.ExpReward
	}
	return game.BaseExpForLevel(def.Level)
}
