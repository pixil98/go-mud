package commands

import "fmt"

// ParamType represents the type of a command parameter.
type ParamType string

const (
	ParamTypeString    ParamType = "string"
	ParamTypeNumber    ParamType = "number"
	ParamTypeDirection ParamType = "direction"
	ParamTypeTarget    ParamType = "target" // Any targetable entity
	ParamTypePlayer    ParamType = "player"
	ParamTypeMob       ParamType = "mob"
	ParamTypeItem      ParamType = "item"
)

// ParamSpec defines a parameter that a command accepts.
type ParamSpec struct {
	Name     string    `json:"name"`
	Type     ParamType `json:"type"`
	Required bool      `json:"required"`
	Rest     bool      `json:"rest"` // If true, captures all remaining input
}

// Command defines a command loaded from JSON.
type Command struct {
	Handler string         `json:"handler"`
	Config  map[string]any `json:"config"` // Static config passed to handler factory
	Params  []ParamSpec    `json:"params"`
}

func (c *Command) Validate() error {
	if c.Handler == "" {
		return fmt.Errorf("command handler not set")
	}

	for i, p := range c.Params {
		if p.Name == "" {
			return fmt.Errorf("param %d: name is required", i)
		}
		if p.Type == "" {
			return fmt.Errorf("param %q: type is required", p.Name)
		}
		// Only the last param can have rest=true
		if p.Rest && i != len(c.Params)-1 {
			return fmt.Errorf("param %q: only the last parameter can have rest=true", p.Name)
		}
	}

	return nil
}
