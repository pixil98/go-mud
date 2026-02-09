package commands

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestScopeFromString(t *testing.T) {
	tests := map[string]struct {
		input string
		exp   Scope
	}{
		"room": {
			input: "room",
			exp:   ScopeRoom,
		},
		"inventory": {
			input: "inventory",
			exp:   ScopeInventory,
		},
		"world": {
			input: "world",
			exp:   ScopeWorld,
		},
		"zone": {
			input: "zone",
			exp:   ScopeZone,
		},
		"Room uppercase": {
			input: "ROOM",
			exp:   ScopeRoom,
		},
		"World mixed case": {
			input: "World",
			exp:   ScopeWorld,
		},
		"unknown returns zero": {
			input: "unknown",
			exp:   0,
		},
		"empty returns zero": {
			input: "",
			exp:   0,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := ScopeFromString(tt.input)
			if got != tt.exp {
				t.Errorf("ScopeFromString(%q) = %d, expected %d", tt.input, got, tt.exp)
			}
		})
	}
}

func TestScopesFromConfig(t *testing.T) {
	tests := map[string]struct {
		input any
		exp   Scope
	}{
		"single string room": {
			input: "room",
			exp:   ScopeRoom,
		},
		"single string world": {
			input: "world",
			exp:   ScopeWorld,
		},
		"array with one scope": {
			input: []any{"room"},
			exp:   ScopeRoom,
		},
		"array with multiple scopes": {
			input: []any{"room", "inventory"},
			exp:   ScopeRoom | ScopeInventory,
		},
		"array with all scopes": {
			input: []any{"room", "inventory", "world", "zone"},
			exp:   ScopeRoom | ScopeInventory | ScopeWorld | ScopeZone,
		},
		"nil returns zero": {
			input: nil,
			exp:   0,
		},
		"int returns zero": {
			input: 42,
			exp:   0,
		},
		"empty array returns zero": {
			input: []any{},
			exp:   0,
		},
		"array with unknown values ignored": {
			input: []any{"room", "unknown", "world"},
			exp:   ScopeRoom | ScopeWorld,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := ScopesFromConfig(tt.input)
			if got != tt.exp {
				t.Errorf("ScopesFromConfig(%v) = %d, expected %d", tt.input, got, tt.exp)
			}
		})
	}
}

func TestIsResolveDirective(t *testing.T) {
	tests := map[string]struct {
		input    any
		expValid bool
		exp      *ResolveDirective
	}{
		"valid directive all fields": {
			input: map[string]any{
				"$resolve":  "player",
				"$scope":    "world",
				"$input":    "target",
				"$optional": true,
			},
			expValid: true,
			exp: &ResolveDirective{
				Resolve:  EntityPlayer,
				Scope:    ScopeWorld,
				Input:    "target",
				Optional: true,
			},
		},
		"valid directive minimal": {
			input: map[string]any{
				"$resolve": "player",
			},
			expValid: true,
			exp: &ResolveDirective{
				Resolve: EntityPlayer,
			},
		},
		"valid directive array scope": {
			input: map[string]any{
				"$resolve": "item",
				"$scope":   []any{"room", "inventory"},
				"$input":   "item",
			},
			expValid: true,
			exp: &ResolveDirective{
				Resolve: EntityItem,
				Scope:   ScopeRoom | ScopeInventory,
				Input:   "item",
			},
		},
		"not a map": {
			input:    "not a directive",
			expValid: false,
		},
		"map without $resolve": {
			input: map[string]any{
				"$scope": "world",
				"$input": "target",
			},
			expValid: false,
		},
		"$resolve not a string": {
			input: map[string]any{
				"$resolve": 42,
			},
			expValid: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, valid := IsResolveDirective(tt.input)

			if valid != tt.expValid {
				t.Errorf("IsResolveDirective valid = %v, expected %v", valid, tt.expValid)
				return
			}

			if !tt.expValid {
				return
			}

			if got.Resolve != tt.exp.Resolve {
				t.Errorf("Resolve = %q, expected %q", got.Resolve, tt.exp.Resolve)
			}
			if got.Scope != tt.exp.Scope {
				t.Errorf("Scope = %d, expected %d", got.Scope, tt.exp.Scope)
			}
			if got.Input != tt.exp.Input {
				t.Errorf("Input = %q, expected %q", got.Input, tt.exp.Input)
			}
			if got.Optional != tt.exp.Optional {
				t.Errorf("Optional = %v, expected %v", got.Optional, tt.exp.Optional)
			}
		})
	}
}

