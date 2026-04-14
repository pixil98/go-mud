package commands

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// Threat effect mode constants for the "mode" config field.
const (
	ThreatModeAdd        = "add"
	ThreatModeSetToTop   = "set_to_top"
	ThreatModeSetToValue = "set_to_value"
)

// validateThreatConfig checks the mode/amount fields shared by threat effects.
func validateThreatConfig(config map[string]string) error {
	mode := config["mode"]
	switch mode {
	case ThreatModeAdd, ThreatModeSetToValue:
		if config["amount"] == "" {
			return fmt.Errorf("amount config required for mode %q", mode)
		}
		if _, err := strconv.Atoi(config["amount"]); err != nil {
			return fmt.Errorf("amount must be an integer: %w", err)
		}
	case ThreatModeSetToTop:
		// No amount needed.
	default:
		return fmt.Errorf("unknown threat mode %q", mode)
	}
	return nil
}

// aoeThreatEffect modifies threat tables on all mobs in the caster's room.
//
// Config fields:
//   - "mode" (string, required): one of "add", "set_to_top", "set_to_value".
//   - "amount" (int string): threat delta for "add", absolute value for "set_to_value".
//     Required for "add" and "set_to_value" modes; ignored by "set_to_top".
//   - "in_combat_only" ("true"/"false", optional): only affect mobs already in combat. Default false.
type aoeThreatEffect struct {
	combat CombatManager
}

func (e *aoeThreatEffect) Spec() *HandlerSpec { return nil }

func (e *aoeThreatEffect) ValidateConfig(config map[string]string) error {
	if err := validateThreatConfig(config); err != nil {
		return err
	}
	if v := config["in_combat_only"]; v != "" && v != "true" && v != "false" {
		return fmt.Errorf("in_combat_only must be \"true\" or \"false\", got %q", v)
	}
	return nil
}

func (e *aoeThreatEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	mode := config["mode"]
	amount, _ := strconv.Atoi(config["amount"])
	inCombatOnly := config["in_combat_only"] == "true"

	applyThreat := func(actor game.Actor, target game.Actor) {
		if inCombatOnly && !target.IsInCombat() {
			return
		}
		if err := e.combat.StartCombat(actor, target); err != nil {
			slog.Warn("aoe_threat: StartCombat failed", "target", target.Id(), "error", err)
			return
		}
		switch mode {
		case ThreatModeAdd:
			e.combat.AddThreat(actor, target, amount)
		case ThreatModeSetToTop:
			e.combat.TopThreat(actor, target)
		case ThreatModeSetToValue:
			e.combat.SetThreat(actor, target, amount)
		}
	}

	return func(actor game.Actor, _ map[string]*TargetRef, _ *AbilityResult) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
		}

		ri := actor.Room()
		if ri == nil {
			return nil
		}

		ri.ForEachMob(func(mi *game.MobileInstance) {
			applyThreat(actor, mi)
		})

		return nil
	}
}

// threatEffect modifies threat tables on combat targets.
//
// Config fields:
//   - "mode" (string, required): one of "add", "set_to_top", "set_to_value".
//   - "amount" (int string): threat delta for "add", absolute value for "set_to_value".
//     Required for "add" and "set_to_value" modes; ignored by "set_to_top".
type threatEffect struct {
	combat CombatManager
}

func (e *threatEffect) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeMobile | targetTypePlayer, Required: true},
		},
	}
}

func (e *threatEffect) ValidateConfig(config map[string]string) error {
	return validateThreatConfig(config)
}

func (e *threatEffect) Create(_ string, config map[string]string, targets []assets.TargetSpec) EffectFunc {
	mode := config["mode"]
	amount, _ := strconv.Atoi(config["amount"])

	return func(actor game.Actor, resolved map[string]*TargetRef, _ *AbilityResult) error {
		ref := resolved["target"]
		if ref == nil || ref.Actor == nil {
			return nil
		}
		target := ref.Actor.Actor()

		// Ensure the caster is in combat with the target.
		if err := e.combat.StartCombat(actor, target); err != nil {
			return NewUserError(err.Error())
		}

		switch mode {
		case ThreatModeAdd:
			e.combat.AddThreat(actor, target, amount)
		case ThreatModeSetToTop:
			e.combat.TopThreat(actor, target)
		case ThreatModeSetToValue:
			e.combat.SetThreat(actor, target, amount)
		}

		return nil
	}
}
