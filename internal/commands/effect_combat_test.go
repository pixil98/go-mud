package commands

import (
	"fmt"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/gametest"
)

func TestAttackEffect(t *testing.T) {
	tests := map[string]struct {
		targets   map[string]*TargetRef
		wantErr   string
	}{
		"mob target: starts combat and deals damage": {
			targets: map[string]*TargetRef{
				"target": {Type: targetTypeActor, Actor: &ActorRef{
					Name:  "Goblin",
					actor: &gametest.BaseActor{ActorId: "mob-1", ActorName: "Goblin", Alive: true},
				}},
			},
		},
		"no targets: no-op": {
			targets: map[string]*TargetRef{},
		},
		"nil target ref: skipped": {
			targets: map[string]*TargetRef{"target": nil},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, _ := newTestRoomInZone("r", "Room", "z")
			player := newTestPlayer("player", "Player", room)
			setCombatReady(player)

			effect := &attackEffect{}
			targetSpecs := []assets.TargetSpec{{Name: "target"}}

			fn := effect.Create("test:0", nil, targetSpecs)
			err := fn(player, tc.targets, &AbilityResult{})

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
		})
	}
}

func TestAttackEffect_PeacefulArea(t *testing.T) {
	room, _ := newTestRoomInZone("r", "Room", "z")
	player := newTestPlayer("player", "Player", room)
	setCombatReady(player)
	player.AddSource("room", room.Perks)

	room.Perks.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeGrant, Key: assets.PerkGrantPeaceful},
	})

	effect := &attackEffect{}
	targetSpecs := []assets.TargetSpec{{Name: "target"}}
	targets := map[string]*TargetRef{
		"target": {Type: targetTypeActor, Actor: &ActorRef{
			actor: &gametest.BaseActor{ActorId: "mob-1", ActorName: "Goblin", Alive: true},
		}},
	}

	fn := effect.Create("test:0", nil, targetSpecs)
	err := fn(player, targets, &AbilityResult{})
	if err == nil {
		t.Fatal("expected peaceful area error, got nil")
	}
	if !containsStr(err.Error(), "peaceful") {
		t.Errorf("error = %q, want to contain \"peaceful\"", err.Error())
	}
}

func TestDamageEffect_InitiatesCombat(t *testing.T) {
	room, _ := newTestRoomInZone("r", "Room", "z")
	player := newTestPlayer("player", "Player", room)
	setCombatReady(player)

	mob := &gametest.BaseActor{
		ActorId:   "mob-1",
		ActorName: "Goblin",
		Alive:     true,
		Resources: map[string][2]int{assets.ResourceHp: {100, 100}},
	}

	effect := &damageEffect{}
	config := map[string]string{"amount": "10"}
	targetSpecs := []assets.TargetSpec{{Name: "target"}}
	targets := map[string]*TargetRef{
		"target": {Type: targetTypeActor, Actor: &ActorRef{
			Name:  "Goblin",
			actor: mob,
		}},
	}

	fn := effect.Create("test:0", config, targetSpecs)
	if err := fn(player, targets, &AbilityResult{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !player.IsInCombat() {
		t.Error("player should be in combat after dealing damage")
	}
	cur, _ := mob.Resource(assets.ResourceHp)
	if cur >= 100 {
		t.Errorf("mob HP should have decreased, got %d", cur)
	}
}

func TestAoeDamageEffect(t *testing.T) {
	hpPerks := []assets.Perk{
		{Type: assets.PerkTypeModifier, Key: assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectMax), Value: 100},
	}

	tests := map[string]struct {
		mobCount   int
		hitAllies  bool
		wantHPDrop bool // whether mobs should take damage
		wantPlayer bool // whether other player should take damage
	}{
		"hits all mobs": {
			mobCount:   3,
			wantHPDrop: true,
		},
		"empty room": {
			mobCount: 0,
		},
		"does not hit allies by default": {
			mobCount:   1,
			hitAllies:  false,
			wantHPDrop: true,
			wantPlayer: false,
		},
		"hits allies when configured": {
			mobCount:   0,
			hitAllies:  true,
			wantPlayer: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, _ := newTestRoomInZone("r", "Room", "z")
			player := newTestPlayer("player", "Player", room)
			setCombatReady(player)

			other := newTestPlayer("other", "Other", room)
			other.SetOwn(hpPerks)
			other.SetResource(assets.ResourceHp, 100)

			var mobs []*game.MobileInstance
			for i := range tc.mobCount {
				mi := newCombatMob(fmt.Sprintf("mob-%d", i), "mob")
				room.AddMob(mi)
				mobs = append(mobs, mi)
			}

			effect := &aoeDamageEffect{}
			config := map[string]string{"amount": "10"}
			if tc.hitAllies {
				config["hit_allies"] = "true"
			}

			fn := effect.Create("test:0", config, nil)
			if err := fn(player, nil, &AbilityResult{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, mi := range mobs {
				cur, _ := mi.Resource(assets.ResourceHp)
				if tc.wantHPDrop && cur >= 100 {
					t.Errorf("mob %s HP should have dropped, got %d", mi.Id(), cur)
				}
			}

			otherHP, _ := other.Resource(assets.ResourceHp)
			if tc.wantPlayer && otherHP >= 100 {
				t.Error("other player HP should have dropped")
			}
			if !tc.wantPlayer && otherHP != 100 {
				t.Errorf("other player HP should be unchanged, got %d", otherHP)
			}
		})
	}
}

func TestAoeDamageEffect_PeacefulArea(t *testing.T) {
	room, _ := newTestRoomInZone("r", "Room", "z")
	player := newTestPlayer("player", "Player", room)
	setCombatReady(player)
	player.AddSource("room", room.Perks)

	room.Perks.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeGrant, Key: assets.PerkGrantPeaceful},
	})

	effect := &aoeDamageEffect{}
	fn := effect.Create("test:0", map[string]string{"amount": "10"}, nil)
	err := fn(player, nil, &AbilityResult{})
	if err == nil {
		t.Fatal("expected peaceful area error, got nil")
	}
	if !containsStr(err.Error(), "peaceful") {
		t.Errorf("error = %q, want to contain \"peaceful\"", err.Error())
	}
}
