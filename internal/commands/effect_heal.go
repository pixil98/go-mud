package commands

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/game"
)

// roomOccupants builds a list of all actors in the room for NotifyHeal.
func roomOccupants(ri *game.RoomInstance) []game.Actor {
	var out []game.Actor
	ri.ForEachMob(func(mi *game.MobileInstance) {
		out = append(out, mi)
	})
	ri.ForEachPlayer(func(_ string, ci *game.CharacterInstance) {
		out = append(out, ci)
	})
	return out
}

// aoeHealEffect heals allies in the caster's room. Players heal players; mobs
// heal mobs. The "hit_enemies" config extends targeting to the opposite side.
//
// Config fields:
//   - "amount" (string, required): flat integer or dice expression.
//   - "overheal" ("true"/"false", optional): allow healing above max HP. Default false.
//   - "hit_enemies" ("true"/"false", optional): also heal opposite-side targets. Default false.
type aoeHealEffect struct{}

func (e *aoeHealEffect) Spec() *HandlerSpec { return nil }

func (e *aoeHealEffect) ValidateConfig(config map[string]string) error {
	amount := config["amount"]
	if amount == "" {
		return fmt.Errorf("amount config required")
	}
	if _, err := combat.ParseDice(amount); err != nil {
		return fmt.Errorf("amount must be an integer or dice expression: %w", err)
	}
	return nil
}

func (e *aoeHealEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	dice, _ := combat.ParseDice(config["amount"])
	overheal := config["overheal"] == "true"
	hitEnemies := config["hit_enemies"] == "true"

	return func(actor game.Actor, _ map[string]*TargetRef, _ *AbilityResult) error {
		ri := actor.Room()
		if ri == nil {
			return nil
		}

		occupants := roomOccupants(ri)

		heal := func(target game.Actor) {
			healAmount := dice.Roll()
			target.AdjustResource(assets.ResourceHp, healAmount, overheal)
			combat.NotifyHeal(actor, target, healAmount/2, occupants)
		}

		isChar := actor.IsCharacter()

		// Heal allies: players heal players, mobs heal mobs.
		if isChar {
			ri.ForEachPlayer(func(_ string, ci *game.CharacterInstance) {
				heal(ci)
			})
		} else {
			ri.ForEachMob(func(mi *game.MobileInstance) {
				heal(mi)
			})
		}

		// Optionally heal enemies.
		if hitEnemies {
			if isChar {
				ri.ForEachMob(func(mi *game.MobileInstance) {
					heal(mi)
				})
			} else {
				ri.ForEachPlayer(func(_ string, ci *game.CharacterInstance) {
					heal(ci)
				})
			}
		}

		return nil
	}
}

// healEffect restores HP to the target and generates threat on all combatants
// fighting the target at half the amount healed.
//
// Config fields:
//   - "amount" (string, required): flat integer or dice expression (e.g. "25", "2d6+3").
//   - "overheal" ("true"/"false", optional): allow healing above max HP. Default false.
type healEffect struct{}

func (e *healEffect) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeActor, Required: true},
		},
	}
}

func (e *healEffect) ValidateConfig(config map[string]string) error {
	amount := config["amount"]
	if amount == "" {
		return fmt.Errorf("amount config required")
	}
	if _, err := combat.ParseDice(amount); err != nil {
		return fmt.Errorf("amount must be an integer or dice expression: %w", err)
	}
	return nil
}

func (e *healEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	dice, _ := combat.ParseDice(config["amount"])
	overheal := config["overheal"] == "true"

	return func(actor game.Actor, resolved map[string]*TargetRef, _ *AbilityResult) error {
		ref := resolved["target"]
		if ref == nil || ref.Actor == nil {
			return nil
		}
		target := ref.Actor.Actor()
		healAmount := dice.Roll()
		target.AdjustResource(assets.ResourceHp, healAmount, overheal)
		combat.NotifyHeal(actor, target, healAmount/2, roomOccupants(actor.Room()))
		return nil
	}
}
