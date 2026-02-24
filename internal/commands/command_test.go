package commands

import (
	"strings"
	"testing"
)

func TestCommand_Validate(t *testing.T) {
	tests := map[string]struct {
		cmd    Command
		expErr string
	}{
		"empty handler": {
			cmd:    Command{},
			expErr: "command handler not set",
		},
		"valid command with no inputs": {
			cmd: Command{
				Handler: "quit",
			},
			expErr: "",
		},
		"valid command with inputs": {
			cmd: Command{
				Handler: "say",
				Inputs: []InputSpec{
					{Name: "text", Type: InputTypeString, Required: true},
				},
			},
			expErr: "",
		},
		"input missing name": {
			cmd: Command{
				Handler: "test",
				Inputs: []InputSpec{
					{Type: InputTypeString},
				},
			},
			expErr: "input 0: name is required",
		},
		"input missing type": {
			cmd: Command{
				Handler: "test",
				Inputs: []InputSpec{
					{Name: "foo"},
				},
			},
			expErr: `input "foo": type is required`,
		},
		"input unknown type": {
			cmd: Command{
				Handler: "test",
				Inputs: []InputSpec{
					{Name: "foo", Type: "bogus"},
				},
			},
			expErr: `input "foo": unknown type "bogus"`,
		},
		"rest input not last": {
			cmd: Command{
				Handler: "test",
				Inputs: []InputSpec{
					{Name: "first", Type: InputTypeString, Rest: true},
					{Name: "second", Type: InputTypeString},
				},
			},
			expErr: `input "first": only the last input can have rest=true`,
		},
		"rest input at end is valid": {
			cmd: Command{
				Handler: "say",
				Inputs: []InputSpec{
					{Name: "target", Type: InputTypeString, Required: true},
					{Name: "text", Type: InputTypeString, Required: true, Rest: true},
				},
			},
			expErr: "",
		},
		"target missing name": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Types: []string{"player"}, Input: "target"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: "target 0: name is required",
		},
		"target missing type": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Input: "target"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: `target "target": type is required`,
		},
		"target unknown type": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Types: []string{"bogus"}, Input: "target"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: `target "target": unknown types [bogus]`,
		},
		"target missing input": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Types: []string{"player"}}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: `target "target": input is required`,
		},
		"target input does not exist": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Types: []string{"player"}, Input: "nonexistent"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: `target "target": input "nonexistent" does not exist in inputs`,
		},
		"target unknown scope": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Types: []string{"player"}, Scopes: []string{"bogus"}, Input: "target"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: `target "target": unknown scopes [bogus]`,
		},
		"valid target": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "target"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: "",
		},
		"valid exit target": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Types: []string{"exit"}, Scopes: []string{"room"}, Input: "direction"}},
				Inputs:  []InputSpec{{Name: "direction", Type: InputTypeString}},
			},
			expErr: "",
		},
		"valid combined object and exit target": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Types: []string{"object", "exit"}, Scopes: []string{"room"}, Input: "target"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: "",
		},
		"valid scope_target": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{
					{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from", Optional: true},
					{Name: "target", Types: []string{"object"}, Scopes: []string{"room", "contents"}, Input: "item", ScopeTarget: "container"},
				},
				Inputs: []InputSpec{
					{Name: "item", Type: InputTypeString, Required: true},
					{Name: "from", Type: InputTypeString},
				},
			},
			expErr: "",
		},
		"scope_target requires contents scope": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{
					{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from"},
					{Name: "target", Types: []string{"object"}, Scopes: []string{"room"}, Input: "item", ScopeTarget: "container"},
				},
				Inputs: []InputSpec{
					{Name: "item", Type: InputTypeString},
					{Name: "from", Type: InputTypeString},
				},
			},
			expErr: `target "target": scope_target requires "contents" in scope`,
		},
		"contents scope requires scope_target": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{
					{Name: "target", Types: []string{"object"}, Scopes: []string{"contents"}, Input: "item"},
				},
				Inputs: []InputSpec{
					{Name: "item", Type: InputTypeString},
				},
			},
			expErr: `target "target": "contents" scope requires scope_target to be set`,
		},
		"scope_target must reference earlier target": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{
					{Name: "target", Types: []string{"object"}, Scopes: []string{"contents"}, Input: "item", ScopeTarget: "container"},
				},
				Inputs: []InputSpec{
					{Name: "item", Type: InputTypeString},
				},
			},
			expErr: `target "target": scope_target "container" must reference a target declared earlier in the targets array`,
		},
		"scope_target only for object targets": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{
					{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from"},
					{Name: "target", Types: []string{"player"}, Scopes: []string{"contents"}, Input: "item", ScopeTarget: "container"},
				},
				Inputs: []InputSpec{
					{Name: "item", Type: InputTypeString},
					{Name: "from", Type: InputTypeString},
				},
			},
			expErr: `target "target": scope_target is only supported for object targets`,
		},
		"valid not_found template": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Types: []string{"player"}, Scopes: []string{"room"}, Input: "who", NotFound: "You don't see '{{ .Inputs.who }}' here."}},
				Inputs:  []InputSpec{{Name: "who", Type: InputTypeString}},
			},
			expErr: "",
		},
		"invalid not_found template": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Types: []string{"player"}, Scopes: []string{"room"}, Input: "who", NotFound: "{{ .Bad }"}},
				Inputs:  []InputSpec{{Name: "who", Type: InputTypeString}},
			},
			expErr: `target "target": invalid not_found template:`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.cmd.Validate()

			if tt.expErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.expErr)
				return
			}

			if !strings.Contains(err.Error(), tt.expErr) {
				t.Errorf("error = %q, expected to contain %q", err.Error(), tt.expErr)
			}
		})
	}
}

