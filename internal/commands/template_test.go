package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
)

func TestExpandTemplate(t *testing.T) {
	tests := map[string]struct {
		tmplStr string
		data    any
		exp     string
		expErr  bool
	}{
		"plain string no expansion": {
			tmplStr: "hello world",
			data:    struct{}{},
			exp:     "hello world",
		},
		"expand actor name": {
			tmplStr: "{{ .Actor.Name }} says hello",
			data: &templateContext{
				Actor: &assets.Character{Name: "test-actor"},
			},
			exp: "test-actor says hello",
		},
		"expand input value": {
			tmplStr: "player-{{ .Inputs.target }}",
			data: &templateContext{
				Actor:  &assets.Character{},
				Inputs: map[string]any{"target": "test-target"},
			},
			exp: "player-test-target",
		},
		"expand multiple values": {
			tmplStr: "{{ .Actor.Name }} targets {{ .Targets.target.Player.Name }}",
			data: &templateContext{
				Actor: &assets.Character{Name: "test-actor"},
				Targets: map[string]*TargetRef{
					"target": {Type: targetTypePlayer, Player: &PlayerRef{Name: "test-target"}},
				},
			},
			exp: "test-actor targets test-target",
		},
		"invalid template syntax": {
			tmplStr: "{{ .Invalid",
			data:    struct{}{},
			expErr:  true,
		},
		"missing field": {
			tmplStr: "{{ .Nonexistent }}",
			data:    struct{}{},
			expErr:  true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ExpandTemplate(tt.tmplStr, tt.data)

			if tt.expErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.exp {
				t.Errorf("got %q, expected %q", got, tt.exp)
			}
		})
	}
}
