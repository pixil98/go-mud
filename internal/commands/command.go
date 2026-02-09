package commands

import (
	"fmt"
	"strings"
)

// InputType represents the type of a command input parameter.
// Only primitive types are supported - target resolution is handled via $resolve directives.
type InputType string

const (
	InputTypeString InputType = "string" // Text input (single word if rest=false, multi-word if rest=true)
	InputTypeNumber InputType = "number" // Integer
)

// Scope defines where to look for targets. Can be combined with bitwise OR.
type Scope int

const (
	ScopeWorld     Scope = 1 << iota // All online players
	ScopeZone                        // Players in current zone
	ScopeRoom                        // Players/mobs/objects in current room
	ScopeInventory                   // Objects in actor's inventory
)

// InputSpec defines an input parameter that a command accepts from user input.
type InputSpec struct {
	Name     string    `json:"name"`
	Type     InputType `json:"type"`
	Required bool      `json:"required"`
	Rest     bool      `json:"rest"` // If true, captures all remaining input
}

// TargetSpec defines a target to be resolved at runtime.
type TargetSpec struct {
	Name     string   `json:"name"`               // Name to access in templates (e.g., "target" -> .Targets.target)
	Type     string   `json:"type"`               // Entity type: player, mob, object, target (polymorphic)
	Scopes   []string `json:"scope,omitempty"`    // Resolution scopes: room, world, zone, inventory
	Input    string   `json:"input"`              // Which input provides the name to resolve
	Optional bool     `json:"optional,omitempty"` // If true, missing input -> nil (no error)
}

// Scope returns the combined Scope value from Scopes slice.
func (t *TargetSpec) Scope() Scope {
	var result Scope
	for _, s := range t.Scopes {
		switch strings.ToLower(s) {
		case "room":
			result |= ScopeRoom
		case "inventory":
			result |= ScopeInventory
		case "world":
			result |= ScopeWorld
		case "zone":
			result |= ScopeZone
		}
	}
	return result
}

// Command defines a command loaded from JSON.
type Command struct {
	Handler string         `json:"handler"`
	Config  map[string]any `json:"config"`  // Config passed to handler, may contain templates
	Targets []TargetSpec   `json:"targets"` // Targets to resolve at runtime
	Inputs  []InputSpec    `json:"inputs"`  // User input parameters
}

func (c *Command) Validate() error {
	if c.Handler == "" {
		return fmt.Errorf("command handler not set")
	}

	for i, input := range c.Inputs {
		if input.Name == "" {
			return fmt.Errorf("input %d: name is required", i)
		}
		if input.Type == "" {
			return fmt.Errorf("input %q: type is required", input.Name)
		}
		// Validate input type is a known primitive
		switch input.Type {
		case InputTypeString, InputTypeNumber:
			// Valid
		default:
			return fmt.Errorf("input %q: unknown type %q", input.Name, input.Type)
		}
		// Only the last input can have rest=true
		if input.Rest && i != len(c.Inputs)-1 {
			return fmt.Errorf("input %q: only the last input can have rest=true", input.Name)
		}
	}

	// Build set of valid input names for target validation
	validInputs := make(map[string]bool)
	for _, input := range c.Inputs {
		validInputs[input.Name] = true
	}

	for i, target := range c.Targets {
		if target.Name == "" {
			return fmt.Errorf("target %d: name is required", i)
		}
		if target.Type == "" {
			return fmt.Errorf("target %q: type is required", target.Name)
		}
		// Validate target type
		switch target.Type {
		case "player", "mobile", "object", "target":
			// Valid
		default:
			return fmt.Errorf("target %q: unknown type %q", target.Name, target.Type)
		}
		if target.Input == "" {
			return fmt.Errorf("target %q: input is required", target.Name)
		}
		// Validate that target.Input references an existing input
		if !validInputs[target.Input] {
			return fmt.Errorf("target %q: input %q does not exist in inputs", target.Name, target.Input)
		}
		// Validate scopes if provided
		if len(target.Scopes) > 0 && target.Scope() == 0 {
			return fmt.Errorf("target %q: unknown scopes %v", target.Name, target.Scopes)
		}
	}

	return nil
}
