package commands

import (
	"fmt"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
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
			actor := &mockActor{id: "player", name: "Player", spendAPFails: tc.spendAPFails}

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

			if actor.spentAP != tc.wantSpentAP {
				t.Errorf("SpendAP called with %d, want %d", actor.spentAP, tc.wantSpentAP)
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
			actor := &mockActor{
				id:           "player",
				name:         "Player",
				spendAPFails: tc.spendAPFails,
				resources:    map[string][2]int{"mana": {tc.startMana, 10}},
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

			if actor.spentAP != tc.wantSpentAP {
				t.Errorf("SpendAP called with %d, want %d", actor.spentAP, tc.wantSpentAP)
			}
			if cur, _ := actor.Resource("mana"); cur != tc.wantMana {
				t.Errorf("mana after execute = %d, want %d", cur, tc.wantMana)
			}
		})
	}
}

// setPlayerAP primes the player's AP to the given value via the perk system.
func setPlayerAP(player *game.CharacterInstance, ap int) {
	player.SetOwn([]assets.Perk{
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
			room, zone := newTestRoomInZone("test-room", "Test Room", "test-zone")
			player := newTestPlayer("test-player", "Tester", room)
			player.AddSource("room", room.Perks)

			world := &mockBuffWorld{zones: map[string]*game.ZoneInstance{"test-zone": zone}}
			effect := &buffEffect{scope: buffScopeRoom, world: world}

			// ValidateConfig first, then Create
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
	player.AddSource("room", room.Perks)

	effect := &buffEffect{scope: buffScopeRoom, world: &mockBuffWorld{zones: map[string]*game.ZoneInstance{"test-zone": zone}}}

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

func TestAttackEffect(t *testing.T) {
	tests := map[string]struct {
		targets    map[string]*TargetRef
		startErr  error
		wantErr   string
		wantStart bool
	}{
		"mob target: starts combat and deals damage": {
			targets: map[string]*TargetRef{
				"target": {Type: targetTypeActor, Actor: &ActorRef{
					Name:  "Goblin",
					actor: newTestMobInstance("mob-1", "Goblin", nil),
				}},
			},
			wantStart: true,
		},
		"no targets: no-op": {
			targets:   map[string]*TargetRef{},
			wantStart: false,
		},
		"nil target ref: skipped": {
			targets:   map[string]*TargetRef{"target": nil},
			wantStart: false,
		},
		"StartCombat error: wrapped as user error": {
			targets: map[string]*TargetRef{
				"target": {Type: targetTypeActor, Actor: &ActorRef{
					Name:  "Goblin",
					actor: newTestMobInstance("mob-1", "Goblin", nil),
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
			if cm.started != tc.wantStart {
				t.Errorf("StartCombat called = %v, want %v", cm.started, tc.wantStart)
			}
		})
	}
}

func TestAttackEffect_PeacefulArea(t *testing.T) {
	room, _ := newTestRoomInZone("r", "Room", "z")
	player := newTestPlayer("player", "Player", room)
	setPlayerAP(player, 2)

	// Wire room as a source so the player inherits room perks.
	player.AddSource("room", room.Perks)

	// Make the room peaceful.
	room.Perks.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeGrant, Key: assets.PerkGrantPeaceful},
	})

	cm := &mockCombatManager{}
	effect := &attackEffect{combat: cm}
	targetSpecs := []assets.TargetSpec{{Name: "target"}}
	targets := map[string]*TargetRef{
		"target": {Type: targetTypeActor, Actor: &ActorRef{
			actor: newTestMobInstance("mob-1", "Goblin", nil),
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
	if cm.started {
		t.Error("StartCombat should not have been called in a peaceful area")
	}
}

func TestDamageEffect_InitiatesCombat(t *testing.T) {
	room, zone := newTestRoomInZone("r", "Room", "z")
	_ = zone
	player := newTestPlayer("player", "Player", room)

	mob := newTestMobInstance("mob-1", "Goblin", nil)
	cm := &mockCombatManager{}
	effect := &damageEffect{combat: cm}

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

func TestAoeDamageEffect(t *testing.T) {
	hpPerks := []assets.Perk{
		{Type: assets.PerkTypeModifier, Key: assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectMax), Value: 100},
	}

	tests := map[string]struct {
		mobCount   int
		hitAllies bool
		wantStarts int
		wantHPDrop bool // whether mobs should take damage
		wantPlayer bool // whether other player should take damage
	}{
		"hits all mobs": {
			mobCount:   3,
			wantStarts: 3,
			wantHPDrop: true,
		},
		"empty room": {
			mobCount:   0,
			wantStarts: 0,
		},
		"does not hit allies by default": {
			mobCount:   1,
			hitAllies: false,
			wantStarts: 1,
			wantHPDrop: true,
			wantPlayer: false,
		},
		"hits allies when configured": {
			mobCount:   0,
			hitAllies: true,
			wantStarts: 1,
			wantPlayer: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, zone := newTestRoomInZone("r", "Room", "z")
			player := newTestPlayer("player", "Player", room)
			setPlayerAP(player, 2)

			// Add a second player to test player damage.
			other := newTestPlayer("other", "Other", room)
			other.SetOwn(hpPerks)
			other.SetResource(assets.ResourceHp, 100)

			var mobs []*game.MobileInstance
			for i := range tc.mobCount {
				id := fmt.Sprintf("mob-%d", i)
				mi := newTestMobInstance(id, "mob", hpPerks)
				mi.SetResource(assets.ResourceHp, 100)
				room.AddMob(mi)
				mobs = append(mobs, mi)
			}

			cm := &mockCombatManager{}
			world := &mockZoneLocator{zones: map[string]*game.ZoneInstance{"z": zone}}
			effect := &aoeDamageEffect{combat: cm, world: world}

			config := map[string]string{"amount": "10"}
			if tc.hitAllies {
				config["hit_allies"] = "true"
			}

			fn := effect.Create("test:0", config, nil)
			if err := fn(player, nil, &AbilityResult{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cm.startCount != tc.wantStarts {
				t.Errorf("StartCombat count = %d, want %d", cm.startCount, tc.wantStarts)
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
	room, zone := newTestRoomInZone("r", "Room", "z")
	player := newTestPlayer("player", "Player", room)
	setPlayerAP(player, 2)
	player.AddSource("room", room.Perks)

	room.Perks.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeGrant, Key: assets.PerkGrantPeaceful},
	})

	cm := &mockCombatManager{}
	world := &mockZoneLocator{zones: map[string]*game.ZoneInstance{"z": zone}}
	effect := &aoeDamageEffect{combat: cm, world: world}

	fn := effect.Create("test:0", map[string]string{"amount": "10"}, nil)
	err := fn(player, nil, &AbilityResult{})
	if err == nil {
		t.Fatal("expected peaceful area error, got nil")
	}
	if !containsStr(err.Error(), "peaceful") {
		t.Errorf("error = %q, want to contain \"peaceful\"", err.Error())
	}
}

func TestAoeThreatEffect(t *testing.T) {
	tests := map[string]struct {
		mode         string
		amount       string
		inCombatOnly bool
		mobInCombat  []bool // per-mob combat state
		startErr     error  // error returned by StartCombat
		wantStarts   int
		wantThreat   int  // total AddThreat calls
		wantTop      bool // TopThreat called
		wantSet      bool // SetThreat called
	}{
		"add mode hits all mobs": {
			mode:        "add",
			amount:      "50",
			mobInCombat: []bool{false, false},
			wantStarts:  2,
			wantThreat:  2,
		},
		"add mode in_combat_only skips out-of-combat mobs": {
			mode:         "add",
			amount:       "50",
			inCombatOnly: true,
			mobInCombat:  []bool{true, false, true},
			wantStarts:   2,
			wantThreat:   2,
		},
		"set_to_top mode": {
			mode:        "set_to_top",
			mobInCombat: []bool{false},
			wantStarts:  1,
			wantTop:     true,
		},
		"set_to_value mode": {
			mode:        "set_to_value",
			amount:      "100",
			mobInCombat: []bool{false},
			wantStarts:  1,
			wantSet:     true,
		},
		"empty room": {
			mode:        "add",
			amount:      "10",
			mobInCombat: nil,
			wantStarts:  0,
			wantThreat:  0,
		},
		"in_combat_only with no mobs in combat": {
			mode:         "add",
			amount:       "10",
			inCombatOnly: true,
			mobInCombat:  []bool{false, false},
			wantStarts:   0,
			wantThreat:   0,
		},
		"StartCombat error skips mob": {
			mode:        "add",
			amount:      "50",
			mobInCombat: []bool{false},
			startErr:    fmt.Errorf("target is not alive"),
			wantStarts:  1,
			wantThreat:  0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, zone := newTestRoomInZone("r", "Room", "z")
			player := newTestPlayer("player", "Player", room)

			for i, inCombat := range tc.mobInCombat {
				id := fmt.Sprintf("mob-%d", i)
				mi := newTestMobInstance(id, "mob", nil)
				mi.SetInCombat(inCombat)
				room.AddMob(mi)
			}

			cm := &mockCombatManager{startedErr: tc.startErr}
			world := &mockZoneLocator{zones: map[string]*game.ZoneInstance{"z": zone}}
			effect := &aoeThreatEffect{combat: cm, world: world}

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

			if cm.startCount != tc.wantStarts {
				t.Errorf("StartCombat count = %d, want %d", cm.startCount, tc.wantStarts)
			}
			if cm.threatCount != tc.wantThreat {
				t.Errorf("AddThreat count = %d, want %d", cm.threatCount, tc.wantThreat)
			}
			if tc.wantTop && !cm.topThreatCalled {
				t.Error("TopThreat not called")
			}
			if tc.wantSet && !cm.setThreatCalled {
				t.Error("SetThreat not called")
			}
		})
	}
}

func TestAoeThreatEffect_PeacefulArea(t *testing.T) {
	room, zone := newTestRoomInZone("r", "Room", "z")
	player := newTestPlayer("player", "Player", room)
	player.AddSource("room", room.Perks)

	room.Perks.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeGrant, Key: assets.PerkGrantPeaceful},
	})

	mi := newTestMobInstance("mob-0", "mob", nil)
	room.AddMob(mi)

	cm := &mockCombatManager{}
	world := &mockZoneLocator{zones: map[string]*game.ZoneInstance{"z": zone}}
	effect := &aoeThreatEffect{combat: cm, world: world}

	fn := effect.Create("test:0", map[string]string{"mode": "add", "amount": "10"}, nil)
	err := fn(player, nil, &AbilityResult{})
	if err == nil {
		t.Fatal("expected peaceful area error, got nil")
	}
	if !containsStr(err.Error(), "peaceful") {
		t.Errorf("error = %q, want to contain \"peaceful\"", err.Error())
	}
	if cm.started {
		t.Error("StartCombat should not have been called in a peaceful area")
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

func TestExecCombatAbility_PlayerTargetRouting(t *testing.T) {
	tests := map[string]struct {
		isCharacter  bool
		wantTargetId string
		wantTargetMsg bool
	}{
		"player target: TargetId and TargetMsg set": {
			isCharacter:   true,
			wantTargetId:  "player-1",
			wantTargetMsg: true,
		},
		"mob target: TargetId empty, no TargetMsg": {
			isCharacter:   false,
			wantTargetId:  "",
			wantTargetMsg: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, _ := newTestRoomInZone("r", "Room", "z")
			actor := newTestMobInstance("mob-actor", "Mob", nil)

			// The effect appends to TargetLines but does not set TargetId,
			// mimicking attackEffect behaviour.
			targetLineEffect := EffectFunc(func(_ shared.Actor, _ map[string]*TargetRef, result *AbilityResult) error {
				result.TargetLines = append(result.TargetLines, "Mob hits you!")
				result.RoomLines = append(result.RoomLines, "Mob hits Target!")
				return nil
			})
			ca := &compiledAbility{effectFuncs: []EffectFunc{targetLineEffect}}

			h := &Handler{abilities: map[string]*compiledAbility{"attack": ca}}

			var target shared.Actor
			if tc.isCharacter {
				target = newTestPlayer("player-1", "Target", room)
			} else {
				target = newTestMobInstance("mob-target", "Target", nil)
			}

			result, err := h.ExecCombatAbility("attack", actor, target)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.TargetId != tc.wantTargetId {
				t.Errorf("TargetId = %q, want %q", result.TargetId, tc.wantTargetId)
			}
			// TargetMsg is only meaningful when TargetId is set; the manager
			// skips delivery when TargetId is empty.
			if tc.wantTargetId != "" && result.TargetMsg == "" {
				t.Error("TargetMsg is empty, want non-empty for player target")
			}
		})
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
