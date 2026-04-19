package game

import (
	"context"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
)

func TestActorInstance_Level(t *testing.T) {
	tests := map[string]struct {
		level int
		want  int
	}{
		"level 1":  {level: 1, want: 1},
		"level 10": {level: 10, want: 10},
		"level 0":  {level: 0, want: 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{level: tc.level, PerkCache: *NewPerkCache(nil, nil)}
			if got := a.Level(); got != tc.want {
				t.Errorf("Level() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestActorInstance_Room(t *testing.T) {
	ri := newTestRoom("r")
	tests := map[string]struct {
		room    *RoomInstance
		wantNil bool
	}{
		"returns set room":  {room: ri},
		"returns nil":       {room: nil, wantNil: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{room: tc.room, PerkCache: *NewPerkCache(nil, nil)}
			got := a.Room()
			if tc.wantNil {
				if got != nil {
					t.Error("Room() expected nil")
				}
				return
			}
			if got != tc.room {
				t.Error("Room() returned unexpected instance")
			}
		})
	}
}

func TestActorInstance_IsInCombat_SetInCombat(t *testing.T) {
	tests := map[string]struct {
		setup func(*ActorInstance)
		want  bool
	}{
		"initial state is false":     {setup: func(*ActorInstance) {}, want: false},
		"true after SetInCombat(true)": {setup: func(a *ActorInstance) { a.SetInCombat(true) }, want: true},
		"false after toggle back":    {setup: func(a *ActorInstance) { a.SetInCombat(true); a.SetInCombat(false) }, want: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
			tc.setup(a)
			if got := a.IsInCombat(); got != tc.want {
				t.Errorf("IsInCombat() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestActorInstance_Inventory(t *testing.T) {
	tests := map[string]struct {
		inv     *Inventory
		wantNil bool
	}{
		"returns set inventory": {inv: NewInventory()},
		"returns nil":           {inv: nil, wantNil: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{inventory: tc.inv, PerkCache: *NewPerkCache(nil, nil)}
			got := a.Inventory()
			if tc.wantNil {
				if got != nil {
					t.Error("Inventory() expected nil")
				}
				return
			}
			if got != tc.inv {
				t.Error("Inventory() returned unexpected instance")
			}
		})
	}
}

func TestActorInstance_Equipment(t *testing.T) {
	tests := map[string]struct {
		eq      *Equipment
		wantNil bool
	}{
		"returns set equipment": {eq: NewEquipment()},
		"returns nil":           {eq: nil, wantNil: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{equipment: tc.eq, PerkCache: *NewPerkCache(nil, nil)}
			got := a.Equipment()
			if tc.wantNil {
				if got != nil {
					t.Error("Equipment() expected nil")
				}
				return
			}
			if got != tc.eq {
				t.Error("Equipment() returned unexpected instance")
			}
		})
	}
}

func TestActorInstance_IsAlive(t *testing.T) {
	hpPerk := assets.Perk{
		Type:  assets.PerkTypeModifier,
		Key:   assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectMax),
		Value: 10,
	}
	tests := map[string]struct {
		currentHP int
		wantAlive bool
	}{
		"HP above zero is alive":      {currentHP: 5, wantAlive: true},
		"HP at exactly zero is dead":  {currentHP: 0, wantAlive: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{PerkCache: *NewPerkCache([]assets.Perk{hpPerk}, nil)}
			a.initResources()
			a.setResourceCurrent(assets.ResourceHp, tc.currentHP)
			if got := a.IsAlive(); got != tc.wantAlive {
				t.Errorf("IsAlive() = %v, want %v", got, tc.wantAlive)
			}
		})
	}
}

func TestActorInstance_ForEachResource(t *testing.T) {
	tests := map[string]struct {
		perks     []assets.Perk
		overrides map[string]int
		wantPairs map[string][2]int // name → {current, max}
	}{
		"no resources yields no iterations": {
			wantPairs: map[string][2]int{},
		},
		"two resources are both visited": {
			perks: []assets.Perk{
				{Type: assets.PerkTypeModifier, Key: assets.BuildKey(assets.ResourcePrefix, "hp", assets.ResourceAspectMax), Value: 10},
				{Type: assets.PerkTypeModifier, Key: assets.BuildKey(assets.ResourcePrefix, "mana", assets.ResourceAspectMax), Value: 20},
			},
			overrides: map[string]int{"hp": 5, "mana": 15},
			wantPairs: map[string][2]int{"hp": {5, 10}, "mana": {15, 20}},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{PerkCache: *NewPerkCache(tc.perks, nil)}
			a.initResources()
			for k, v := range tc.overrides {
				a.setResourceCurrent(k, v)
			}
			seen := make(map[string][2]int)
			a.ForEachResource(func(name string, current, maximum int) {
				seen[name] = [2]int{current, maximum}
			})
			for k, want := range tc.wantPairs {
				if got, ok := seen[k]; !ok || got != want {
					t.Errorf("resource[%q] = %v, want %v", k, got, want)
				}
			}
		})
	}
}

func TestActorInstance_ThreatTable(t *testing.T) {
	tests := map[string]struct {
		setup func() *ActorInstance
		check func(*testing.T, *ActorInstance)
	}{
		"HasThreatEntries false before any entry": {
			setup: func() *ActorInstance { return &ActorInstance{PerkCache: *NewPerkCache(nil, nil)} },
			check: func(t *testing.T, a *ActorInstance) {
				if a.HasThreatEntries() {
					t.Error("HasThreatEntries should be false initially")
				}
			},
		},
		"EnsureThreat adds entry visible via HasThreatFrom and ThreatSnapshot": {
			setup: func() *ActorInstance {
				a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
				a.EnsureThreat("e", newTestMI("e", "E"))
				return a
			},
			check: func(t *testing.T, a *ActorInstance) {
				if !a.HasThreatFrom("e") {
					t.Error("HasThreatFrom = false, want true")
				}
				if !a.HasThreatEntries() {
					t.Error("HasThreatEntries = false, want true")
				}
				if v := a.ThreatSnapshot()["e"]; v != 1 {
					t.Errorf("initial threat = %d, want 1", v)
				}
			},
		},
		"AddThreatFrom increments threat": {
			setup: func() *ActorInstance {
				a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
				a.EnsureThreat("e", newTestMI("e", "E"))
				a.AddThreatFrom("e", 9) // 1 + 9 = 10
				return a
			},
			check: func(t *testing.T, a *ActorInstance) {
				if v := a.ThreatSnapshot()["e"]; v != 10 {
					t.Errorf("threat after AddThreatFrom = %d, want 10", v)
				}
			},
		},
		"SetThreatFrom sets absolute value": {
			setup: func() *ActorInstance {
				a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
				a.EnsureThreat("e", newTestMI("e", "E"))
				a.SetThreatFrom("e", 42)
				return a
			},
			check: func(t *testing.T, a *ActorInstance) {
				if v := a.ThreatSnapshot()["e"]; v != 42 {
					t.Errorf("threat after SetThreatFrom = %d, want 42", v)
				}
			},
		},
		"TopThreatFrom makes entry exceed current highest": {
			setup: func() *ActorInstance {
				a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
				a.EnsureThreat("e", newTestMI("e", "E"))
				a.EnsureThreat("f", newTestMI("f", "F"))
				a.SetThreatFrom("f", 100)
				a.TopThreatFrom("e")
				return a
			},
			check: func(t *testing.T, a *ActorInstance) {
				if v := a.ThreatSnapshot()["e"]; v <= 100 {
					t.Errorf("threat after TopThreatFrom = %d, want >100", v)
				}
			},
		},
		"ThreatEnemies returns all actor references": {
			setup: func() *ActorInstance {
				a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
				a.EnsureThreat("e", newTestMI("e", "E"))
				a.EnsureThreat("f", newTestMI("f", "F"))
				return a
			},
			check: func(t *testing.T, a *ActorInstance) {
				if n := len(a.ThreatEnemies()); n != 2 {
					t.Errorf("ThreatEnemies count = %d, want 2", n)
				}
			},
		},
		"ResolveCombatTarget returns non-nil with entries present": {
			setup: func() *ActorInstance {
				a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
				a.EnsureThreat("e", newTestMI("e", "E"))
				return a
			},
			check: func(t *testing.T, a *ActorInstance) {
				if target := a.ResolveCombatTarget(""); target == nil {
					t.Error("ResolveCombatTarget should not return nil")
				}
			},
		},
		"RemoveThreatEntry removes the entry": {
			setup: func() *ActorInstance {
				a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
				a.EnsureThreat("e", newTestMI("e", "E"))
				a.RemoveThreatEntry("e")
				return a
			},
			check: func(t *testing.T, a *ActorInstance) {
				if a.HasThreatFrom("e") {
					t.Error("HasThreatFrom = true after RemoveThreatEntry, want false")
				}
			},
		},
		"ClearThreatTable removes all entries": {
			setup: func() *ActorInstance {
				a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
				a.EnsureThreat("e", newTestMI("e", "E"))
				a.EnsureThreat("f", newTestMI("f", "F"))
				a.ClearThreatTable()
				return a
			},
			check: func(t *testing.T, a *ActorInstance) {
				if a.HasThreatEntries() {
					t.Error("HasThreatEntries = true after ClearThreatTable, want false")
				}
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.check(t, tc.setup())
		})
	}
}

func TestResourceLine(t *testing.T) {
	tests := map[string]struct {
		name    string
		current int
		maximum int
		want    string
	}{
		"formats name and values": {name: "HP", current: 45, maximum: 50, want: "HP: 45/50"},
		"zero current":            {name: "Mana", current: 0, maximum: 100, want: "Mana: 0/100"},
		"current equals max":      {name: "Rage", current: 10, maximum: 10, want: "Rage: 10/10"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := ResourceLine(tc.name, tc.current, tc.maximum); got != tc.want {
				t.Errorf("ResourceLine = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWearSlots(t *testing.T) {
	tests := map[string]struct {
		perks   []assets.Perk
		wantLen int
	}{
		"no grants returns empty":     {wantLen: 0},
		"two distinct slots granted":  {
			perks: []assets.Perk{
				{Type: assets.PerkTypeGrant, Key: assets.PerkGrantWearSlot, Arg: "head"},
				{Type: assets.PerkTypeGrant, Key: assets.PerkGrantWearSlot, Arg: "finger"},
			},
			wantLen: 2,
		},
		"duplicate slot creates two entries": {
			perks: []assets.Perk{
				{Type: assets.PerkTypeGrant, Key: assets.PerkGrantWearSlot, Arg: "finger"},
				{Type: assets.PerkTypeGrant, Key: assets.PerkGrantWearSlot, Arg: "finger"},
			},
			wantLen: 2,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{PerkCache: *NewPerkCache(tc.perks, nil)}
			if got := len(a.WearSlots()); got != tc.wantLen {
				t.Errorf("WearSlots() len = %d, want %d", got, tc.wantLen)
			}
		})
	}
}

func TestCountSlot(t *testing.T) {
	tests := map[string]struct {
		slots []string
		slot  string
		want  int
	}{
		"empty slice":        {slots: nil, slot: "finger", want: 0},
		"slot not present":   {slots: []string{"head", "body"}, slot: "finger", want: 0},
		"slot present once":  {slots: []string{"head", "finger"}, slot: "finger", want: 1},
		"slot present twice": {slots: []string{"finger", "head", "finger"}, slot: "finger", want: 2},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := countSlot(tc.slots, tc.slot); got != tc.want {
				t.Errorf("countSlot(%v, %q) = %d, want %d", tc.slots, tc.slot, got, tc.want)
			}
		})
	}
}



func TestSetFollowing_ManagesReverseLinks(t *testing.T) {
	tests := map[string]struct {
		setup  func() (follower Actor, leader Actor)
		verify func(t *testing.T, follower, leader Actor)
	}{
		"follow adds reverse link": {
			setup: func() (Actor, Actor) {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				a.SetFollowing(b)
				return a, b
			},
			verify: func(t *testing.T, follower, leader Actor) {
				if follower.Following() == nil || follower.Following().Id() != "b" {
					t.Error("follower should be following leader")
				}
				followers := leader.Followers()
				if len(followers) != 1 || followers[0].Id() != "a" {
					t.Errorf("leader should have follower, got %v", followers)
				}
			},
		},
		"unfollow removes reverse link": {
			setup: func() (Actor, Actor) {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				a.SetFollowing(b)
				a.SetFollowing(nil)
				return a, b
			},
			verify: func(t *testing.T, follower, leader Actor) {
				if follower.Following() != nil {
					t.Error("follower should not be following anyone")
				}
				if len(leader.Followers()) != 0 {
					t.Error("leader should have no followers")
				}
			},
		},
		"switch leader moves reverse link": {
			setup: func() (Actor, Actor) {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				a.SetFollowing(b)
				a.SetFollowing(c)
				return a, b
			},
			verify: func(t *testing.T, follower, oldLeader Actor) {
				if follower.Following() == nil || follower.Following().Id() != "c" {
					t.Error("follower should be following new leader")
				}
				if len(oldLeader.Followers()) != 0 {
					t.Error("old leader should have no followers")
				}
			},
		},
		"mob can follow character": {
			setup: func() (Actor, Actor) {
				mi := newTestMI("mob1", "Wolf")
				ci := newTestCI("player1", "Hero")
				mi.SetFollowing(ci)
				return mi, ci
			},
			verify: func(t *testing.T, follower, leader Actor) {
				if follower.Following() == nil || follower.Following().Id() != "player1" {
					t.Error("mob should be following player")
				}
				followers := leader.Followers()
				if len(followers) != 1 || followers[0].Id() != "mob1" {
					t.Errorf("player should have mob follower, got %v", followers)
				}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			follower, leader := tt.setup()
			tt.verify(t, follower, leader)
		})
	}
}

func TestGroupedFollowers(t *testing.T) {
	tests := map[string]struct {
		setup  func() Actor
		expIds []string
	}{
		"no grouped followers": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				return a
			},
			expIds: nil,
		},
		"one grouped follower": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				return a
			},
			expIds: []string{"b"},
		},
		"mixed grouped and ungrouped": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				c.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				return a
			},
			expIds: []string{"b"},
		},
		"ungrouping removes from grouped list": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				a.SetFollowerGrouped("b", false)
				return a
			},
			expIds: nil,
		},
		"nested: sub-leader's grouped followers are separate": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				b.SetFollowerGrouped("c", true)
				// a's direct grouped followers should only be b, not c
				return a
			},
			expIds: []string{"b"},
		},
		"nested: sub-leader has own grouped followers": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				b.SetFollowerGrouped("c", true)
				return b
			},
			expIds: []string{"c"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			leader := tt.setup()
			grouped := leader.GroupedFollowers()

			if len(grouped) != len(tt.expIds) {
				t.Fatalf("expected %d grouped followers, got %d", len(tt.expIds), len(grouped))
			}

			ids := make(map[string]bool)
			for _, ft := range grouped {
				ids[ft.Id()] = true
			}
			for _, id := range tt.expIds {
				if !ids[id] {
					t.Errorf("expected grouped follower %q not found", id)
				}
			}
		})
	}
}

