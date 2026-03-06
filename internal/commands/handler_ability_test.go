package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// newTestRoomInZone creates a room and a zone instance with that room registered.
func newTestRoomInZone(roomId, name, zoneId string) (*game.RoomInstance, *game.ZoneInstance) {
	room, _ := newTestRoom(roomId, name, zoneId)
	zone, _ := newTestZone(zoneId)
	zone.AddRoom(room)
	return room, zone
}

func TestRoomBuffEffect(t *testing.T) {
	tests := map[string]struct {
		config  map[string]any
		wantErr string
		wantMod int
		wantKey string
	}{
		"applies perks to room": {
			config: map[string]any{
				"duration": float64(5),
				"perks": []any{
					map[string]any{"type": "modifier", "key": "test-key", "value": float64(10)},
				},
			},
			wantKey: "test-key",
			wantMod: 10,
		},
		"missing duration": {
			config: map[string]any{
				"perks": []any{
					map[string]any{"type": "modifier", "key": "test-key", "value": float64(1)},
				},
			},
			wantErr: "positive duration",
		},
		"missing perks": {
			config: map[string]any{
				"duration": float64(3),
			},
			wantErr: "perks config required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, zone := newTestRoomInZone("test-room", "Test Room", "test-zone")
			player := newTestPlayer("test-player", "Tester", room)
			player.PerkCache.AddSource("room", room.Perks)

			world := &mockZoneLocator{zones: map[string]*game.ZoneInstance{"test-zone": zone}}
			effect := &roomBuffEffect{world: world}

			ability := &assets.Ability{
				Name:   "test-ability",
				Config: tc.config,
			}
			in := &CommandInput{Char: player}

			err := effect.Execute(ability, in, nil)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !containsStr(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want containing %q", err, tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify perks were applied to the room.
			if !room.Perks.HasPerks("test-ability") {
				t.Error("room does not have timed perk entry")
			}

			// Verify the player sees the room perks through their PerkCache.
			if got := player.ModifierValue(tc.wantKey); got != tc.wantMod {
				t.Errorf("player modifier %q = %d, want %d", tc.wantKey, got, tc.wantMod)
			}
		})
	}
}

func TestRoomBuffEffectExpiry(t *testing.T) {
	room, zone := newTestRoomInZone("test-room", "Test Room", "test-zone")
	player := newTestPlayer("test-player", "Tester", room)
	player.PerkCache.AddSource("room", room.Perks)

	effect := &roomBuffEffect{world: &mockZoneLocator{zones: map[string]*game.ZoneInstance{"test-zone": zone}}}

	ability := &assets.Ability{
		Name: "test-ability",
		Config: map[string]any{
			"duration": float64(2),
			"perks": []any{
				map[string]any{"type": "modifier", "key": "test-key", "value": float64(5)},
			},
		},
	}

	if err := effect.Execute(ability, &CommandInput{Char: player}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := player.ModifierValue("test-key"); got != 5 {
		t.Fatalf("before tick = %d, want 5", got)
	}

	room.Perks.Tick()
	if got := player.ModifierValue("test-key"); got != 5 {
		t.Errorf("after 1 tick = %d, want 5", got)
	}

	room.Perks.Tick()
	if got := player.ModifierValue("test-key"); got != 0 {
		t.Errorf("after 2 ticks = %d, want 0", got)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
