package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// mockCharStore implements storage.Storer[*game.Character] for testing
type mockCharStore struct {
	chars map[string]*game.Character
}

func (m *mockCharStore) Get(id string) *game.Character {
	return m.chars[id]
}

func (m *mockCharStore) GetAll() map[storage.Identifier]*game.Character {
	result := make(map[storage.Identifier]*game.Character)
	for k, v := range m.chars {
		result[storage.Identifier(k)] = v
	}
	return result
}

func (m *mockCharStore) Save(id string, char *game.Character) error {
	m.chars[id] = char
	return nil
}

// mockZoneStore implements storage.Storer[*game.Zone] for testing
type mockZoneStore struct {
	zones map[string]*game.Zone
}

func (m *mockZoneStore) Get(id string) *game.Zone {
	return m.zones[id]
}

func (m *mockZoneStore) GetAll() map[storage.Identifier]*game.Zone {
	result := make(map[storage.Identifier]*game.Zone)
	for k, v := range m.zones {
		result[storage.Identifier(k)] = v
	}
	return result
}

func (m *mockZoneStore) Save(id string, zone *game.Zone) error {
	m.zones[id] = zone
	return nil
}

// mockRoomStore implements storage.Storer[*game.Room] for testing
type mockRoomStore struct {
	rooms map[string]*game.Room
}

func (m *mockRoomStore) Get(id string) *game.Room {
	return m.rooms[id]
}

func (m *mockRoomStore) GetAll() map[storage.Identifier]*game.Room {
	result := make(map[storage.Identifier]*game.Room)
	for k, v := range m.rooms {
		result[storage.Identifier(k)] = v
	}
	return result
}

func (m *mockRoomStore) Save(id string, room *game.Room) error {
	m.rooms[id] = room
	return nil
}

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
			tmplStr: "zone-{{ .State.ZoneId }}",
			data: &TemplateData{
				State: &game.PlayerState{
					ZoneId: storage.Identifier("forest"),
				},
			},
			exp: "zone-forest",
		},
		"expand state room": {
			tmplStr: "room-{{ .State.RoomId }}",
			data: &TemplateData{
				State: &game.PlayerState{
					RoomId: storage.Identifier("clearing"),
				},
			},
			exp: "room-clearing",
		},
		"expand args": {
			tmplStr: "player-{{ .Args.target }}",
			data: &TemplateData{
				State: &game.PlayerState{},
				Args: map[string]any{
					"target": "bob",
				},
			},
			exp: "player-bob",
		},
		"expand multiple values": {
			tmplStr: "zone-{{ .State.ZoneId }}-room-{{ .State.RoomId }}",
			data: &TemplateData{
				State: &game.PlayerState{
					ZoneId: storage.Identifier("castle"),
					RoomId: storage.Identifier("throne"),
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
	charStore := &mockCharStore{
		chars: map[string]*game.Character{
			"testplayer": {CharName: "TestPlayer"},
		},
	}
	zoneStore := &mockZoneStore{zones: map[string]*game.Zone{}}
	roomStore := &mockRoomStore{rooms: map[string]*game.Room{}}
	world := game.NewWorldState(nil, charStore, zoneStore, roomStore)
	charId := storage.Identifier("testplayer")
	_ = world.AddPlayer(charId, make(chan []byte, 1), "testzone", "testroom")

	tests := map[string]struct {
		args    []ParsedArg
		expArgs map[string]any
	}{
		"empty args": {
			args:    nil,
			expArgs: map[string]any{},
		},
		"single arg": {
			args: []ParsedArg{
				{
					Spec:  &ParamSpec{Name: "count", Type: ParamTypeNumber},
					Value: 5,
				},
			},
			expArgs: map[string]any{
				"count": 5,
			},
		},
		"multiple string args": {
			args: []ParsedArg{
				{
					Spec:  &ParamSpec{Name: "direction", Type: ParamTypeDirection},
					Value: "north",
				},
				{
					Spec:  &ParamSpec{Name: "message", Type: ParamTypeString},
					Value: "hello there",
				},
			},
			expArgs: map[string]any{
				"direction": "north",
				"message":   "hello there",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := NewTemplateData(world, charId, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Actor == nil {
				t.Errorf("Actor is nil, expected character")
				return
			}

			if got.Actor.Name() != "TestPlayer" {
				t.Errorf("Actor.Name() = %q, expected %q", got.Actor.Name(), "TestPlayer")
			}

			if got.State == nil {
				t.Errorf("State is nil, expected player state")
				return
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
