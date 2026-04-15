package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/gametest"
)

func TestAttackEffect(t *testing.T) {
	tests := map[string]struct {
		targets   map[string][]*TargetRef
		wantErr   string
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
		{Type: assets.PerkTypeGrant, Key: assets.PerkGrantPeaceful},
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

func TestAoeDamageEffect(t *testing.T) {
	// Each test sets up a room with player, other (another player), wildMob,
	// and petMob all present. casterRole picks who casts the AoE. setup can
	// wire up follow relationships. expHit lists roles that should take
	// damage; all other non-caster roles must be unchanged. The caster is
	// never hit by its own AoE.
	type fixture struct {
		player  *game.CharacterInstance
		other   *game.CharacterInstance
		wildMob *game.MobileInstance
		petMob  *game.MobileInstance
	}

	tests := map[string]struct {
		casterRole string // "player" or "wildMob"
		setup      func(f *fixture)
		expHit     []string
	}{
		"player caster hits all wild mobs": {
			casterRole: "player",
			expHit:     []string{"wildMob", "petMob"},
		},
		"player caster spares own pet": {
			casterRole: "player",
			setup: func(f *fixture) {
				f.petMob.SetFollowing(f.player)
			},
			expHit: []string{"wildMob"},
		},
		"player caster spares another player's pet": {
			casterRole: "player",
			setup: func(f *fixture) {
				f.petMob.SetFollowing(f.other)
			},
			expHit: []string{"wildMob"},
		},
		"mob caster hits all players": {
			casterRole: "wildMob",
			expHit:     []string{"player", "other"},
		},
		"mob caster hits players' pets": {
			casterRole: "wildMob",
			setup: func(f *fixture) {
				f.petMob.SetFollowing(f.other)
			},
			expHit: []string{"player", "other", "petMob"},
		},
		"mob caster spares other wild mobs": {
			casterRole: "wildMob",
			// petMob stays wild (no follow)
			expHit: []string{"player", "other"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, _ := newTestRoomInZone("r", "Room", "z")

			player := newTestPlayer("player", "Player", room)
			setCombatReady(player)

			other := newTestPlayer("other", "Other", room)
			setCombatReady(other)

			wildMob := newCombatMob("wild", "wild")
			room.AddMob(wildMob)

			petMob := newCombatMob("pet", "pet")
			room.AddMob(petMob)

			f := &fixture{player: player, other: other, wildMob: wildMob, petMob: petMob}
			if tc.setup != nil {
				tc.setup(f)
			}

			var caster game.Actor
			switch tc.casterRole {
			case "player":
				caster = player
			case "wildMob":
				caster = wildMob
			default:
				t.Fatalf("unknown casterRole %q", tc.casterRole)
			}

			effect := &aoeDamageEffect{}
			fn := effect.Create("test:0", map[string]string{"amount": "10"}, nil)
			if err := fn(caster, nil, &AbilityResult{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			roles := map[string]game.Actor{
				"player":  player,
				"other":   other,
				"wildMob": wildMob,
				"petMob":  petMob,
			}
			wantHit := make(map[string]bool, len(tc.expHit))
			for _, r := range tc.expHit {
				wantHit[r] = true
			}

			for role, a := range roles {
				cur, _ := a.Resource(assets.ResourceHp)
				if a == caster {
					if cur != 100 {
						t.Errorf("caster %q should not damage itself, HP=%d", role, cur)
					}
					continue
				}
				if wantHit[role] {
					if cur >= 100 {
						t.Errorf("role %q should have taken damage, HP=%d", role, cur)
					}
				} else {
					if cur != 100 {
						t.Errorf("role %q should be unchanged, HP=%d", role, cur)
					}
				}
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