func TestGroupPublishTarget(t *testing.T) {
	tests := map[string]struct {
		setup  func() Actor
		expIds []string
	}{
		"leader with direct grouped followers": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				return a
			},
			expIds: []string{"a", "b"},
		},
		"subgroup: sub-leader grouped followers included": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				b.SetFollowerGrouped("c", true)
				return a
			},
			expIds: []string{"a", "b", "c"},
		},
		"subgroup: ungrouped sub-follower excluded": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				// c is NOT grouped by b
				return a
			},
			expIds: []string{"a", "b"},
		},
		"mob followers skipped in publish": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				mi := newTestMI("mob1", "Wolf")
				mi.SetFollowing(a)
				a.SetFollowerGrouped("mob1", true)
				return a
			},
			expIds: []string{"a"},
		},
		"deep subgroup: three levels": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				d := newTestCI("d", "D")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				b.SetFollowerGrouped("c", true)
				d.SetFollowing(c)
				c.SetFollowerGrouped("d", true)
				return a
			},
			expIds: []string{"a", "b", "c", "d"},
		},
		"subgroup with mob pet: mob skipped, pet owner included": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				pet := newTestMI("pet1", "Wolf")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				pet.SetFollowing(b)
				b.SetFollowerGrouped("pet1", true)
				return a
			},
			// pet is a mob so skipped by publish; a and b are characters
			expIds: []string{"a", "b"},
		},
		"two sub-leaders with own subgroups": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				d := newTestCI("d", "D")
				e := newTestCI("e", "E")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(a)
				a.SetFollowerGrouped("c", true)
				d.SetFollowing(b)
				b.SetFollowerGrouped("d", true)
				e.SetFollowing(c)
				c.SetFollowerGrouped("e", true)
				return a
			},
			expIds: []string{"a", "b", "c", "d", "e"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			leader := tt.setup()
			pg := GroupPublishTarget(leader)

			var gotIds []string
			pg.ForEachPlayer(func(id string, _ *CharacterInstance) {
				gotIds = append(gotIds, id)
			})

			if len(gotIds) != len(tt.expIds) {
				t.Fatalf("expected %d publish targets, got %d: %v", len(tt.expIds), len(gotIds), gotIds)
			}

			idSet := make(map[string]bool)
			for _, id := range gotIds {
				idSet[id] = true
			}
			for _, id := range tt.expIds {
				if !idSet[id] {
					t.Errorf("expected publish target %q not found in %v", id, gotIds)
				}
			}
		})
	}
}

