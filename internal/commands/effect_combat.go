package commands

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/game"
)

// attackEffect reads the actor's attack grants and performs one attack roll per
// grant. Each hit delegates to dealDamage for damage application and threat.
type attackEffect struct{}

func (e *attackEffect) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeMobile, Required: true},
		},
	}
}

func (e *attackEffect) ValidateConfig(_ map[string]string) error { return nil }

func (e *attackEffect) Create(_ string, _ map[string]string, targets []assets.TargetSpec) EffectFunc {
	return func(actor game.Actor, resolved map[string][]*TargetRef, result *AbilityResult) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
		}
		for _, spec := range targets {
			for _, ref := range resolved[spec.Name] {
				if ref.Actor == nil {
					continue
				}
				target := ref.Actor.Actor()
				if err := combat.StartCombat(actor, target); err != nil {
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
						damage = dealDamage(actor, target, dice.Roll(), dmgType)
					}
					result.ActorLines = append(result.ActorLines, combat.HitMsgActor(targetName, damage))
					result.TargetLines = append(result.TargetLines, combat.HitMsgTarget(actorName, damage))
					result.RoomLines = append(result.RoomLines, combat.HitMsgRoom(actorName, targetName, damage))
				}
			}
		}
		return nil
	}
}

// damageEffect applies damage to the primary target and initiates combat if the target is a mob.
//
// Config fields:
//   - "amount" (string, required): flat integer or dice expression (e.g. "25", "2d6+3").
//   - "damage_types" (comma-separated string, optional): damage type tags (e.g. "fire,ice").
type damageEffect struct{}

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

	return func(actor game.Actor, resolved map[string][]*TargetRef, _ *AbilityResult) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
		}

		for _, spec := range targets {
			for _, ref := range resolved[spec.Name] {
				if ref.Actor == nil {
					continue
				}
				dealDamage(actor, ref.Actor.Actor(), dice.Roll(), primaryType)
			}
		}
		return nil
	}
}

// dealDamage applies raw damage of the given type to a target, handling CalcDamage,
// reflected damage, combat initiation, and threat. Returns the final damage dealt.
func dealDamage(actor, target game.Actor, raw int, dmgType string) int {
	damage, reflected := combat.CalcDamage(raw, dmgType, actor, target)
	target.AdjustResource(assets.ResourceHp, -damage, false)
	if reflected > 0 {
		actor.AdjustResource(assets.ResourceHp, -reflected, false)
	}
	_ = combat.StartCombat(actor, target)
	combat.AddThreat(actor, target, damage)
	return damage
}