func TestTargetSpec_Scope(t *testing.T) {
	tests := map[string]struct {
		scopes []string
		exp    Scope
	}{
		"room":                 {scopes: []string{"room"}, exp: ScopeRoom},
		"inventory":            {scopes: []string{"inventory"}, exp: ScopeInventory},
		"world":                {scopes: []string{"world"}, exp: ScopeWorld},
		"zone":                 {scopes: []string{"zone"}, exp: ScopeZone},
		"Room uppercase":       {scopes: []string{"ROOM"}, exp: ScopeRoom},
		"World mixed case":     {scopes: []string{"World"}, exp: ScopeWorld},
		"unknown returns zero": {scopes: []string{"unknown"}, exp: 0},
		"empty returns zero":   {scopes: []string{}, exp: 0},
		"nil returns zero":     {scopes: nil, exp: 0},
		"room and inventory":   {scopes: []string{"room", "inventory"}, exp: ScopeRoom | ScopeInventory},
		"world and zone":       {scopes: []string{"world", "zone"}, exp: ScopeWorld | ScopeZone},
		"equipment":            {scopes: []string{"equipment"}, exp: ScopeEquipment},
		"contents":             {scopes: []string{"contents"}, exp: ScopeContents},
		"all scopes":           {scopes: []string{"room", "inventory", "equipment", "contents", "world", "zone"}, exp: ScopeRoom | ScopeInventory | ScopeEquipment | ScopeContents | ScopeWorld | ScopeZone},
		"mixed with unknown":   {scopes: []string{"room", "bogus"}, exp: ScopeRoom},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			spec := TargetSpec{Scopes: tt.scopes}
			got := spec.Scope()
			if got != tt.exp {
				t.Errorf("TargetSpec{Scopes: %v}.Scope() = %d, expected %d", tt.scopes, got, tt.exp)
			}
		})
	}
}

func TestTargetType_Label(t *testing.T) {
	tests := map[string]struct {
		tt  TargetType
		exp string
	}{
		"player":   {TargetTypePlayer, "Player"},
		"mobile":   {TargetTypeMobile, "Mobile"},
		"object":   {TargetTypeObject, "Object"},
		"exit":     {TargetTypeExit, "Exit"},
		"combined": {TargetTypePlayer | TargetTypeMobile, "Target"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.tt.Label(); got != tt.exp {
				t.Errorf("Label() = %q, expected %q", got, tt.exp)
			}
		})
	}
}

func TestTargetType_String(t *testing.T) {
	tests := map[string]struct {
		tt  TargetType
		exp string
	}{
		"player":   {TargetTypePlayer, "player"},
		"mobile":   {TargetTypeMobile, "mobile"},
		"object":   {TargetTypeObject, "object"},
		"exit":     {TargetTypeExit, "exit"},
		"combined": {TargetTypePlayer | TargetTypeMobile, "target"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.tt.String(); got != tt.exp {
				t.Errorf("String() = %q, expected %q", got, tt.exp)
			}
		})
	}
}
