package assets

import (
	"fmt"

	"github.com/pixil98/go-errors"
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

func (a *Ability) Validate() error {
	el := errors.NewErrorList()

	if len(a.Effects) == 0 {
		el.Add(fmt.Errorf("at least one effect is required"))
	}
	for i, e := range a.Effects {
		if e.Type == "" {
			el.Add(fmt.Errorf("effect %d: type is required", i))
		}
	}

	// Validate embedded command. The handler is set by registerAbility at
	// compile time, so we use ValidateInputsTargets to skip the handler check.
	if err := a.Command.ValidateInputsTargets(); err != nil {
		el.Add(fmt.Errorf("command: %w", err))
	}

	return el.Err()
}
