package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestExpandTemplate(t *testing.T) {
	tests := map[string]struct {
		tmplStr string
		data    *TemplateData
		exp     string
		expErr  bool
	}{
		"plain string no expansion": {
			tmplStr: "hello world",
			data:    &TemplateData{},
			exp:     "hello world",
		},
		"expand state zone": {
			tmplStr: "zone-{{ .State.Zone }}",
			data: &TemplateData{
				State: &game.EntityState{
					Zone: storage.Identifier("forest"),
				},
			},
			exp: "zone-forest",
		},
		"expand state room": {
			tmplStr: "room-{{ .State.Room }}",
			data: &TemplateData{
				State: &game.EntityState{
					Room: storage.Identifier("clearing"),
				},
			},
			exp: "room-clearing",
		},
		"expand args": {
			tmplStr: "player-{{ .Args.target }}",
			data: &TemplateData{
				State: &game.EntityState{},
				Args: map[string]any{
					"target": "bob",
				},
			},
			exp: "player-bob",
		},
		"expand multiple values": {
			tmplStr: "zone-{{ .State.Zone }}-room-{{ .State.Room }}",
			data: &TemplateData{
				State: &game.EntityState{
					Zone: storage.Identifier("castle"),
					Room: storage.Identifier("throne"),
				},
			},
			exp: "zone-castle-room-throne",
		},
		"invalid template syntax": {
			tmplStr: "{{ .Invalid",
			data:    &TemplateData{},
			expErr:  true,
		},
		"missing field": {
			tmplStr: "{{ .Nonexistent }}",
			data:    &TemplateData{},
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

func TestNewTemplateData(t *testing.T) {
	tests := map[string]struct {
		state   *game.EntityState
		args    []ParsedArg
		expArgs map[string]any
	}{
		"empty args": {
			state:   &game.EntityState{},
			args:    nil,
			expArgs: map[string]any{},
		},
		"single arg": {
			state: &game.EntityState{},
			args: []ParsedArg{
				{
					Spec:  &ParamSpec{Name: "count"},
					Value: 5,
				},
			},
			expArgs: map[string]any{
				"count": 5,
			},
		},
		"multiple args": {
			state: &game.EntityState{},
			args: []ParsedArg{
				{
					Spec:  &ParamSpec{Name: "target"},
					Value: "bob",
				},
				{
					Spec:  &ParamSpec{Name: "message"},
					Value: "hello there",
				},
			},
			expArgs: map[string]any{
				"target":  "bob",
				"message": "hello there",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := NewTemplateData(tt.state, tt.args)

			if got.State != tt.state {
				t.Errorf("State pointer mismatch")
			}

			if len(got.Args) != len(tt.expArgs) {
				t.Errorf("Args length = %d, expected %d", len(got.Args), len(tt.expArgs))
				return
			}

			for k, exp := range tt.expArgs {
				if got.Args[k] != exp {
					t.Errorf("Args[%q] = %v, expected %v", k, got.Args[k], exp)
				}
			}
		})
	}
}
