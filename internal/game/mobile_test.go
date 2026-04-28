package game

import (
	"context"
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestNewMobileInstance(t *testing.T) {
	tests := map[string]struct {
		level int
	}{
		"level set from definition": {level: 5},
		"level zero":                {level: 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ref := storage.NewResolvedSmartIdentifier("test-mob", &assets.Mobile{
				ShortDesc: "a goblin",
				Level:     tc.level,
			})
			mi, err := NewMobileInstance(ref)
			if err != nil {
				t.Fatalf("NewMobileInstance: %v", err)
			}
			if mi.Level() != tc.level {
				t.Errorf("Level() = %d, want %d", mi.Level(), tc.level)
			}
			if mi.Id() == "" {
				t.Error("Id() should not be empty")
			}
		})
	}
}

func TestMobileInstance_Name(t *testing.T) {
	tests := map[string]struct {
		shortDesc string
	}{
		"returns short desc": {shortDesc: "a fierce goblin"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mi := newTestMI("mob", "a test mob")
			mi.Mobile.Get().ShortDesc = tc.shortDesc
			if got := mi.Name(); got != tc.shortDesc {
				t.Errorf("Name() = %q, want %q", got, tc.shortDesc)
			}
		})
	}
}

func TestMobileInstance_Flags(t *testing.T) {
	tests := map[string]struct {
		setup     func(*MobileInstance)
		wantFlags []string
	}{
		"no flags when idle":           {setup: func(*MobileInstance) {}, wantFlags: nil},
		"fighting flag when in combat": {
			setup: func(mi *MobileInstance) {
				enemy := newTestMI("enemy", "enemy")
				mi.EnsureThreat(enemy.Id(), enemy)
			},
			wantFlags: []string{"fighting"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mi := newTestMI("mob", "a test mob")
			tc.setup(mi)
			got := mi.Flags()
			if len(got) != len(tc.wantFlags) {
				t.Fatalf("Flags() = %v, want %v", got, tc.wantFlags)
			}
			for i, f := range got {
				if f != tc.wantFlags[i] {
					t.Errorf("Flags()[%d] = %q, want %q", i, f, tc.wantFlags[i])
				}
			}
		})
	}
}

