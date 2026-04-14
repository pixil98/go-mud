package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/gametest"
)

func TestExecuteAbility_APGating(t *testing.T) {
	tests := map[string]struct {
		apCost       int
		spendAPFails bool
		wantErr      string
		wantSpentAP  int // cost passed to SpendAP
	}{
		"sufficient ap succeeds": {
			apCost:      1,
			wantSpentAP: 1,
		},
		"zero ap_cost treated as 1": {
			apCost:      0,
			wantSpentAP: 1,
		},
		"insufficient ap fails": {
			apCost:       1,
			spendAPFails: true,
			wantErr:      "not ready",
			wantSpentAP:  1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actor := &gametest.BaseActor{ActorId: "player", ActorName: "Player", SpendAPFails: tc.spendAPFails}

			ca := &compiledAbility{apCost: tc.apCost}
			_, err := ca.exec(actor, nil, ExecAbilityOpts{})

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !containsStr(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want containing %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if actor.SpentAP != tc.wantSpentAP {
				t.Errorf("SpendAP called with %d, want %d", actor.SpentAP, tc.wantSpentAP)
			}
		})
	}
}

func TestExecuteAbility_CostCheckOrdering(t *testing.T) {
	tests := map[string]struct {
		startMana    int
		resourceCost int
		spendAPFails bool
		wantErr      string
		wantMana     int
		wantSpentAP  int // 0 means SpendAP was never called
	}{
		"insufficient resource does not spend AP": {
			startMana:    5,
			resourceCost: 10,
			wantErr:      "mana",
			wantMana:     5,
			wantSpentAP:  0,
		},
		"insufficient AP does not spend resource": {
			startMana:    10,
			resourceCost: 5,
			spendAPFails: true,
			wantErr:      "not ready",
			wantMana:     10,
			wantSpentAP:  1,
		},
		"both sufficient: AP and resource deducted": {
			startMana:    10,
			resourceCost: 5,
			wantMana:     5,
			wantSpentAP:  1,
		},
		"no resource cost: only AP spent": {
			startMana:    10,
			resourceCost: 0,
			wantMana:     10,
			wantSpentAP:  1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actor := &gametest.BaseActor{
				ActorId:      "player",
				ActorName:    "Player",
				SpendAPFails: tc.spendAPFails,
				Resources:    map[string][2]int{"mana": {tc.startMana, 10}},
			}

			ca := &compiledAbility{
				apCost:       1,
				resource:     "mana",
				resourceCost: tc.resourceCost,
			}
			_, err := ca.exec(actor, nil, ExecAbilityOpts{})

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !containsStr(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want containing %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if actor.SpentAP != tc.wantSpentAP {
				t.Errorf("SpendAP called with %d, want %d", actor.SpentAP, tc.wantSpentAP)
			}
			if cur, _ := actor.Resource("mana"); cur != tc.wantMana {
				t.Errorf("mana after execute = %d, want %d", cur, tc.wantMana)
			}
		})
	}
}

// setCombatReady gives the player AP and HP so combat effects can function.
func setCombatReady(player *game.CharacterInstance) {
	player.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: assets.PerkKeyActionPointsMax, Value: 10},
		{Type: assets.PerkTypeModifier, Key: assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectMax), Value: 100},
	})
	player.ResetAP()
	player.SetResource(assets.ResourceHp, 100)
}

// newTestRoomInZone creates a room and a zone instance with that room registered.
func newTestRoomInZone(roomId, name, zoneId string) (*game.RoomInstance, *game.ZoneInstance) {
	room, _ := newTestRoom(roomId, name, zoneId)
	zone, _ := newTestZone(zoneId)
	zone.AddRoom(room)
	return room, zone
}

func TestRoomBuffEffect(t *testing.T) {
	tests := map[string]struct {
		config  map[string]string
		wantErr string
		wantMod int
		wantKey string
	}{
		"applies perk to room": {
			config: map[string]string{
				"duration":   "5",
				"perk_type":  "modifier",
				"perk_key":   "test-key",
				"perk_value": "10",
			},
			wantKey: "test-key",
			wantMod: 10,
		},
		"missing duration": {
			config: map[string]string{
				"perk_type":  "modifier",
				"perk_key":   "test-key",
				"perk_value": "1",
			},
			wantErr: "positive duration",
		},
		"missing perk_type": {
			config: map[string]string{
				"duration": "3",
				"perk_key": "test-key",
			},
			wantErr: "perk_type or grant_key config required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, _ := newTestRoomInZone("test-room", "Test Room", "test-zone")
			player := newTestPlayer("test-player", "Tester", room)
			player.AddSource("room", room.Perks)

			effect := &buffEffect{scope: buffScopeRoom}

			if err := effect.ValidateConfig(tc.config); err != nil {
				if tc.wantErr != "" {
					if !containsStr(err.Error(), tc.wantErr) {
						t.Fatalf("error = %q, want containing %q", err, tc.wantErr)
					}
					return
				}
				t.Fatalf("unexpected validate error: %v", err)
			}

			fn := effect.Create("test-ability:0", tc.config, nil)
			err := fn(player, nil, &AbilityResult{})

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

			if got := player.ModifierValue(tc.wantKey); got != tc.wantMod {
				t.Errorf("player modifier %q = %d, want %d", tc.wantKey, got, tc.wantMod)
			}
		})
	}
}

func TestRoomBuffEffectExpiry(t *testing.T) {
	room, _ := newTestRoomInZone("test-room", "Test Room", "test-zone")
	player := newTestPlayer("test-player", "Tester", room)
	player.AddSource("room", room.Perks)

	effect := &buffEffect{scope: buffScopeRoom}

	config := map[string]string{
		"duration":   "2",
		"perk_type":  "modifier",
		"perk_key":   "test-key",
		"perk_value": "5",
	}
	fn := effect.Create("test-ability:0", config, nil)

	if err := fn(player, nil, &AbilityResult{}); err != nil {
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
