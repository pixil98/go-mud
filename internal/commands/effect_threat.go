package commands

import (
	"fmt"
	"strconv"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/combat"
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

// threatEffect modifies threat tables on combat targets.
//
// Config fields:
//   - "mode" (string, required): one of "add", "set_to_top", "set_to_value".
//   - "amount" (int string): threat delta for "add", absolute value for "set_to_value".
//     Required for "add" and "set_to_value" modes; ignored by "set_to_top".
type threatEffect struct{}

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
	inCombatOnly := config["in_combat_only"] == "true"

	return func(actor game.Actor, resolved map[string][]*TargetRef, _ *AbilityResult) error {
		for _, ref := range resolved["target"] {
			if ref.Actor == nil {
				continue
			}
			target := ref.Actor.Actor()

			if inCombatOnly && !target.IsInCombat() {
				continue
			}

			if err := combat.StartCombat(actor, target); err != nil {
				continue
			}

			switch mode {
			case ThreatModeAdd:
				combat.AddThreat(actor, target, amount)
			case ThreatModeSetToTop:
				combat.TopThreat(actor, target)
			case ThreatModeSetToValue:
				combat.SetThreat(actor, target, amount)
			}
		}
		return nil
	}
}