func TestMobileInstance_IsCharacter(t *testing.T) {
	tests := map[string]struct {
		want bool
	}{
		"mobs are not characters": {want: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mi := newTestMI("mob", "a test mob")
			if got := mi.IsCharacter(); got != tc.want {
				t.Errorf("IsCharacter() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMobileInstance_SpendAP(t *testing.T) {
	tests := map[string]struct {
		cost int
		want bool
	}{
		"always returns true regardless of cost": {cost: 100, want: true},
		"zero cost also returns true":            {cost: 0, want: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mi := newTestMI("mob", "a test mob")
			if got := mi.SpendAP(tc.cost); got != tc.want {
				t.Errorf("SpendAP(%d) = %v, want %v", tc.cost, got, tc.want)
			}
		})
	}
}


func TestNewCorpse(t *testing.T) {
	tests := map[string]struct {
		inventoryItems []string
		equippedItems  []string
		wantItemCount  int
	}{
		"empty mob yields empty corpse": {
			wantItemCount: 0,
		},
		"inventory items move to corpse": {
			inventoryItems: []string{"sword", "potion"},
			wantItemCount:  2,
		},
		"equipped items move to corpse": {
			equippedItems: []string{"helm", "boots"},
			wantItemCount: 2,
		},
		"both inventory and equipped items": {
			inventoryItems: []string{"coin"},
			equippedItems:  []string{"ring"},
			wantItemCount:  2,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ref := storage.NewResolvedSmartIdentifier("test-mob", &assets.Mobile{ShortDesc: "a test mob", Level: 5})
			mi, _ := NewMobileInstance(ref)
			for _, id := range tc.inventoryItems {
				mi.inventory.AddObj(newTestObj(id))
			}
			for _, id := range tc.equippedItems {
				mi.equipment.equip("slot", newTestObj(id))
			}

			corpse := newCorpse(mi)

			if corpse == nil {
				t.Fatal("newCorpse returned nil")
			}
			if corpse.Contents == nil {
				t.Fatal("corpse Contents is nil")
			}
			if !strings.Contains(corpse.Object.Get().ShortDesc, "test mob") {
				t.Errorf("ShortDesc %q does not contain mob name", corpse.Object.Get().ShortDesc)
			}
			if !corpse.Object.Get().HasFlag(assets.ObjectFlagContainer) {
				t.Error("corpse does not have container flag")
			}

			var count int
			corpse.Contents.ForEachObj(func(_ string, _ *ObjectInstance) { count++ })
			if count != tc.wantItemCount {
				t.Errorf("corpse item count = %d, want %d", count, tc.wantItemCount)
			}

			// Mob's inventory and equipment should be drained.
			if mi.inventory.Len() != 0 {
				t.Errorf("mob inventory not drained: %d items remain", mi.inventory.Len())
			}
			if mi.equipment.Len() != 0 {
				t.Errorf("mob equipment not drained: %d items remain", mi.equipment.Len())
			}
		})
	}
}

func TestMobileInstance_OnDeath(t *testing.T) {
	// Build a room with the mob in it.
	room := storage.NewResolvedSmartIdentifier("test-room", &assets.Room{
		Name: "Test Room",
	})
	ri, _ := NewRoomInstance(room)

	ref := storage.NewResolvedSmartIdentifier("test-mob", &assets.Mobile{ShortDesc: "a test mob", Level: 3})
	mi, _ := NewMobileInstance(ref)
	mi.inventory.AddObj(newTestObj("coin"))
	ri.mobiles[mi.Id()] = mi

	drops := mi.OnDeath()

	if len(drops) != 1 {
		t.Fatalf("OnDeath returned %d drops, want 1", len(drops))
	}
	corpse := drops[0]
	if corpse == nil {
		t.Fatal("drop is nil")
	}
	var itemCount int
	corpse.Contents.ForEachObj(func(_ string, _ *ObjectInstance) { itemCount++ })
	if itemCount != 1 {
		t.Errorf("corpse has %d items, want 1", itemCount)
	}
}

func TestMobileInstance_Notify(t *testing.T) {
	tests := map[string]struct{}{
		"no-op for mob": {},
	}
	for name := range tests {
		t.Run(name, func(t *testing.T) {
			mi := newTestMI("mob", "a mob")
			mi.Notify("any message") // must not panic
		})
	}
}

func TestMobileInstance_QueueTickMsg(t *testing.T) {
	tests := map[string]struct{}{
		"no-op for mob": {},
	}
	for name := range tests {
		t.Run(name, func(t *testing.T) {
			mi := newTestMI("mob", "a mob")
			mi.QueueTickMsg("any message") // must not panic
			if len(mi.tickMsgBuf) != 0 {
				t.Errorf("tickMsgBuf should remain empty for mobs, got %d entries", len(mi.tickMsgBuf))
			}
		})
	}
}

func TestMobileInstance_sweepDeadEnemies(t *testing.T) {
	tests := map[string]struct {
		killEnemy    bool
		wantInTable  bool
		wantInCombat bool
	}{
		"alive enemy stays in table":           {killEnemy: false, wantInTable: true, wantInCombat: true},
		"dead enemy removed and combat cleared": {killEnemy: true, wantInTable: false, wantInCombat: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room := newTestRoom("room")

			mi := newTestMI("mob", "mob")
			room.AddMob(mi)

			enemy := newEnemyMI("enemy")
			if tc.killEnemy {
				enemy.setResourceCurrent(assets.ResourceHp, 0)
			}
			room.AddMob(enemy)

			mi.EnsureThreat("enemy", enemy)
			mi.sweepDeadEnemies()

			if got := mi.HasThreatFrom("enemy"); got != tc.wantInTable {
				t.Errorf("HasThreatFrom(enemy) = %v, want %v", got, tc.wantInTable)
			}
			if got := mi.IsInCombat(); got != tc.wantInCombat {
				t.Errorf("IsInCombat() = %v, want %v", got, tc.wantInCombat)
			}
		})
	}
}

func TestMobileInstance_combatTick(t *testing.T) {
	ctx := context.Background()
	tests := map[string]struct {
		hasTarget    bool
		wantInCombat bool
		wantAbility  bool
	}{
		"no target clears combat":       {hasTarget: false, wantInCombat: false, wantAbility: false},
		"with target executes auto-use": {hasTarget: true, wantInCombat: true, wantAbility: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			autoUsePerk := assets.Perk{Type: assets.PerkTypeGrant, Key: assets.PerkGrantAutoUse, Arg: "attack"}
			fc := &fakeCommander{}
			mi := newTestMI("mob", "mob")
			mi.commander = fc

			if tc.hasTarget {
				mi.SetOwn([]assets.Perk{autoUsePerk})
				mi.EnsureThreat("target", newEnemyMI("target"))
			}

			mi.combatTick(ctx, "")

			if got := mi.IsInCombat(); got != tc.wantInCombat {
				t.Errorf("IsInCombat() = %v, want %v", got, tc.wantInCombat)
			}
			if tc.wantAbility && len(fc.abilities) == 0 {
				t.Error("expected ability to fire, got none")
			}
			if !tc.wantAbility && len(fc.abilities) != 0 {
				t.Errorf("expected no abilities, got %v", fc.abilities)
			}
		})
	}
}

