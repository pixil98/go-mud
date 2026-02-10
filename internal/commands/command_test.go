package commands

import (
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
				Targets: []TargetSpec{{Type: "player", Input: "target"}},
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
				Targets: []TargetSpec{{Name: "target", Type: "bogus", Input: "target"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: `target "target": unknown type "bogus"`,
		},
		"target missing input": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Type: "player"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: `target "target": input is required`,
		},
		"target input does not exist": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Type: "player", Input: "nonexistent"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: `target "target": input "nonexistent" does not exist in inputs`,
		},
		"target unknown scope": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Type: "player", Scopes: []string{"bogus"}, Input: "target"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: `target "target": unknown scopes [bogus]`,
		},
		"valid target": {
			cmd: Command{
				Handler: "test",
				Targets: []TargetSpec{{Name: "target", Type: "player", Scopes: []string{"world"}, Input: "target"}},
				Inputs:  []InputSpec{{Name: "target", Type: InputTypeString}},
			},
			expErr: "",
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

			if err.Error() != tt.expErr {
				t.Errorf("error = %q, expected %q", err.Error(), tt.expErr)
			}
		})
	}
}

func TestTargetSpec_Scope(t *testing.T) {
	tests := map[string]struct {
		scopes []string
		exp    Scope
	}{
		"room":                   {scopes: []string{"room"}, exp: ScopeRoom},
		"inventory":              {scopes: []string{"inventory"}, exp: ScopeInventory},
		"world":                  {scopes: []string{"world"}, exp: ScopeWorld},
		"zone":                   {scopes: []string{"zone"}, exp: ScopeZone},
		"Room uppercase":         {scopes: []string{"ROOM"}, exp: ScopeRoom},
		"World mixed case":       {scopes: []string{"World"}, exp: ScopeWorld},
		"unknown returns zero":   {scopes: []string{"unknown"}, exp: 0},
		"empty returns zero":     {scopes: []string{}, exp: 0},
		"nil returns zero":       {scopes: nil, exp: 0},
		"room and inventory":     {scopes: []string{"room", "inventory"}, exp: ScopeRoom | ScopeInventory},
		"world and zone":         {scopes: []string{"world", "zone"}, exp: ScopeWorld | ScopeZone},
		"all scopes":             {scopes: []string{"room", "inventory", "world", "zone"}, exp: ScopeRoom | ScopeInventory | ScopeWorld | ScopeZone},
		"mixed with unknown":     {scopes: []string{"room", "bogus"}, exp: ScopeRoom},
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
