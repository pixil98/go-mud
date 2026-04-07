package assets

import (
	"strings"
	"testing"
)

func TestCommand_Help(t *testing.T) {
	tests := map[string]struct {
		cmd      Command
		name     string
		contains []string
	}{
		"basic": {
			cmd:  Command{Description: "test description"},
			name: "test-cmd",
			contains: []string{
				"TEST-CMD - test description",
			},
		},
		"with aliases": {
			cmd:  Command{Description: "test description", Aliases: []string{"a", "b"}},
			name: "test-cmd",
			contains: []string{
				"Aliases: a, b",
			},
		},
		"with inputs": {
			cmd: Command{
				Description: "test description",
				Inputs: []InputSpec{
					{Name: "target", Type: InputTypeString, Required: true},
					{Name: "amount", Type: InputTypeNumber},
				},
			},
			name: "test-cmd",
			contains: []string{
				"Usage: test-cmd <target> [amount]",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.cmd.Help(tc.name)
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("Help() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

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
				Handler: "test-handler",
			},
		},
		"valid command with inputs": {
			cmd: Command{
				Handler: "test-handler",
				Inputs: []InputSpec{
					{Name: "test-input", Type: InputTypeString, Required: true},
				},
			},
		},
		"input missing name": {
			cmd: Command{
				Handler: "test-handler",
				Inputs:  []InputSpec{{Type: InputTypeString}},
			},
			expErr: "input 0: name is required",
		},
		"input missing type": {
			cmd: Command{
				Handler: "test-handler",
				Inputs:  []InputSpec{{Name: "test-input"}},
			},
			expErr: `input "test-input": type is required`,
		},
		"input unknown type": {
			cmd: Command{
				Handler: "test-handler",
				Inputs:  []InputSpec{{Name: "test-input", Type: "bogus"}},
			},
			expErr: `input "test-input": unknown type "bogus"`,
		},
		"rest input not last": {
			cmd: Command{
				Handler: "test-handler",
				Inputs: []InputSpec{
					{Name: "test-first", Type: InputTypeString, Rest: true},
					{Name: "test-second", Type: InputTypeString},
				},
			},
			expErr: `input "test-first": only the last input can have rest=true`,
		},
		"rest input at end is valid": {
			cmd: Command{
				Handler: "test-handler",
				Inputs: []InputSpec{
					{Name: "test-target", Type: InputTypeString, Required: true},
					{Name: "test-text", Type: InputTypeString, Required: true, Rest: true},
				},
			},
		},
		"target missing name": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Types: []string{TargetPlayer}, Input: "test-target"}},
				Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
			},
			expErr: "target 0: name is required",
		},
		"target missing type": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Input: "test-target"}},
				Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
			},
			expErr: `target "test-target": type is required`,
		},
		"target unknown type": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Types: []string{"bogus"}, Input: "test-target"}},
				Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
			},
			expErr: `target "test-target": unknown type "bogus"`,
		},
		"target missing input": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetPlayer}}},
				Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
			},
			expErr: `target "test-target": input is required`,
		},
		"target input does not exist": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetPlayer}, Input: "nonexistent"}},
				Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
			},
			expErr: `target "test-target": input "nonexistent" does not exist in inputs`,
		},
		"target unknown scope": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetPlayer}, Scopes: []string{"bogus"}, Input: "test-target"}},
				Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
			},
			expErr: `target "test-target": unknown scope "bogus"`,
		},
		"valid target": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetPlayer}, Scopes: []string{ScopeWorld}, Input: "test-target"}},
				Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
			},
		},
		"valid exit target": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetExit}, Scopes: []string{ScopeRoom}, Input: "test-dir"}},
				Inputs:  []InputSpec{{Name: "test-dir", Type: InputTypeString}},
			},
		},
		"valid combined object and exit target": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetObject, TargetExit}, Scopes: []string{ScopeRoom}, Input: "test-target"}},
				Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
			},
		},
		"valid scope_target": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{
					{Name: "test-container", Types: []string{TargetObject}, Scopes: []string{ScopeRoom}, Input: "test-from", Optional: true},
					{Name: "test-target", Types: []string{TargetObject}, Scopes: []string{ScopeRoom, ScopeContents}, Input: "test-item", ScopeTarget: "test-container"},
				},
				Inputs: []InputSpec{
					{Name: "test-item", Type: InputTypeString, Required: true},
					{Name: "test-from", Type: InputTypeString},
				},
			},
		},
		"scope_target requires contents scope": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{
					{Name: "test-container", Types: []string{TargetObject}, Scopes: []string{ScopeRoom}, Input: "test-from"},
					{Name: "test-target", Types: []string{TargetObject}, Scopes: []string{ScopeRoom}, Input: "test-item", ScopeTarget: "test-container"},
				},
				Inputs: []InputSpec{
					{Name: "test-item", Type: InputTypeString},
					{Name: "test-from", Type: InputTypeString},
				},
			},
			expErr: `target "test-target": scope_target requires "contents" in scope`,
		},
		"contents scope requires scope_target": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{
					{Name: "test-target", Types: []string{TargetObject}, Scopes: []string{ScopeContents}, Input: "test-item"},
				},
				Inputs: []InputSpec{
					{Name: "test-item", Type: InputTypeString},
				},
			},
			expErr: `target "test-target": "contents" scope requires scope_target to be set`,
		},
		"scope_target must reference earlier target": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{
					{Name: "test-target", Types: []string{TargetObject}, Scopes: []string{ScopeContents}, Input: "test-item", ScopeTarget: "test-container"},
				},
				Inputs: []InputSpec{
					{Name: "test-item", Type: InputTypeString},
				},
			},
			expErr: `target "test-target": scope_target "test-container" must reference a target declared earlier in the targets array`,
		},
		"scope_target only for object targets": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{
					{Name: "test-container", Types: []string{TargetObject}, Scopes: []string{ScopeRoom}, Input: "test-from"},
					{Name: "test-target", Types: []string{TargetPlayer}, Scopes: []string{ScopeContents}, Input: "test-item", ScopeTarget: "test-container"},
				},
				Inputs: []InputSpec{
					{Name: "test-item", Type: InputTypeString},
					{Name: "test-from", Type: InputTypeString},
				},
			},
			expErr: `target "test-target": scope_target is only supported for object targets`,
		},
		"valid not_found template": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetPlayer}, Scopes: []string{ScopeRoom}, Input: "test-who", NotFound: "You don't see '{{ .Inputs.who }}' here."}},
				Inputs:  []InputSpec{{Name: "test-who", Type: InputTypeString}},
			},
		},
		"invalid not_found template": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetPlayer}, Scopes: []string{ScopeRoom}, Input: "test-who", NotFound: "{{ .Bad }"}},
				Inputs:  []InputSpec{{Name: "test-who", Type: InputTypeString}},
			},
			expErr: `target "test-target": invalid not_found template:`,
		},
		"allow_unresolved requires optional": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetPlayer}, Scopes: []string{ScopeRoom}, Input: "test-who", AllowUnresolved: true}},
				Inputs:  []InputSpec{{Name: "test-who", Type: InputTypeString}},
			},
			expErr: `target "test-target": allow_unresolved requires optional`,
		},
		"allow_unresolved with optional is valid": {
			cmd: Command{
				Handler: "test-handler",
				Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetPlayer}, Scopes: []string{ScopeRoom}, Input: "test-who", Optional: true, AllowUnresolved: true}},
				Inputs:  []InputSpec{{Name: "test-who", Type: InputTypeString}},
			},
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