func TestResolver_ResolvePlayer(t *testing.T) {
	tests := map[string]struct {
		chars          map[string]*game.Character
		onlinePlayers  map[string]struct{ zone, room storage.Identifier }
		actorZone      storage.Identifier
		actorRoom      storage.Identifier
		name           string
		scope          Scope
		expPlayerName  string
		expErr         string
	}{
		"world scope finds any player": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"other-zone", "other-room"},
			},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "bob",
			scope:         ScopeWorld,
			expPlayerName: "Bob",
		},
		"zone scope finds player in same zone": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "other-room"},
			},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "bob",
			scope:         ScopeZone,
			expPlayerName: "Bob",
		},
		"zone scope rejects player in different zone": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"other-zone", "other-room"},
			},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "bob",
			scope:     ScopeZone,
			expErr:    "Player 'bob' not found",
		},
		"room scope finds player in same room": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "my-room"},
			},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "bob",
			scope:         ScopeRoom,
			expPlayerName: "Bob",
		},
		"room scope rejects player in different room same zone": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "other-room"},
			},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "bob",
			scope:     ScopeRoom,
			expErr:    "Player 'bob' not found",
		},
		"combined scope room or zone finds in zone": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "other-room"},
			},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "bob",
			scope:         ScopeRoom | ScopeZone,
			expPlayerName: "Bob",
		},
		"case insensitive match": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "my-room"},
			},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "BOB",
			scope:         ScopeWorld,
			expPlayerName: "Bob",
		},
		"player not found": {
			chars:         map[string]*game.Character{},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "nobody",
			scope:         ScopeWorld,
			expErr:        "Player 'nobody' not found",
		},
		"player offline": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "bob",
			scope:         ScopeWorld,
			expErr:        "Player 'bob' not found",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			charStore := &mockCharStore{chars: tt.chars}
			world := game.NewWorldState(nil, charStore, &mockZoneStore{zones: map[string]*game.Zone{}}, &mockRoomStore{rooms: map[string]*game.Room{}}, &mockMobileStore{mobiles: map[string]*game.Mobile{}})

			// Add actor
			actorChan := make(chan []byte, 1)
			_ = world.AddPlayer("actor", actorChan, tt.actorZone, tt.actorRoom)
			actorState := world.GetPlayer("actor")

			// Add other players
			for charId, loc := range tt.onlinePlayers {
				ch := make(chan []byte, 1)
				_ = world.AddPlayer(storage.Identifier(charId), ch, loc.zone, loc.room)
			}

			resolver := NewResolver(world)
			result, err := resolver.resolvePlayer(actorState, tt.name, tt.scope)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if result.Name != tt.expPlayerName {
				t.Errorf("Name = %q, expected %q", result.Name, tt.expPlayerName)
			}
		})
	}
}

func TestResolver_ResolveTarget(t *testing.T) {
	tests := map[string]struct {
		chars          map[string]*game.Character
		onlinePlayers  map[string]struct{ zone, room storage.Identifier }
		actorZone      storage.Identifier
		actorRoom      storage.Identifier
		name           string
		scope          Scope
		expType        string
		expName        string
		expErr         string
	}{
		"resolves player as target": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "my-room"},
			},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "bob",
			scope:     ScopeWorld,
			expType:   "player",
			expName:   "Bob",
		},
		"target not found": {
			chars:         map[string]*game.Character{},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "nobody",
			scope:         ScopeWorld,
			expErr:        "Target 'nobody' not found",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			charStore := &mockCharStore{chars: tt.chars}
			world := game.NewWorldState(nil, charStore, &mockZoneStore{zones: map[string]*game.Zone{}}, &mockRoomStore{rooms: map[string]*game.Room{}}, &mockMobileStore{mobiles: map[string]*game.Mobile{}})

			// Add actor
			actorChan := make(chan []byte, 1)
			_ = world.AddPlayer("actor", actorChan, tt.actorZone, tt.actorRoom)
			actorState := world.GetPlayer("actor")

			// Add other players
			for charId, loc := range tt.onlinePlayers {
				ch := make(chan []byte, 1)
				_ = world.AddPlayer(storage.Identifier(charId), ch, loc.zone, loc.room)
			}

			resolver := NewResolver(world)
			result, err := resolver.resolveTarget(actorState, tt.name, tt.scope)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if result.Type != tt.expType {
				t.Errorf("Type = %q, expected %q", result.Type, tt.expType)
			}

			if result.Name != tt.expName {
				t.Errorf("Name = %q, expected %q", result.Name, tt.expName)
			}
		})
	}
}