func TestMobileInstance_Tick(t *testing.T) {
	ctx := context.Background()
	tests := map[string]struct {
		nilCommander bool
		itemLifetime int
		wantItemGone bool
	}{
		"nil commander logs error and returns": {nilCommander: true},
		"out of combat ticks inventory decay": {
			itemLifetime: 1,
			wantItemGone: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ref := storage.NewResolvedSmartIdentifier("mob", &assets.Mobile{ShortDesc: "a mob"})
			mi, _ := NewMobileInstance(ref)
			mi.randIntN = neverRand // prevent wander/scavenge

			if !tc.nilCommander {
				fc := &fakeCommander{}
				mi.commander = fc
			}
			if tc.itemLifetime > 0 {
				oi := newTestObj("potion", tc.itemLifetime)
				oi.ActivateDecay()
				mi.inventory.AddObj(oi)
			}

			mi.Tick(ctx)

			if tc.itemLifetime > 0 {
				if got := mi.inventory.Len() == 0; got != tc.wantItemGone {
					t.Errorf("item gone = %v, want %v", got, tc.wantItemGone)
				}
			}
		})
	}
}

func TestMobileInstance_tryWander(t *testing.T) {
	ctx := context.Background()
	tests := map[string]struct {
		flags        []string
		setupRoom    func(ri *RoomInstance, destRi *RoomInstance, zi *ZoneInstance)
		randResult   func(int) int
		wantCommands int
	}{
		"sentinel flag skips wander": {
			flags:        []string{"sentinel"},
			randResult:   zeroRand,
			setupRoom:    func(ri, dest *RoomInstance, zi *ZoneInstance) {},
			wantCommands: 0,
		},
		"non-zero rand skips wander": {
			randResult:   neverRand,
			setupRoom:    func(ri, dest *RoomInstance, zi *ZoneInstance) {},
			wantCommands: 0,
		},
		"no exits skips wander": {
			randResult:   zeroRand,
			setupRoom:    func(ri, dest *RoomInstance, zi *ZoneInstance) {},
			wantCommands: 0,
		},
		"closed exit skipped": {
			randResult: zeroRand,
			setupRoom: func(ri, dest *RoomInstance, zi *ZoneInstance) {
				zi.AddRoom(ri)
				zi.AddRoom(dest)
				ri.exits["north"] = &ResolvedExit{Exit: assets.Exit{}, Dest: dest, closed: true}
			},
			wantCommands: 0,
		},
		"valid exit causes wander": {
			randResult: zeroRand,
			setupRoom: func(ri, dest *RoomInstance, zi *ZoneInstance) {
				zi.AddRoom(ri)
				zi.AddRoom(dest)
				ri.exits["north"] = &ResolvedExit{Exit: assets.Exit{}, Dest: dest}
			},
			wantCommands: 1,
		},
		"stay_zone flag skips cross-zone exit": {
			flags:      []string{"stay_zone"},
			randResult: zeroRand,
			setupRoom: func(ri, dest *RoomInstance, zi *ZoneInstance) {
				otherZone := newTestZone("other")
				zi.AddRoom(ri)
				otherZone.AddRoom(dest)
				ri.exits["north"] = &ResolvedExit{Exit: assets.Exit{}, Dest: dest}
			},
			wantCommands: 0,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fc := &fakeCommander{}
			mobDef := storage.NewResolvedSmartIdentifier("mob", &assets.Mobile{
				ShortDesc: "a mob",
				Flags:     tc.flags,
			})
			mi, _ := NewMobileInstance(mobDef)
			mi.commander = fc
			mi.randIntN = tc.randResult

			zi := newTestZone("z")
			ri := newTestRoom("r1")
			dest := newTestRoom("r2")
			tc.setupRoom(ri, dest, zi)

			mi.mu.Lock()
			mi.room = ri
			mi.mu.Unlock()

			mi.tryWander(ctx)

			if got := len(fc.commands); got != tc.wantCommands {
				t.Errorf("commands issued = %d, want %d (commands: %v)", got, tc.wantCommands, fc.commands)
			}
		})
	}
}

