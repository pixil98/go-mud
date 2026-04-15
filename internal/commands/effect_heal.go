package commands

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/game"
)

// groupHealEffect heals every member of the caster's group who is present in
// the caster's room. A solo caster (not in a group) heals only themselves.
//
// Config fields:
//   - "amount" (string, required): flat integer or dice expression.
//   - "overheal" ("true"/"false", optional): allow healing above max HP. Default false.
type groupHealEffect struct{}

func (e *groupHealEffect) Spec() *HandlerSpec { return nil }

func (e *groupHealEffect) ValidateConfig(config map[string]string) error {
	amount := config["amount"]
	if amount == "" {
		return fmt.Errorf("amount config required")
	}
	if _, err := combat.ParseDice(amount); err != nil {
		return fmt.Errorf("amount must be an integer or dice expression: %w", err)
	}
	return nil
}

func (e *groupHealEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	dice, _ := combat.ParseDice(config["amount"])
	overheal := config["overheal"] == "true"

	return func(actor game.Actor, _ map[string]*TargetRef, _ *AbilityResult) error {
		ri := actor.Room()
		if ri == nil {
			return nil
		}

		var occupants []game.Actor
		ri.ForEachActor(func(a game.Actor) { occupants = append(occupants, a) })

		heal := func(target game.Actor) {
			healAmount := dice.Roll()
			target.AdjustResource(assets.ResourceHp, healAmount, overheal)
			combat.NotifyHeal(actor, target, healAmount/2, occupants)
		}

		root := game.GroupLeader(actor)
		if root == nil {
			root = actor
		}
		game.WalkGroup(root, func(member game.Actor) {
			if member.Room() == ri {
				heal(member)
			}
		})
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

		var occupants []game.Actor
		if ri := actor.Room(); ri != nil {
			ri.ForEachActor(func(a game.Actor) { occupants = append(occupants, a) })
		}
		combat.NotifyHeal(actor, target, healAmount/2, occupants)
		return nil
	}
}