var _ Actor = (*CharacterInstance)(nil)
var _ Actor = (*MobileInstance)(nil)

func TestActorInstance_SetResource(t *testing.T) {
	const resourceMax = 20
	perks := []assets.Perk{
		{Type: assets.PerkTypeModifier, Key: assets.BuildKey(assets.ResourcePrefix, "a", assets.ResourceAspectMax), Value: resourceMax},
	}

	tests := map[string]struct {
		set     int
		wantCur int
	}{
		"within range":      {set: 10, wantCur: 10},
		"clamps above max":  {set: resourceMax + 5, wantCur: resourceMax},
		"clamps below zero": {set: -5, wantCur: 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{PerkCache: *NewPerkCache(perks, nil)}
			a.initResources()

			a.SetResource("a", tc.set)

			cur, mx := a.Resource("a")
			if cur != tc.wantCur {
				t.Errorf("current = %d, want %d", cur, tc.wantCur)
			}
			if mx != resourceMax {
				t.Errorf("max = %d, want %d", mx, resourceMax)
			}
		})
	}
}

func TestActorInstance_AdjustResource(t *testing.T) {
	const resourceMax = 20
	withResource := []assets.Perk{
		{Type: assets.PerkTypeModifier, Key: assets.BuildKey(assets.ResourcePrefix, "a", assets.ResourceAspectMax), Value: resourceMax},
	}

	tests := map[string]struct {
		perks    []assets.Perk
		startCur int
		delta    int
		overfill bool
		wantCur  int
		wantMax  int
	}{
		"positive delta":               {perks: withResource, startCur: 10, delta: 5, wantCur: 15, wantMax: resourceMax},
		"negative delta":               {perks: withResource, startCur: 10, delta: -3, wantCur: 7, wantMax: resourceMax},
		"clamps at zero":               {perks: withResource, startCur: 5, delta: -10, wantCur: 0, wantMax: resourceMax},
		"clamps at max":                {perks: withResource, startCur: 15, delta: 10, wantCur: resourceMax, wantMax: resourceMax},
		"overfill exceeds max":         {perks: withResource, startCur: 15, delta: 10, overfill: true, wantCur: 25, wantMax: resourceMax},
		"non-existent resource no-op":  {perks: nil, startCur: 0, delta: 10, wantCur: 0, wantMax: 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{PerkCache: *NewPerkCache(tc.perks, nil)}
			a.initResources()
			a.setResourceCurrent("a", tc.startCur)

			a.AdjustResource("a", tc.delta, tc.overfill)

			cur, mx := a.Resource("a")
			if cur != tc.wantCur {
				t.Errorf("current = %d, want %d", cur, tc.wantCur)
			}
			if mx != tc.wantMax {
				t.Errorf("max = %d, want %d", mx, tc.wantMax)
			}
		})
	}
}

