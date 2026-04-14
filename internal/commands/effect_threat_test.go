package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
)

func TestAoeThreatEffect(t *testing.T) {
	tests := map[string]struct {
		mode         string
		amount       string
		inCombatOnly bool
		mobInCombat  []bool // per-mob combat state
		wantInCombat bool   // whether mobs should end up in combat
	}{
		"add mode hits all mobs": {
			mode:         "add",
			amount:       "50",
			mobInCombat:  []bool{false, false},
			wantInCombat: true,
		},
		"add mode in_combat_only skips out-of-combat mobs": {
			mode:         "add",
			amount:       "50",
			inCombatOnly: true,
			mobInCombat:  []bool{true, false, true},
		},
		"set_to_top mode": {
			mode:         "set_to_top",
			mobInCombat:  []bool{false},
			wantInCombat: true,
		},
		"set_to_value mode": {
			mode:         "set_to_value",
			amount:       "100",
			mobInCombat:  []bool{false},
			wantInCombat: true,
		},
		"empty room": {
			mode:        "add",
			amount:      "10",
			mobInCombat: nil,
		},
		"in_combat_only with no mobs in combat": {
			mode:         "add",
			amount:       "10",
			inCombatOnly: true,
			mobInCombat:  []bool{false, false},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, _ := newTestRoomInZone("r", "Room", "z")
			player := newTestPlayer("player", "Player", room)
			setCombatReady(player)

			for i, inCombat := range tc.mobInCombat {
				mi := newCombatMob("mob-"+string(rune('a'+i)), "mob")
				mi.SetInCombat(inCombat)
				room.AddMob(mi)
			}

			effect := &aoeThreatEffect{}
			config := map[string]string{"mode": tc.mode}
			if tc.amount != "" {
				config["amount"] = tc.amount
			}
			if tc.inCombatOnly {
				config["in_combat_only"] = "true"
			}

			if err := effect.ValidateConfig(config); err != nil {
				t.Fatalf("unexpected validate error: %v", err)
			}

			fn := effect.Create("test:0", config, nil)
			if err := fn(player, nil, &AbilityResult{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantInCombat && !player.IsInCombat() {
				t.Error("player should be in combat after threat effect")
			}
		})
	}
}

func TestAoeThreatEffect_PeacefulArea(t *testing.T) {
	room, _ := newTestRoomInZone("r", "Room", "z")
	player := newTestPlayer("player", "Player", room)
	player.AddSource("room", room.Perks)

	room.Perks.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeGrant, Key: assets.PerkGrantPeaceful},
	})

	mi := newCombatMob("mob-a", "mob")
	room.AddMob(mi)

	effect := &aoeThreatEffect{}
	fn := effect.Create("test:0", map[string]string{"mode": "add", "amount": "10"}, nil)
	err := fn(player, nil, &AbilityResult{})
	if err == nil {
		t.Fatal("expected peaceful area error, got nil")
	}
	if !containsStr(err.Error(), "peaceful") {
		t.Errorf("error = %q, want to contain \"peaceful\"", err.Error())
	}
}

func TestAoeThreatEffect_ValidateConfig(t *testing.T) {
	tests := map[string]struct {
		config  map[string]string
		wantErr string
	}{
		"valid add": {
			config: map[string]string{"mode": "add", "amount": "10"},
		},
		"valid set_to_top": {
			config: map[string]string{"mode": "set_to_top"},
		},
		"valid set_to_value": {
			config: map[string]string{"mode": "set_to_value", "amount": "50"},
		},
		"valid in_combat_only true": {
			config: map[string]string{"mode": "add", "amount": "10", "in_combat_only": "true"},
		},
		"valid in_combat_only false": {
			config: map[string]string{"mode": "add", "amount": "10", "in_combat_only": "false"},
		},
		"unknown mode": {
			config:  map[string]string{"mode": "bogus"},
			wantErr: "unknown threat mode",
		},
		"add missing amount": {
			config:  map[string]string{"mode": "add"},
			wantErr: "amount config required",
		},
		"set_to_value missing amount": {
			config:  map[string]string{"mode": "set_to_value"},
			wantErr: "amount config required",
		},
		"non-integer amount": {
			config:  map[string]string{"mode": "add", "amount": "abc"},
			wantErr: "amount must be an integer",
		},
		"in_combat_only typo rejected": {
			config:  map[string]string{"mode": "add", "amount": "10", "in_combat_only": "yes"},
			wantErr: "in_combat_only must be",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			effect := &aoeThreatEffect{}
			err := effect.ValidateConfig(tc.config)
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
		})
	}
}
