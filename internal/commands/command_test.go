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
