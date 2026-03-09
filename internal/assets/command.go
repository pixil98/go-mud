package assets

import (
	"fmt"
	"slices"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// Input type constants.
const (
	InputTypeString = "string"
	InputTypeNumber = "number"
)

// Target type constants.
const (
	TargetPlayer = "player"
	TargetMobile = "mobile"
	TargetObject = "object"
	TargetExit   = "exit"
)

// Scope constants.
const (
	ScopeRoom      = "room"
	ScopeInventory = "inventory"
	ScopeEquipment = "equipment"
	ScopeWorld     = "world"
	ScopeZone      = "zone"
	ScopeContents  = "contents"
	ScopeGroup     = "group"
)

var (
	validInputTypes  = []string{InputTypeString, InputTypeNumber}
	validTargetTypes = []string{TargetPlayer, TargetMobile, TargetObject, TargetExit}
	validScopes      = []string{ScopeRoom, ScopeInventory, ScopeEquipment, ScopeWorld, ScopeZone, ScopeContents, ScopeGroup}
)

// InputSpec defines an input parameter that a command accepts from user input.
type InputSpec struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Rest     bool   `json:"rest"`              // If true, captures all remaining input
	Missing  string `json:"missing,omitempty"` // Custom error when required input is absent (e.g., "Get what?")
}

// TargetSpec defines a target to be resolved at runtime.
// When ScopeTarget is set, the referenced target must appear earlier in the targets
// array so it is resolved first. If the referenced target resolved to an object with
// contents, this target is resolved exclusively from those contents.
type TargetSpec struct {
	Name        string   `json:"name"`                   // Name to access in templates (e.g., "target" -> .Targets.target)
	Types       []string `json:"types"`                  // Entity types: player, mobile, object, exit
	Scopes      []string `json:"scopes,omitempty"`       // Resolution scopes: room, world, zone, inventory, equipment, contents, group
	Input       string   `json:"input"`                  // Which input provides the name to resolve
	Optional    bool     `json:"optional,omitempty"`     // If true, missing input -> nil (no error)
	ScopeTarget string   `json:"scope_target,omitempty"` // Resolve inside this target's contents when present; falls back to normal scopes
	NotFound    string   `json:"not_found,omitempty"`    // Custom template for "not found" error; supports {{ .Input }}
}

// Command defines a command loaded from JSON.
type Command struct {
	Handler     string            `json:"handler"`
	Category    string            `json:"category,omitempty"`    // Grouping for help display
	Description string            `json:"description,omitempty"` // Short description for help
	Priority    int               `json:"priority,omitempty"`    // Higher values win prefix-match ties (default 0)
	Aliases     []string          `json:"aliases,omitempty"`     // Alternative names that resolve to this command (e.g., "nw" for "northwest")
	Config      map[string]string `json:"config"`                // Config passed to handler, may contain templates
	Targets     []TargetSpec      `json:"targets"`               // Targets to resolve at runtime
	Inputs      []InputSpec       `json:"inputs"`                // User input parameters
}

func (c *Command) Validate() error {
	if c.Handler == "" {
		return fmt.Errorf("command handler not set")
	}
	return c.ValidateInputsTargets()
}

// ValidateInputsTargets validates inputs and targets without requiring a handler.
// Used by Ability.Validate where the handler is set at compile time.
func (c *Command) ValidateInputsTargets() error {
	for i, input := range c.Inputs {
		if input.Name == "" {
			return fmt.Errorf("input %d: name is required", i)
		}
		if input.Type == "" {
			return fmt.Errorf("input %q: type is required", input.Name)
		}
		if !slices.Contains(validInputTypes, input.Type) {
			return fmt.Errorf("input %q: unknown type %q", input.Name, input.Type)
		}
		if input.Rest && i != len(c.Inputs)-1 {
			return fmt.Errorf("input %q: only the last input can have rest=true", input.Name)
		}
	}

	validInputs := make(map[string]bool)
	for _, input := range c.Inputs {
		validInputs[input.Name] = true
	}

	for i, target := range c.Targets {
		if target.Name == "" {
			return fmt.Errorf("target %d: name is required", i)
		}
		if len(target.Types) == 0 {
			return fmt.Errorf("target %q: type is required", target.Name)
		}
		for _, t := range target.Types {
			if !slices.Contains(validTargetTypes, t) {
				return fmt.Errorf("target %q: unknown type %q", target.Name, t)
			}
		}
		if target.Input == "" {
			return fmt.Errorf("target %q: input is required", target.Name)
		}
		if !validInputs[target.Input] {
			return fmt.Errorf("target %q: input %q does not exist in inputs", target.Name, target.Input)
		}
		for _, s := range target.Scopes {
			if !slices.Contains(validScopes, s) {
				return fmt.Errorf("target %q: unknown scope %q", target.Name, s)
			}
		}
		hasContents := slices.Contains(target.Scopes, ScopeContents)
		if target.ScopeTarget != "" {
			if len(target.Types) != 1 || target.Types[0] != TargetObject {
				return fmt.Errorf("target %q: scope_target is only supported for object targets", target.Name)
			}
			if !hasContents {
				return fmt.Errorf("target %q: scope_target requires \"contents\" in scope", target.Name)
			}
			found := false
			for j := 0; j < i; j++ {
				if c.Targets[j].Name == target.ScopeTarget {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("target %q: scope_target %q must reference a target declared earlier in the targets array", target.Name, target.ScopeTarget)
			}
		} else if hasContents {
			return fmt.Errorf("target %q: \"contents\" scope requires scope_target to be set", target.Name)
		}

		if target.NotFound != "" {
			if _, err := template.New("").Funcs(sprig.TxtFuncMap()).Parse(target.NotFound); err != nil {
				return fmt.Errorf("target %q: invalid not_found template: %w", target.Name, err)
			}
		}
	}

	return nil
}
