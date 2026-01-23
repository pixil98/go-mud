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
		"valid command with no params": {
			cmd: Command{
				Handler: "quit",
			},
			expErr: "",
		},
		"valid command with params": {
			cmd: Command{
				Handler: "say",
				Params: []ParamSpec{
					{Name: "text", Type: ParamTypeString, Required: true},
				},
			},
			expErr: "",
		},
		"param missing name": {
			cmd: Command{
				Handler: "test",
				Params: []ParamSpec{
					{Type: ParamTypeString},
				},
			},
			expErr: "param 0: name is required",
		},
		"param missing type": {
			cmd: Command{
				Handler: "test",
				Params: []ParamSpec{
					{Name: "foo"},
				},
			},
			expErr: `param "foo": type is required`,
		},
		"rest param not last": {
			cmd: Command{
				Handler: "test",
				Params: []ParamSpec{
					{Name: "first", Type: ParamTypeString, Rest: true},
					{Name: "second", Type: ParamTypeString},
				},
			},
			expErr: `param "first": only the last parameter can have rest=true`,
		},
		"rest param at end is valid": {
			cmd: Command{
				Handler: "say",
				Params: []ParamSpec{
					{Name: "target", Type: ParamTypePlayer, Required: true},
					{Name: "text", Type: ParamTypeString, Required: true, Rest: true},
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