func TestActorInstance_SetCommander(t *testing.T) {
	tests := map[string]struct {
		setNil bool
	}{
		"sets non-nil commander": {setNil: false},
		"sets nil commander":     {setNil: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
			var fc Commander
			if !tc.setNil {
				fc = &fakeCommander{}
			}
			a.SetCommander(fc)
			if a.commander != fc {
				t.Errorf("commander = %v, want %v", a.commander, fc)
			}
		})
	}
}

func TestActorInstance_ClaimDeath(t *testing.T) {
	tests := map[string]struct {
		calls    int
		wantFirst bool
	}{
		"first call returns true":  {calls: 1, wantFirst: true},
		"second call returns false": {calls: 2, wantFirst: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
			var got bool
			for i := 0; i < tc.calls; i++ {
				got = a.ClaimDeath()
			}
			wantLast := tc.calls == 1 && tc.wantFirst
			if tc.calls > 1 {
				wantLast = false
			}
			if tc.calls == 1 && got != tc.wantFirst {
				t.Errorf("ClaimDeath() = %v, want %v", got, tc.wantFirst)
			}
			if tc.calls > 1 && got != wantLast {
				t.Errorf("ClaimDeath() on call %d = %v, want %v", tc.calls, got, wantLast)
			}
		})
	}
}

