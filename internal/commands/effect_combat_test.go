package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/gametest"
)

func TestAttackEffect(t *testing.T) {
	tests := map[string]struct {
		targets map[string][]*TargetRef
		wantErr string
	}{
		"mob target: starts combat and deals damage": {
			targets: map[string][]*TargetRef{
				"target": {{Type: targetTypeActor, Actor: &ActorRef{
					Name:  "Goblin",
					actor: &gametest.BaseActor{ActorId: "mob-1", ActorName: "Goblin", Alive: true},
				}}},
			},
		},
		"no targets: no-op": {
			targets: map[string][]*TargetRef{},
		},
		"nil target ref: skipped": {
			targets: map[string][]*TargetRef{"target": nil},
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
		{Type: assets.PerkTypeGrant, Key: string(assets.RoomFlagPeaceful)},
	})

	effect := &attackEffect{}
	targetSpecs := []assets.TargetSpec{{Name: "target"}}
	targets := map[string][]*TargetRef{
		"target": {{Type: targetTypeActor, Actor: &ActorRef{
			actor: &gametest.BaseActor{ActorId: "mob-1", ActorName: "Goblin", Alive: true},
		}}},
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
	targets := map[string][]*TargetRef{
		"target": {{Type: targetTypeActor, Actor: &ActorRef{
			Name:  "Goblin",
			actor: mob,
		}}},
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