func TestMobileInstance_tryScavenge(t *testing.T) {
	ctx := context.Background()
	tests := map[string]struct {
		flags        []string
		roomItem     *ObjectInstance
		randResult   func(int) int
		wantCommands int
	}{
		"non-scavenger skips": {
			randResult:   zeroRand,
			wantCommands: 0,
		},
		"non-zero rand skips": {
			flags:        []string{"scavenger"},
			randResult:   neverRand,
			wantCommands: 0,
		},
		"empty room skips": {
			flags:        []string{"scavenger"},
			randResult:   zeroRand,
			wantCommands: 0,
		},
		"immobile item skipped": {
			flags:      []string{"scavenger"},
			randResult: zeroRand,
			roomItem: func() *ObjectInstance {
				oi, _ := NewObjectInstance(storage.NewResolvedSmartIdentifier("rock", &assets.Object{
					Aliases: []string{"rock"}, ShortDesc: "a rock", Flags: []string{"immobile"},
				}))
				return oi
			}(),
			wantCommands: 0,
		},
		"picks up first valid item": {
			flags:      []string{"scavenger"},
			randResult: zeroRand,
			roomItem:   newTestObj("coin"),
			wantCommands: 1,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fc := &fakeCommander{}
			mobDef := storage.NewResolvedSmartIdentifier("mob", &assets.Mobile{
				ShortDesc: "a mob",
				Flags:     tc.flags,
			})
			mi, _ := NewMobileInstance(mobDef)
			mi.commander = fc
			mi.randIntN = tc.randResult

			ri := newTestRoom("r")
			if tc.roomItem != nil {
				ri.AddObj(tc.roomItem)
			}
			mi.mu.Lock()
			mi.room = ri
			mi.mu.Unlock()

			mi.tryScavenge(ctx)

			if got := len(fc.commands); got != tc.wantCommands {
				t.Errorf("commands issued = %d, want %d (commands: %v)", got, tc.wantCommands, fc.commands)
			}
		})
	}
}