func TestActorInstance_QueueTickMsg(t *testing.T) {
	tests := map[string]struct {
		msgs     []string
		wantLen  int
	}{
		"single message queued":   {msgs: []string{"hello"}, wantLen: 1},
		"two messages queued":     {msgs: []string{"a", "b"}, wantLen: 2},
		"no messages queued":      {msgs: nil, wantLen: 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
			for _, m := range tc.msgs {
				a.QueueTickMsg(m)
			}
			if got := len(a.tickMsgBuf); got != tc.wantLen {
				t.Errorf("tickMsgBuf len = %d, want %d", got, tc.wantLen)
			}
		})
	}
}

func TestActorInstance_regenTick(t *testing.T) {
	const maxHP = 20
	hpPerks := []assets.Perk{
		{Type: assets.PerkTypeModifier, Key: assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectMax), Value: maxHP},
		{Type: assets.PerkTypeModifier, Key: assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectRegen), Value: 3},
	}
	tests := map[string]struct {
		startHP  int
		wantHP   int
	}{
		"regen applied when below max": {startHP: 10, wantHP: 13},
		"regen does not exceed max":    {startHP: 19, wantHP: maxHP},
		"full HP stays full":           {startHP: maxHP, wantHP: maxHP},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a := &ActorInstance{PerkCache: *NewPerkCache(hpPerks, nil)}
			a.initResources()
			a.setResourceCurrent(assets.ResourceHp, tc.startHP)
			a.mu.Lock()
			a.regenTick()
			a.mu.Unlock()
			cur, _ := a.Resource(assets.ResourceHp)
			if cur != tc.wantHP {
				t.Errorf("HP after regenTick = %d, want %d", cur, tc.wantHP)
			}
		})
	}
}

