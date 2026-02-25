package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/game"
)

// mockCharStore implements storage.Storer[*game.Character] for testing
type mockCharStore struct {
	chars map[string]*game.Character
}

func (m *mockCharStore) Get(id string) *game.Character {
	return m.chars[id]
}

func (m *mockCharStore) GetAll() map[string]*game.Character {
	result := make(map[string]*game.Character)
	for k, v := range m.chars {
		result[k] = v
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

func (m *mockZoneStore) GetAll() map[string]*game.Zone {
	result := make(map[string]*game.Zone)
	for k, v := range m.zones {
		result[k] = v
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

func (m *mockRoomStore) GetAll() map[string]*game.Room {
	result := make(map[string]*game.Room)
	for k, v := range m.rooms {
		result[k] = v
	}
	return result
}

func (m *mockRoomStore) Save(id string, room *game.Room) error {
	m.rooms[id] = room
	return nil
}

// mockMobileStore implements storage.Storer[*game.Mobile] for testing
type mockMobileStore struct {
	mobiles map[string]*game.Mobile
}

func (m *mockMobileStore) Get(id string) *game.Mobile {
	return m.mobiles[id]
}

func (m *mockMobileStore) GetAll() map[string]*game.Mobile {
	result := make(map[string]*game.Mobile)
	for k, v := range m.mobiles {
		result[k] = v
	}
	return result
}

func (m *mockMobileStore) Save(id string, mobile *game.Mobile) error {
	m.mobiles[id] = mobile
	return nil
}

// mockObjectStore implements storage.Storer[*game.Object] for testing
type mockObjectStore struct {
	objects map[string]*game.Object
}

func (m *mockObjectStore) Get(id string) *game.Object {
	return m.objects[id]
}

func (m *mockObjectStore) GetAll() map[string]*game.Object {
	result := make(map[string]*game.Object)
	for k, v := range m.objects {
		result[k] = v
	}
	return result
}

func (m *mockObjectStore) Save(id string, object *game.Object) error {
	m.objects[id] = object
	return nil
}

// mockRaceStore implements storage.Storer[*game.Race] for testing
type mockRaceStore struct {
	races map[string]*game.Race
}

func (m *mockRaceStore) Get(id string) *game.Race {
	return m.races[id]
}

func (m *mockRaceStore) GetAll() map[string]*game.Race {
	result := make(map[string]*game.Race)
	for k, v := range m.races {
		result[k] = v
	}
	return result
}

func (m *mockRaceStore) Save(id string, race *game.Race) error {
	m.races[id] = race
	return nil
}

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
		"expand session zone": {
			tmplStr: "zone-{{ .Session.ZoneId }}",
			data: struct {
				Session *game.PlayerState
			}{
				Session: &game.PlayerState{
					ZoneId: "forest",
				},
			},
			exp: "zone-forest",
		},
		"expand session room": {
			tmplStr: "room-{{ .Session.RoomId }}",
			data: struct {
				Session *game.PlayerState
			}{
				Session: &game.PlayerState{
					RoomId: "clearing",
				},
			},
			exp: "room-clearing",
		},
		"expand config value": {
			tmplStr: "player-{{ .Config.target }}",
			data: struct {
				Config map[string]any
			}{
				Config: map[string]any{
					"target": "bob",
				},
			},
			exp: "player-bob",
		},
		"expand multiple values": {
			tmplStr: "zone-{{ .Session.ZoneId }}-room-{{ .Session.RoomId }}",
			data: struct {
				Session *game.PlayerState
			}{
				Session: &game.PlayerState{
					ZoneId: "castle",
					RoomId: "throne",
				},
			},
			exp: "zone-castle-room-throne",
		},
		"expand actor name": {
			tmplStr: "{{ .Actor.Name }} says hello",
			data: struct {
				Actor *game.Character
			}{
				Actor: &game.Character{Name: "Bob"},
			},
			exp: "Bob says hello",
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
