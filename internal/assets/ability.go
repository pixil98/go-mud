package assets

import (
	"errors"
	"fmt"
	"strings"
)

// EffectSpec pairs an effect type with its config in the ability JSON.
type EffectSpec struct {
	Type   string            `json:"type"`
	Config map[string]string `json:"config,omitempty"`
}

// Ability defines an ability (spell, skill, etc.) loaded from asset files.
// Each ability auto-registers its embedded Command as a top-level command.
// Messages live in Command.Config as message_actor, message_target, message_room.
type Ability struct {
	Effects []EffectSpec `json:"effects"` // effect handlers to execute in order, each with its own config
	Command Command      `json:"command"` // inputs, targets, description, and all config including messages
}

// Validate checks that the ability has at least one effect and valid command inputs.
func (a *Ability) Validate() error {
	var errs []error

	if len(a.Effects) == 0 {
		errs = append(errs, fmt.Errorf("at least one effect is required"))
	}
	for i, e := range a.Effects {
		if e.Type == "" {
			errs = append(errs, fmt.Errorf("effect %d: type is required", i))
		}
	}

	// Validate embedded command. The handler is set by registerAbility at
	// compile time, so we use ValidateInputsTargets to skip the handler check.
	if err := a.Command.ValidateInputsTargets(); err != nil {
		errs = append(errs, fmt.Errorf("command: %w", err))
	}

	return errors.Join(errs...)
}

// Help returns a formatted help string including ability-specific cost info.
func (a *Ability) Help(name string) string {
	base := a.Command.Help(name)

	var costs []string
	if cost := a.Command.Config["ap_cost"]; cost != "" && cost != "0" {
		costs = append(costs, fmt.Sprintf("%s AP", cost))
	}
	if resource := a.Command.Config["resource"]; resource != "" {
		if cost := a.Command.Config["resource_cost"]; cost != "" {
			costs = append(costs, fmt.Sprintf("%s %s", cost, resource))
		}
	}
	if len(costs) > 0 {
		base += "\nCost: " + strings.Join(costs, ", ")
	}

	return base
}

// HelpSummary returns the category and description for help listing.
func (a *Ability) HelpSummary() (string, string) {
	return a.Command.HelpSummary()
}
