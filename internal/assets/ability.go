package assets

import (
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pixil98/go-errors"
)

// Ability type constants.
const (
	AbilityTypeSpell = "spell"
	AbilityTypeSkill = "skill"
)



// Ability defines a spell or skill loaded from asset files.
// Both types use an embedded Command for input parsing and target resolution.
// Skills auto-register their Command as a top-level command.
// Spells are invoked via "cast"; the cast handler uses the Command's inputs
// and targets to parse remaining args and resolve targets.
type Ability struct {
	Name         string          `json:"name"`
	Type         string          `json:"type"`                    // "spell" or "skill"
	Description  string          `json:"description,omitempty"`
	Resource     string          `json:"resource,omitempty"`      // pool name: "mana", "stamina"
	ResourceCost int             `json:"resource_cost,omitempty"`
	APCost       int             `json:"ap_cost,omitempty"`       // action points consumed; 0 treated as 1
	Command      Command         `json:"command"`                 // inputs and targets for resolution
	Handler      string          `json:"handler"`                 // effect handler name (e.g. "damage")
	Config       map[string]any  `json:"config,omitempty"`        // handler-specific config (includes *_key fields for key_mod integration)
	Messages     AbilityMessages `json:"messages,omitempty"`
}

// AbilityMessages holds message templates for ability effects.
type AbilityMessages struct {
	Actor  string `json:"actor,omitempty"`  // what the caster sees
	Target string `json:"target,omitempty"` // what the target sees
	Room   string `json:"room,omitempty"`   // what bystanders see
}

func (a *Ability) Validate() error {
	el := errors.NewErrorList()

	if a.Name == "" {
		el.Add(fmt.Errorf("name is required"))
	}
	if a.Type == "" {
		el.Add(fmt.Errorf("type is required"))
	} else if a.Type != AbilityTypeSpell && a.Type != AbilityTypeSkill {
		el.Add(fmt.Errorf("type must be %q or %q", AbilityTypeSpell, AbilityTypeSkill))
	}
	if a.Handler == "" {
		el.Add(fmt.Errorf("handler is required"))
	}

	// Resource invariants
	if a.ResourceCost > 0 && a.Resource == "" {
		el.Add(fmt.Errorf("resource_cost requires resource"))
	}
	if a.ResourceCost < 0 {
		el.Add(fmt.Errorf("resource_cost must not be negative"))
	}
	if a.APCost < 0 {
		el.Add(fmt.Errorf("ap_cost must not be negative"))
	}

	// Validate embedded command — use ability's handler since the command's
	// handler is set at registration time, not in the ability JSON.
	cmd := a.Command
	cmd.Handler = a.Handler
	if err := cmd.Validate(); err != nil {
		el.Add(fmt.Errorf("command: %w", err))
	}

	if a.Messages.Actor == "" && a.Messages.Target == "" && a.Messages.Room == "" {
		el.Add(fmt.Errorf("at least one message is required"))
	}
	el.Add(validateTemplate("messages.actor", a.Messages.Actor))
	el.Add(validateTemplate("messages.target", a.Messages.Target))
	el.Add(validateTemplate("messages.room", a.Messages.Room))

	return el.Err()
}

// validateTemplate checks template syntax. Returns nil for empty strings.
func validateTemplate(field, tmpl string) error {
	if tmpl == "" {
		return nil
	}
	if _, err := template.New("").Funcs(sprig.TxtFuncMap()).Parse(tmpl); err != nil {
		return fmt.Errorf("%s: invalid template: %w", field, err)
	}
	return nil
}
