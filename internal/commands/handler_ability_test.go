package commands

import (
	"fmt"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

func TestExecuteAbility_APGating(t *testing.T) {
	tests := map[string]struct {
		startAP int
		apCost  int
		wantErr string
		wantAP  int // AP remaining after the call
	}{
		"sufficient ap succeeds": {
			startAP: 2,
			apCost:  1,
			wantAP:  1,
		},
		"exact ap succeeds": {
			startAP: 1,
			apCost:  1,
			wantAP:  0,
		},
		"zero ap_cost treated as 1": {
			startAP: 1,
			apCost:  0,
			wantAP:  0,
		},
		"insufficient ap fails": {
			startAP: 0,
			apCost:  1,
			wantErr: "not ready",
			wantAP:  0,
		},
		"cost exceeds available ap fails": {
			startAP: 1,
			apCost:  2,
			wantErr: "not ready",
			wantAP:  1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, zone := newTestRoomInZone("r", "Room", "z")
			_ = zone
			player := newTestPlayer("player", "Player", room)
			setPlayerAP(player, tc.startAP)

			ability := &assets.Ability{
				Name:     "test",
				APCost:   tc.apCost,
				Handler:  "test",
				Messages: assets.AbilityMessages{},
			}
			in := &CommandInput{Char: player}

			err := executeAbility(ability, in, nil, nil, nil, nil)

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

			if got := player.CurrentAP(); got != tc.wantAP {
				t.Errorf("AP after execute = %d, want %d", got, tc.wantAP)
			}
		})
	}
}

func TestExecuteAbility_CostCheckOrdering(t *testing.T) {
	const manaMax = 10
	tests := map[string]struct {
		startAP      int
		apCost       int
		startMana    int
		resourceCost int
		wantErr      string
		wantAP       int
		wantMana     int
	}{
		"insufficient resource does not spend AP": {
			startAP:      1,
			apCost:       1,
			startMana:    5,
			resourceCost: 10,
			wantErr:      "mana",
			wantAP:       1,
			wantMana:     5,
		},
		"insufficient AP does not spend resource": {
			startAP:      0,
			apCost:       1,
			startMana:    10,
			resourceCost: 5,
			wantErr:      "not ready",
			wantAP:       0,
			wantMana:     10,
		},
		"both sufficient: AP and resource deducted": {
			startAP:      1,
			apCost:       1,
			startMana:    10,
			resourceCost: 5,
			wantAP:       0,
			wantMana:     5,
		},
		"no resource cost: only AP spent": {
			startAP:      1,
			apCost:       1,
			startMana:    10,
			resourceCost: 0,
			wantAP:       0,
			wantMana:     10,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, zone := newTestRoomInZone("r", "Room", "z")
			_ = zone
			player := newTestPlayer("player", "Player", room)

			// Set up both perks at once so mana.max is in place when SetResource is called.
			perks := []assets.Perk{
				{Type: assets.PerkTypeModifier, Key: "core.resource.mana.max", Value: manaMax},
			}
			if tc.startAP > 0 {
				perks = append(perks, assets.Perk{Type: assets.PerkTypeModifier, Key: assets.PerkKeyActionPointsMax, Value: tc.startAP})
			}
			player.PerkCache.SetOwn(perks)
			player.SetResource("mana", tc.startMana)
			player.ResetAP()

			ability := &assets.Ability{
				Name:         "test",
				APCost:       tc.apCost,
				Resource:     "mana",
				ResourceCost: tc.resourceCost,
				Handler:      "test",
				Messages:     assets.AbilityMessages{},
			}
			in := &CommandInput{Char: player}

			err := executeAbility(ability, in, nil, nil, nil, nil)

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

			if got := player.CurrentAP(); got != tc.wantAP {
				t.Errorf("AP after execute = %d, want %d", got, tc.wantAP)
			}
			if cur, _ := player.Resource("mana"); cur != tc.wantMana {
				t.Errorf("mana after execute = %d, want %d", cur, tc.wantMana)
			}
		})
	}
}

