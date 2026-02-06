package commands

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestResolvePlayer(t *testing.T) {
	tests := map[string]struct {
		chars         map[string]*game.Character
		onlinePlayers []string
		input         string
		expCharName   string
		expErr        string
	}{
		"exact match": {
			chars:         map[string]*game.Character{"bob": {Name: "Bob"}},
			onlinePlayers: []string{"bob"},
			input:         "bob",
			expCharName:   "Bob",
		},
		"case insensitive match uppercase input": {
			chars:         map[string]*game.Character{"bob": {Name: "Bob"}},
			onlinePlayers: []string{"bob"},
			input:         "BOB",
			expCharName:   "Bob",
		},
		"case insensitive match mixed case input": {
			chars:         map[string]*game.Character{"bob": {Name: "Bob"}},
			onlinePlayers: []string{"bob"},
			input:         "BoB",
			expCharName:   "Bob",
		},
		"player not found": {
			chars:         map[string]*game.Character{"bob": {Name: "Bob"}},
			onlinePlayers: []string{"bob"},
			input:         "alice",
			expErr:        "Player 'alice' not found",
		},
		"player offline": {
			chars:         map[string]*game.Character{"bob": {Name: "Bob"}},
			onlinePlayers: []string{},
			input:         "bob",
			expErr:        "Player 'bob' not found",
		},
		"partial match does not work": {
			chars:         map[string]*game.Character{"bobby": {Name: "Bobby"}},
			onlinePlayers: []string{"bobby"},
			input:         "bob",
			expErr:        "Player 'bob' not found",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			charStore := &mockCharStore{chars: tt.chars}
			world := game.NewWorldState(nil, charStore, &mockZoneStore{zones: map[string]*game.Zone{}}, &mockRoomStore{rooms: map[string]*game.Room{}}, &mockMobileStore{mobiles: map[string]*game.Mobile{}})

			for _, charId := range tt.onlinePlayers {
				_ = world.AddPlayer(storage.Identifier(charId), make(chan []byte, 1), "zone", "room")
			}

			resolver := &DefaultTargetResolver{}
			result, err := resolver.ResolvePlayer(world, tt.input)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Character == nil {
				t.Fatal("result.Character is nil")
			}
			if result.Character.Name != tt.expCharName {
				t.Errorf("Character.Name() = %q, expected %q", result.Character.Name, tt.expCharName)
			}

			if result.State == nil {
				t.Fatal("result.State is nil")
			}

			if result.Raw != tt.input {
				t.Errorf("Raw = %q, expected %q", result.Raw, tt.input)
			}
		})
	}
}

func TestResolveTarget(t *testing.T) {
	tests := map[string]struct {
		chars         map[string]*game.Character
		onlinePlayers []string
		input         string
		expCharName   string
		expErr        string
	}{
		"resolves player": {
			chars:         map[string]*game.Character{"alice": {Name: "Alice"}},
			onlinePlayers: []string{"alice"},
			input:         "alice",
			expCharName:   "Alice",
		},
		"not found returns generic error": {
			chars:         map[string]*game.Character{},
			onlinePlayers: []string{},
			input:         "nobody",
			expErr:        "Target 'nobody' not found",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			charStore := &mockCharStore{chars: tt.chars}
			world := game.NewWorldState(nil, charStore, &mockZoneStore{zones: map[string]*game.Zone{}}, &mockRoomStore{rooms: map[string]*game.Room{}}, &mockMobileStore{mobiles: map[string]*game.Mobile{}})

			for _, charId := range tt.onlinePlayers {
				_ = world.AddPlayer(storage.Identifier(charId), make(chan []byte, 1), "zone", "room")
			}

			resolver := &DefaultTargetResolver{}
			result, err := resolver.ResolveTarget(world, tt.input)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			resolved, ok := result.(*ResolvedPlayer)
			if !ok {
				t.Fatalf("expected *ResolvedPlayer, got %T", result)
			}

			if resolved.Character.Name != tt.expCharName {
				t.Errorf("Character.Name() = %q, expected %q", resolved.Character.Name, tt.expCharName)
			}
		})
	}
}