func TestActorInstance_autoUseTick(t *testing.T) {
	ctx := context.Background()
	tests := map[string]struct {
		grants        []string
		preTicks      int // ticks to run before checking, to advance cooldowns
		wantAbilities []string
	}{
		"no grants fires nothing": {
			grants:        nil,
			wantAbilities: nil,
		},
		"single grant fires ability": {
			grants:        []string{"heal"},
			wantAbilities: []string{"heal"},
		},
		"grant with cooldown:2 fires on first tick then skips next": {
			grants:        []string{"smite:2"},
			preTicks:      0,
			wantAbilities: []string{"smite"},
		},
		"grant on cooldown does not fire": {
			grants:        []string{"smite:3"},
			preTicks:      1, // fire once to set cooldown, then check second tick
			wantAbilities: nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fc := &fakeCommander{}
			a := &ActorInstance{PerkCache: *NewPerkCache(nil, nil)}
			a.commander = fc
			target := newTestMI("t", "Target")

			// Pre-ticks to advance cooldown state.
			for range tc.preTicks {
				a.autoUseTick(ctx, tc.grants, target)
			}
			fc.abilities = nil // reset after pre-ticks

			a.autoUseTick(ctx, tc.grants, target)

			if len(fc.abilities) != len(tc.wantAbilities) {
				t.Fatalf("abilities fired = %v, want %v", fc.abilities, tc.wantAbilities)
			}
			for i, want := range tc.wantAbilities {
				if fc.abilities[i] != want {
					t.Errorf("ability[%d] = %q, want %q", i, fc.abilities[i], want)
				}
			}
		})
	}
}