// setPlayerAP primes the player's AP to the given value via the perk system.
func setPlayerAP(player *game.CharacterInstance, ap int) {
	player.PerkCache.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: assets.PerkKeyActionPointsMax, Value: ap},
	})
	player.ResetAP()
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

func TestAttackEffect(t *testing.T) {
	tests := map[string]struct {
		targets    map[string]*TargetRef
		startErr   error
		wantErr    string
		wantStart  bool
		wantQueued bool
	}{
		"mob target: StartCombat and QueueAttack called": {
			targets: map[string]*TargetRef{
				"target": {Type: targetTypeMobile, Mob: &MobileRef{
					InstanceId: "mob-1",
					Name:       "Goblin",
					instance:   newTestMobInstance("mob-1", "Goblin"),
				}},
			},
			wantStart:  true,
			wantQueued: true,
		},
		"already in combat: StartCombat and QueueAttack still called": {
			targets: map[string]*TargetRef{
				"target": {Type: targetTypeMobile, Mob: &MobileRef{
					InstanceId: "mob-1",
					Name:       "Goblin",
					instance:   newTestMobInstance("mob-1", "Goblin"),
				}},
			},
			wantStart:  true,
			wantQueued: true,
		},
		"no targets: no-op": {
			targets:    map[string]*TargetRef{},
			wantStart:  false,
			wantQueued: false,
		},
		"nil target ref: skipped": {
			targets:    map[string]*TargetRef{"target": nil},
			wantStart:  false,
			wantQueued: false,
		},
		"StartCombat error: wrapped as user error": {
			targets: map[string]*TargetRef{
				"target": {Type: targetTypeMobile, Mob: &MobileRef{
					InstanceId: "mob-1",
					Name:       "Goblin",
					instance:   newTestMobInstance("mob-1", "Goblin"),
				}},
			},
			startErr: fmt.Errorf("target is not alive"),
			wantErr:  "target is not alive",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, _ := newTestRoomInZone("r", "Room", "z")
			player := newTestPlayer("player", "Player", room)
			setPlayerAP(player, 2)

			cm := &mockCombatManager{startedErr: tc.startErr}
			effect := &attackEffect{combat: cm}
			ability := &assets.Ability{
				Command: assets.Command{
					Targets: []assets.TargetSpec{{Name: "target"}},
				},
			}

			err := effect.Execute(ability, &CommandInput{Char: player}, tc.targets)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !containsStr(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want containing %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cm.started != tc.wantStart {
				t.Errorf("StartCombat called = %v, want %v", cm.started, tc.wantStart)
			}
			if cm.queued != tc.wantQueued {
				t.Errorf("QueueAttack called = %v, want %v", cm.queued, tc.wantQueued)
			}
		})
	}
}

func TestDamageEffect_InitiatesCombat(t *testing.T) {
	room, zone := newTestRoomInZone("r", "Room", "z")
	_ = zone
	player := newTestPlayer("player", "Player", room)

	mob := newTestMobInstance("mob-1", "Goblin")
	cm := &mockCombatManager{}
	effect := &damageEffect{combat: cm}

	ability := &assets.Ability{
		Config: map[string]any{"base_damage": float64(10)},
		Command: assets.Command{
			Targets: []assets.TargetSpec{{Name: "target"}},
		},
	}
	targets := map[string]*TargetRef{
		"target": {Type: targetTypeMobile, Mob: &MobileRef{
			InstanceId: "mob-1",
			Name:       "Goblin",
			instance:   mob,
		}},
	}

	if err := effect.Execute(ability, &CommandInput{Char: player}, targets); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cm.started {
		t.Error("StartCombat not called")
	}
	if !cm.threatAdded {
		t.Error("AddThreat not called")
	}
	if cm.threatAmount != 10 {
		t.Errorf("AddThreat amount = %d, want 10", cm.threatAmount)
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