func TestResolveMob(t *testing.T) {
	tests := map[string]struct {
		input  string
		expRaw string
	}{
		"stub returns raw input": {
			input:  "goblin",
			expRaw: "goblin",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			charStore := &mockCharStore{chars: map[string]*game.Character{}}
			world := game.NewWorldState(nil, charStore, &mockZoneStore{zones: map[string]*game.Zone{}}, &mockRoomStore{rooms: map[string]*game.Room{}}, &mockMobileStore{mobiles: map[string]*game.Mobile{}})

			resolver := &DefaultTargetResolver{}
			result, err := resolver.ResolveMob(world, tt.input)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Raw != tt.expRaw {
				t.Errorf("Raw = %q, expected %q", result.Raw, tt.expRaw)
			}
		})
	}
}

func TestResolveItem(t *testing.T) {
	tests := map[string]struct {
		input  string
		expRaw string
	}{
		"stub returns raw input": {
			input:  "sword",
			expRaw: "sword",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			charStore := &mockCharStore{chars: map[string]*game.Character{}}
			world := game.NewWorldState(nil, charStore, &mockZoneStore{zones: map[string]*game.Zone{}}, &mockRoomStore{rooms: map[string]*game.Room{}}, &mockMobileStore{mobiles: map[string]*game.Mobile{}})

			resolver := &DefaultTargetResolver{}
			result, err := resolver.ResolveItem(world, tt.input)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Raw != tt.expRaw {
				t.Errorf("Raw = %q, expected %q", result.Raw, tt.expRaw)
			}
		})
	}
}

func TestNewTemplateData_TargetResolution(t *testing.T) {
	tests := map[string]struct {
		chars         map[string]*game.Character
		onlinePlayers []string
		actorId       string
		args          []ParsedArg
		expTargetName string
		expErr        string
	}{
		"resolves player target": {
			chars: map[string]*game.Character{
				"alice": {Name: "Alice"},
				"bob":   {Name: "Bob"},
			},
			onlinePlayers: []string{"alice", "bob"},
			actorId:       "alice",
			args: []ParsedArg{
				{
					Spec:  &ParamSpec{Name: "target", Type: ParamTypePlayer},
					Value: "bob",
				},
			},
			expTargetName: "Bob",
		},
		"player not found returns error": {
			chars: map[string]*game.Character{
				"alice": {Name: "Alice"},
			},
			onlinePlayers: []string{"alice"},
			actorId:       "alice",
			args: []ParsedArg{
				{
					Spec:  &ParamSpec{Name: "target", Type: ParamTypePlayer},
					Value: "nobody",
				},
			},
			expErr: "Player 'nobody' not found",
		},
		"string args pass through unchanged": {
			chars: map[string]*game.Character{
				"alice": {Name: "Alice"},
			},
			onlinePlayers: []string{"alice"},
			actorId:       "alice",
			args: []ParsedArg{
				{
					Spec:  &ParamSpec{Name: "message", Type: ParamTypeString},
					Value: "hello",
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			charStore := &mockCharStore{chars: tt.chars}
			world := game.NewWorldState(nil, charStore, &mockZoneStore{zones: map[string]*game.Zone{}}, &mockRoomStore{rooms: map[string]*game.Room{}}, &mockMobileStore{mobiles: map[string]*game.Mobile{}})

			for _, charId := range tt.onlinePlayers {
				_ = world.AddPlayer(storage.Identifier(charId), make(chan []byte, 1), "zone", "room")
			}

			data, err := NewTemplateData(world, storage.Identifier(tt.actorId), tt.args)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expTargetName != "" {
				target, ok := data.Args["target"].(*ResolvedPlayer)
				if !ok {
					t.Fatalf("Args[\"target\"] is %T, expected *ResolvedPlayer", data.Args["target"])
				}
				if target.Character.Name != tt.expTargetName {
					t.Errorf("target.Character.Name() = %q, expected %q", target.Character.Name, tt.expTargetName)
				}
			}
		})
	}
}