func TestMobileInstance_tryAggro(t *testing.T) {
	tests := map[string]struct {
		flags         []string
		randResult    func(int) int
		mobDead       bool
		mobDarkvision bool
		roomDark      bool
		playerInRoom  bool
		playerDead    bool
		wantAggro     bool
	}{
		"no aggressive flag skips":    {playerInRoom: true},
		"non-zero rand skips aggro":   {flags: []string{"aggressive"}, randResult: neverRand, playerInRoom: true},
		"aggressive with no players":  {flags: []string{"aggressive"}},
		"aggressive attacks living player": {
			flags: []string{"aggressive"}, playerInRoom: true, wantAggro: true,
		},
		"aggressive skips dead player": {
			flags: []string{"aggressive"}, playerInRoom: true, playerDead: true,
		},
		"dead mob does not aggro": {
			flags: []string{"aggressive"}, mobDead: true, playerInRoom: true,
		},
		"blind in dark room skips": {
			flags: []string{"aggressive"}, roomDark: true, playerInRoom: true,
		},
		"darkvision mob aggros in dark room": {
			flags: []string{"aggressive"}, roomDark: true, mobDarkvision: true,
			playerInRoom: true, wantAggro: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			perks := []assets.Perk{testHPPerk}
			if tc.mobDarkvision {
				perks = append(perks, assets.Perk{Type: assets.PerkTypeGrant, Key: assets.PerkGrantIgnoreRestriction, Arg: string(assets.RoomFlagDark)})
			}
			mobDef := storage.NewResolvedSmartIdentifier("mob", &assets.Mobile{
				ShortDesc: "a mob",
				Flags:     tc.flags,
				Perks:     perks,
			})
			mi, _ := NewMobileInstance(mobDef)
			mi.randIntN = zeroRand
			if tc.randResult != nil {
				mi.randIntN = tc.randResult
			}
			if tc.mobDead {
				mi.setResourceCurrent(assets.ResourceHp, 0)
			}

			var roomPerks []assets.Perk
			if tc.roomDark {
				roomPerks = []assets.Perk{{Type: assets.PerkTypeGrant, Key: string(assets.RoomFlagDark)}}
			}
			roomDef := storage.NewResolvedSmartIdentifier("r", &assets.Room{Name: "r", Perks: roomPerks})
			ri, _ := NewRoomInstance(roomDef)
			mi.mu.Lock()
			mi.room = ri
			mi.mu.Unlock()

			var player *CharacterInstance
			if tc.playerInRoom {
				player = newTestCI("p1", "Player")
				player.PerkCache = *NewPerkCache([]assets.Perk{testHPPerk}, nil)
				player.initResources()
				if tc.playerDead {
					player.setResourceCurrent(assets.ResourceHp, 0)
				}
				ri.AddPlayer(player.Id(), player)
			}

			if got := mi.tryAggro(); got != tc.wantAggro {
				t.Errorf("tryAggro() = %v, want %v", got, tc.wantAggro)
			}

			if tc.wantAggro {
				if !mi.HasThreatFrom(player.Id()) {
					t.Error("mob should have threat from player")
				}
				if !player.HasThreatFrom(mi.Id()) {
					t.Error("player should have threat from mob")
				}
			} else {
				if mi.IsInCombat() {
					t.Error("mob should not be in combat")
				}
				if player != nil && player.IsInCombat() {
					t.Error("player should not be in combat")
				}
			}
		})
	}
}

func TestMobileInstance_Move(t *testing.T) {
	tests := map[string]struct {
		name string
	}{
		"moves mob from one room to another": {},
	}
	for name := range tests {
		t.Run(name, func(t *testing.T) {
			from := newTestRoom("from")
			to := newTestRoom("to")
			mi := newTestMI("mob", "a mob")
			from.AddMob(mi)

			mi.Move(from, to)

			if _, inFrom := from.mobiles[mi.Id()]; inFrom {
				t.Error("mob still in from-room after Move")
			}
			if _, inTo := to.mobiles[mi.Id()]; !inTo {
				t.Error("mob not in to-room after Move")
			}
		})
	}
}


func TestMobileInstance_CombatTarget(t *testing.T) {
	tests := map[string]struct {
		setup func(mi *MobileInstance) Actor
	}{
		"nil when not in combat": {
			setup: func(*MobileInstance) Actor { return nil },
		},
		"returns highest-threat enemy regardless of insertion order": {
			setup: func(mi *MobileInstance) Actor {
				low := newEnemyMI("low")
				high := newEnemyMI("high")
				mi.EnsureThreat(low.Id(), low)
				mi.EnsureThreat(high.Id(), high)
				mi.AddThreatFrom(high.Id(), 100)
				return high
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mi := newTestMI("mob", "mob")
			want := tc.setup(mi)
			if got := mi.CombatTarget(); got != want {
				t.Errorf("CombatTarget() = %v, want %v", got, want)
			}
		})
	}
}

func TestMobileInstance_StatSections(t *testing.T) {
	tests := map[string]struct {
		name  string
		level int
	}{
		"returns name and level in first section": {name: "Orc", level: 3},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mob := storage.NewResolvedSmartIdentifier("orc", &assets.Mobile{ShortDesc: tc.name, Level: tc.level})
			mi, _ := NewMobileInstance(mob)

			sections := mi.StatSections()

			if len(sections) == 0 {
				t.Fatal("StatSections() returned no sections")
			}
			foundName, foundLevel := false, false
			for _, line := range sections[0].Lines {
				if line.Value == tc.name {
					foundName = true
				}
				if line.Value == "Level 3" {
					foundLevel = true
				}
			}
			if !foundName {
				t.Errorf("name %q not found in first section", tc.name)
			}
			if !foundLevel {
				t.Errorf("level line not found in first section")
			}
		})
	}
}
