package commands

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
)

// attackEffect reads the actor's attack grants and performs one attack roll per
// grant. Each hit delegates to dealDamage for damage application and threat.
type attackEffect struct {
	combat CombatManager
}

func (e *attackEffect) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeMobile, Required: true},
		},
	}
}

func (e *attackEffect) ValidateConfig(_ map[string]string) error { return nil }

func (e *attackEffect) Create(_ string, _ map[string]string, targets []assets.TargetSpec) EffectFunc {
	return func(actor shared.Actor, resolved map[string]*TargetRef, result *AbilityResult) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
		}
		for _, spec := range targets {
			ref := resolved[spec.Name]
			if ref == nil || ref.Actor == nil {
				continue
			}
			target := ref.Actor.Actor()
			if err := e.combat.StartCombat(actor, target); err != nil {
				return NewUserError(err.Error())
			}
			actorName := actor.Name()
			targetName := ref.Actor.Name
			attackArgs := actor.GrantArgs(assets.PerkGrantAttack)
			if len(attackArgs) == 0 {
				attackArgs = []string{"1d4"}
			}
			for _, arg := range attackArgs {
				dmgType, diceExpr := combat.ParseAttackArg(arg)
				attackBonus := assets.ApplyModifiers(0, 0, actor, assets.CombatAttackPrefix)
				roll := combat.RollAttack(attackBonus)
				ac := assets.ApplyModifiers(0, 0, target, assets.CombatACPrefix)
				var damage int
				if roll >= ac {
					dice, err := combat.ParseDice(diceExpr)
					if err != nil {
						dice = combat.DiceRoll{Count: 1, Sides: 4}
					}
					damage = dealDamage(e.combat, actor, target, dice.Roll(), dmgType)
				}
				result.ActorLines = append(result.ActorLines, combat.HitMsgActor(targetName, damage))
				result.TargetLines = append(result.TargetLines, combat.HitMsgTarget(actorName, damage))
				result.RoomLines = append(result.RoomLines, combat.HitMsgRoom(actorName, targetName, damage))
			}
			return nil
		}
		return nil
	}
}

// damageEffect applies damage to the primary target and initiates combat if the target is a mob.
//
// Config fields:
//   - "amount" (string, required): flat integer or dice expression (e.g. "25", "2d6+3").
//   - "damage_types" (comma-separated string, optional): damage type tags (e.g. "fire,ice").
type damageEffect struct {
	combat CombatManager
}

func (e *damageEffect) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeMobile | targetTypePlayer, Required: true},
		},
	}
}

func (e *damageEffect) ValidateConfig(config map[string]string) error {
	amount := config["amount"]
	if amount == "" {
		return fmt.Errorf("amount config required")
	}
	if _, err := combat.ParseDice(amount); err != nil {
		return fmt.Errorf("amount must be an integer or dice expression: %w", err)
	}
	return nil
}

func (e *damageEffect) Create(_ string, config map[string]string, targets []assets.TargetSpec) EffectFunc {
	dice, _ := combat.ParseDice(config["amount"])

	var damageTypes []string
	if dt := config["damage_types"]; dt != "" {
		damageTypes = strings.Split(dt, ",")
	}
	primaryType := assets.DamageTypeUntyped
	if len(damageTypes) > 0 {
		primaryType = damageTypes[0]
	}

	return func(actor shared.Actor, resolved map[string]*TargetRef, _ *AbilityResult) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
		}

		raw := dice.Roll()

		for _, spec := range targets {
			ref := resolved[spec.Name]
			if ref == nil {
				continue
			}
			dealDamage(e.combat, actor, ref.Actor.Actor(), raw, primaryType)
			return nil
		}

		return nil
	}
}

// dealDamage applies raw damage of the given type to a target, handling CalcDamage,
// reflected damage, combat initiation, and threat. Returns the final damage dealt.
func dealDamage(cm CombatManager, actor shared.Actor, target shared.Actor, raw int, dmgType string) int {
	damage, reflected := combat.CalcDamage(raw, dmgType, actor, target)
	target.AdjustResource(assets.ResourceHp, -damage, false)
	if reflected > 0 {
		actor.AdjustResource(assets.ResourceHp, -reflected, false)
	}
	_ = cm.StartCombat(actor, target)
	cm.AddThreat(actor, target, damage)
	return damage
}

// aoeDamageEffect applies damage to enemies in the caster's room. Players hit
// mobs; mobs hit players. The "hit_allies" config extends targeting to same-side.
//
// Config fields:
//   - "amount" (string, required): flat integer or dice expression before modifiers.
//   - "damage_type" (string, optional): damage type tag. Defaults to untyped.
//   - "hit_allies" ("true"/"false", optional): also damage same-side targets. Default false.
type aoeDamageEffect struct {
	combat CombatManager
	world  ZoneLocator
}

func (e *aoeDamageEffect) Spec() *HandlerSpec { return nil }

func (e *aoeDamageEffect) ValidateConfig(config map[string]string) error {
	amount := config["amount"]
	if amount == "" {
		return fmt.Errorf("amount config required")
	}
	if _, err := combat.ParseDice(amount); err != nil {
		return fmt.Errorf("amount must be an integer or dice expression: %w", err)
	}
	return nil
}

func (e *aoeDamageEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	dice, _ := combat.ParseDice(config["amount"])
	dmgType := config["damage_type"]
	if dmgType == "" {
		dmgType = assets.DamageTypeUntyped
	}
	hitAllies := config["hit_allies"] == "true"

	return func(actor shared.Actor, _ map[string]*TargetRef, _ *AbilityResult) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
		}

		zoneId, roomId := actor.Location()
		zi := e.world.GetZone(zoneId)
		if zi == nil {
			return nil
		}
		ri := zi.GetRoom(roomId)
		if ri == nil {
			return nil
		}

		actorId := actor.Id()
		isChar := actor.IsCharacter()

		// Hit enemies: players hit mobs, mobs hit players.
		if isChar {
			ri.ForEachMob(func(mi *game.MobileInstance) {
				dealDamage(e.combat, actor, mi, dice.Roll(), dmgType)
			})
		} else {
			ri.ForEachPlayer(func(charId string, ci *game.CharacterInstance) {
				dealDamage(e.combat, actor, ci, dice.Roll(), dmgType)
			})
		}

		// Optionally hit allies (skip self).
		if hitAllies {
			if isChar {
				ri.ForEachPlayer(func(charId string, ci *game.CharacterInstance) {
					if charId == actorId {
						return
					}
					dealDamage(e.combat, actor, ci, dice.Roll(), dmgType)
				})
			} else {
				ri.ForEachMob(func(mi *game.MobileInstance) {
					if mi.Id() == actorId {
						return
					}
					dealDamage(e.combat, actor, mi, dice.Roll(), dmgType)
				})
			}
		}

		return nil
	}
}
