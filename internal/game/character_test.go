package game

import (
	"context"
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func newTestCharacterInstance() *CharacterInstance {
	char := storage.NewResolvedSmartIdentifier("test-char", &assets.Character{Name: "Tester"})
	room := newTestRoom("test-room")
	ci, _ := NewCharacterInstance(char, make(chan []byte, 1), room)
	return ci
}

func TestNewCharacterInstance_WithInventory(t *testing.T) {
	tests := map[string]struct {
		inventory []assets.ObjectSpawn
		equipment []assets.EquipmentSpawn
		wantItems int
	}{
		"character with inventory items materializes them": {
			inventory: []assets.ObjectSpawn{
				{Object: storage.NewResolvedSmartIdentifier("sword", &assets.Object{Aliases: []string{"sword"}, ShortDesc: "a sword"})},
				{Object: storage.NewResolvedSmartIdentifier("shield", &assets.Object{Aliases: []string{"shield"}, ShortDesc: "a shield"})},
			},
			wantItems: 2,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			char := &assets.Character{Name: "Hero", Inventory: tc.inventory, Equipment: tc.equipment}
			ci, err := NewCharacterInstance(
				storage.NewResolvedSmartIdentifier("hero", char),
				nil,
				newTestRoom("r"),
			)
			if err != nil {
				t.Fatalf("NewCharacterInstance: %v", err)
			}
			count := 0
			ci.inventory.ForEachObj(func(string, *ObjectInstance) { count++ })
			if count != tc.wantItems {
				t.Errorf("inventory count = %d, want %d", count, tc.wantItems)
			}
		})
	}
}


func TestStat_Mod(t *testing.T) {
	tests := map[string]struct {
		score int
		want  int
	}{
		"10 is +0 (average)":          {score: 10, want: 0},
		"12 is +1":                    {score: 12, want: 1},
		"8 is -1":                     {score: 8, want: -1},
		"18 is +4":                    {score: 18, want: 4},
		"1 is -5 (floor, not -4)":     {score: 1, want: -5},
		"9 is -1 (floor, not 0)":      {score: 9, want: -1},
		"11 rounds down to 0":         {score: 11, want: 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := Stat(tc.score).Mod(); got != tc.want {
				t.Errorf("Stat(%d).Mod() = %d, want %d", tc.score, got, tc.want)
			}
		})
	}
}

func TestCharacterInstance_CurrentAP(t *testing.T) {
	tests := map[string]struct {
		perks   []assets.Perk
		spend   int
		wantAP  int
	}{
		"zero after no reset": {
			wantAP: 0,
		},
		"reflects remaining after spend": {
			perks:  []assets.Perk{{Type: assets.PerkTypeModifier, Key: assets.PerkKeyActionPointsMax, Value: 3}},
			spend:  1,
			wantAP: 2,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			if tc.perks != nil {
				ci.SetOwn(tc.perks)
				ci.ResetAP()
			}
			ci.SpendAP(tc.spend)
			if got := ci.CurrentAP(); got != tc.wantAP {
				t.Errorf("CurrentAP() = %d, want %d", got, tc.wantAP)
			}
		})
	}
}

func TestCharacterInstance_IsLinkless_MarkLinkless(t *testing.T) {
	tests := map[string]struct {
		markLinkless bool
		wantLinkless bool
	}{
		"initially not linkless":   {markLinkless: false, wantLinkless: false},
		"linkless after mark":      {markLinkless: true, wantLinkless: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			if tc.markLinkless {
				ci.MarkLinkless()
			}
			if got := ci.IsLinkless(); got != tc.wantLinkless {
				t.Errorf("IsLinkless() = %v, want %v", got, tc.wantLinkless)
			}
		})
	}
}

func TestCharacterInstance_LinklessAt(t *testing.T) {
	tests := map[string]struct {
		markLinkless bool
		wantZero     bool
	}{
		"zero before mark":    {markLinkless: false, wantZero: true},
		"non-zero after mark": {markLinkless: true, wantZero: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			if tc.markLinkless {
				ci.MarkLinkless()
			}
			got := ci.LinklessAt()
			if tc.wantZero && !got.IsZero() {
				t.Errorf("LinklessAt() = %v, want zero", got)
			}
			if !tc.wantZero && got.IsZero() {
				t.Error("LinklessAt() = zero, want non-zero")
			}
		})
	}
}

func TestCharacterInstance_LastActivity_MarkActive(t *testing.T) {
	tests := map[string]struct {
		callMarkActive bool
	}{
		"initially set to now at creation": {callMarkActive: false},
		"updates after MarkActive":         {callMarkActive: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			before := ci.LastActivity()
			if tc.callMarkActive {
				ci.MarkActive()
			}
			after := ci.LastActivity()
			if after.IsZero() {
				t.Error("LastActivity() should not be zero")
			}
			if tc.callMarkActive && !after.Equal(before) && after.Before(before) {
				t.Error("LastActivity() should be >= previous value after MarkActive")
			}
		})
	}
}

func TestCharacterInstance_Reattach(t *testing.T) {
	tests := map[string]struct {
		firstMarkLinkless bool
	}{
		"clears linkless and refreshes channels":            {firstMarkLinkless: true},
		"works even if not previously linkless":             {firstMarkLinkless: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			if tc.firstMarkLinkless {
				ci.MarkLinkless()
			}

			newMsgs := make(chan []byte, 1)
			ci.Reattach(newMsgs)

			if ci.IsLinkless() {
				t.Error("IsLinkless() = true after Reattach, want false")
			}
			if !ci.LinklessAt().IsZero() {
				t.Errorf("LinklessAt() = %v after Reattach, want zero", ci.LinklessAt())
			}
			// done channel should be open (not closed)
			select {
			case <-ci.Done():
				t.Error("Done() should not be closed after Reattach")
			default:
			}
		})
	}
}

func TestCharacterInstance_Subscribe_Unsubscribe(t *testing.T) {
	tests := map[string]struct {
		subject        string
		nilSubscriber  bool
		subscribeTwice bool
		unsubscribe    bool
		wantInSubs     bool
		wantErr        bool
	}{
		"subscribe adds to subs":              {subject: "room.1", wantInSubs: true},
		"unsubscribe removes from subs":       {subject: "room.1", unsubscribe: true, wantInSubs: false},
		"unsubscribe nonexistent is no-op":    {subject: "nonexistent", unsubscribe: true, wantInSubs: false},
		"nil subscriber returns error":        {subject: "room.1", nilSubscriber: true, wantErr: true, wantInSubs: false},
		"re-subscribe replaces old unsub":     {subject: "room.1", subscribeTwice: true, wantInSubs: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			msgs := make(chan []byte, 1)
			char := storage.NewResolvedSmartIdentifier("c", &assets.Character{Name: "T"})
			ci, _ := NewCharacterInstance(char, msgs, newTestRoom("r"))
			if !tc.nilSubscriber {
				ci.subscriber = &fakeSubscriber{}
			}

			if tc.subject != "nonexistent" {
				if tc.subscribeTwice {
					_ = ci.Subscribe(tc.subject)
				}
				err := ci.Subscribe(tc.subject)
				if tc.wantErr && err == nil {
					t.Error("expected error from Subscribe, got nil")
				}
				if !tc.wantErr && err != nil {
					t.Fatalf("Subscribe: %v", err)
				}
			}
			if tc.unsubscribe {
				ci.Unsubscribe(tc.subject)
			}

			_, inSubs := ci.subs[tc.subject]
			if inSubs != tc.wantInSubs {
				t.Errorf("subs[%q] present = %v, want %v", tc.subject, inSubs, tc.wantInSubs)
			}
		})
	}
}

func TestCharacterInstance_UnsubscribeAll(t *testing.T) {
	tests := map[string]struct {
		subjects []string
	}{
		"no subs is a no-op":       {subjects: nil},
		"clears all subscriptions": {subjects: []string{"room.1", "zone.2"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			ci.subscriber = &fakeSubscriber{}
			for _, s := range tc.subjects {
				_ = ci.Subscribe(s)
			}
			ci.UnsubscribeAll()
			if len(ci.subs) != 0 {
				t.Errorf("subs len = %d after UnsubscribeAll, want 0", len(ci.subs))
			}
		})
	}
}

func TestCharacterInstance_OnDeath(t *testing.T) {
	tests := map[string]struct {
		startHP    int
		maxHP      int
		wantMsg    bool
		wantQuit   bool
		wantDone   bool
		wantFullHP bool
	}{
		"death sends message, sets quit, closes done, restores HP": {
			startHP:    0,
			maxHP:      20,
			wantMsg:    true,
			wantQuit:   true,
			wantDone:   true,
			wantFullHP: true,
		},
		"death with nil msgs channel does not panic": {
			startHP:  0,
			maxHP:    20,
			wantQuit: true,
			wantDone: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			hpMaxPerk := assets.Perk{
				Type:  assets.PerkTypeModifier,
				Key:   assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectMax),
				Value: tc.maxHP,
			}
			char := storage.NewResolvedSmartIdentifier("test-char", &assets.Character{Name: "Tester"})
			var msgs chan []byte
			if tc.wantMsg {
				msgs = make(chan []byte, 1)
			}
			ci, _ := NewCharacterInstance(char, msgs, newTestRoom("test-room"))
			ci.SetOwn([]assets.Perk{hpMaxPerk})
			ci.initResources()
			ci.setResourceCurrent(assets.ResourceHp, tc.startHP)

			ci.OnDeath()

			if tc.wantMsg {
				select {
				case msg := <-msgs:
					if len(msg) == 0 {
						t.Error("expected non-empty death message")
					}
				default:
					t.Error("expected message in channel, got none")
				}
			}
			if got := ci.IsQuit(); got != tc.wantQuit {
				t.Errorf("IsQuit() = %v, want %v", got, tc.wantQuit)
			}
			select {
			case <-ci.Done():
				if !tc.wantDone {
					t.Error("done channel closed but should not be")
				}
			default:
				if tc.wantDone {
					t.Error("done channel not closed but should be")
				}
			}
			if tc.wantFullHP {
				cur, mx := ci.Resource(assets.ResourceHp)
				if cur != mx {
					t.Errorf("HP after OnDeath = %d, want %d (max)", cur, mx)
				}
			}
		})
	}
}

func TestCharacterInstance_SpendAP(t *testing.T) {
	tests := map[string]struct {
		startAP int
		spend   int
		wantOk  bool
		wantAP  int
	}{
		"spend within budget": {
			startAP: 3,
			spend:   2,
			wantOk:  true,
			wantAP:  1,
		},
		"spend exact budget": {
			startAP: 2,
			spend:   2,
			wantOk:  true,
			wantAP:  0,
		},
		"spend exceeds budget": {
			startAP: 1,
			spend:   2,
			wantOk:  false,
			wantAP:  1,
		},
		"spend from empty": {
			startAP: 0,
			spend:   1,
			wantOk:  false,
			wantAP:  0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			ci.currentAP = tc.startAP

			got := ci.SpendAP(tc.spend)
			if got != tc.wantOk {
				t.Errorf("SpendAP(%d) = %v, want %v", tc.spend, got, tc.wantOk)
			}
			if ci.currentAP != tc.wantAP {
				t.Errorf("currentAP after SpendAP = %d, want %d", ci.currentAP, tc.wantAP)
			}
		})
	}
}

func TestCharacterInstance_ResetAP(t *testing.T) {
	tests := map[string]struct {
		perks  []assets.Perk
		wantAP int
	}{
		"no perk gives zero": {
			wantAP: 0,
		},
		"perk sets max ap": {
			perks:  []assets.Perk{{Type: assets.PerkTypeModifier, Key: assets.PerkKeyActionPointsMax, Value: 3}},
			wantAP: 3,
		},
		"perk value 1 gives 1": {
			perks:  []assets.Perk{{Type: assets.PerkTypeModifier, Key: assets.PerkKeyActionPointsMax, Value: 1}},
			wantAP: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			ci.SetOwn(tc.perks)
			ci.currentAP = 0 // exhaust AP before reset to confirm it restores

			ci.ResetAP()

			if ci.currentAP != tc.wantAP {
				t.Errorf("currentAP after ResetAP = %d, want %d", ci.currentAP, tc.wantAP)
			}
		})
	}
}

func TestCharacterInstance_Name(t *testing.T) {
	tests := map[string]struct {
		name string
	}{
		"returns character name": {name: "Gandalf"},
		"returns exact case":     {name: "lowerCase"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			char := storage.NewResolvedSmartIdentifier("c", &assets.Character{Name: tc.name})
			ci, _ := NewCharacterInstance(char, nil, newTestRoom("r"))
			if got := ci.Name(); got != tc.name {
				t.Errorf("Name() = %q, want %q", got, tc.name)
			}
		})
	}
}

func TestCharacterInstance_Asset(t *testing.T) {
	tests := map[string]struct {
		charName string
	}{
		"returns pointer to underlying asset": {charName: "Hero"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &assets.Character{Name: tc.charName}
			ci, _ := NewCharacterInstance(storage.NewResolvedSmartIdentifier("c", c), nil, newTestRoom("r"))
			if got := ci.Asset(); got != c {
				t.Error("Asset() returned unexpected pointer")
			}
		})
	}
}

func TestCharacterInstance_Flags(t *testing.T) {
	tests := map[string]struct {
		setup     func(*CharacterInstance)
		wantFlags []string
	}{
		"no flags when idle": {
			setup:     func(*CharacterInstance) {},
			wantFlags: nil,
		},
		"fighting flag when in combat": {
			setup: func(ci *CharacterInstance) {
				enemy := newTestMI("enemy", "enemy")
				ci.EnsureThreat(enemy.Id(), enemy)
			},
			wantFlags: []string{"fighting"},
		},
		"linkless flag when linkless": {
			setup: func(ci *CharacterInstance) {
				ci.mu.Lock()
				ci.linkless = true
				ci.mu.Unlock()
			},
			wantFlags: []string{"linkless"},
		},
		"both flags when in combat and linkless": {
			setup: func(ci *CharacterInstance) {
				enemy := newTestMI("enemy", "enemy")
				ci.EnsureThreat(enemy.Id(), enemy)
				ci.mu.Lock()
				ci.linkless = true
				ci.mu.Unlock()
			},
			wantFlags: []string{"fighting", "linkless"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			tc.setup(ci)
			got := ci.Flags()
			if len(got) != len(tc.wantFlags) {
				t.Fatalf("Flags() = %v, want %v", got, tc.wantFlags)
			}
			flagSet := make(map[string]bool)
			for _, f := range got {
				flagSet[f] = true
			}
			for _, f := range tc.wantFlags {
				if !flagSet[f] {
					t.Errorf("expected flag %q not in %v", f, got)
				}
			}
		})
	}
}

func TestCharacterInstance_EffectiveStats(t *testing.T) {
	tests := map[string]struct {
		baseStats map[assets.StatKey]int
		perks     []assets.Perk
		wantSTR   int
	}{
		"base stats with no modifiers": {
			baseStats: map[assets.StatKey]int{assets.StatSTR: 12},
			wantSTR:   12,
		},
		"stat modifier adds to base": {
			baseStats: map[assets.StatKey]int{assets.StatSTR: 12},
			perks:     []assets.Perk{{Type: assets.PerkTypeModifier, Key: assets.PerkKeySTR, Value: 2}},
			wantSTR:   14,
		},
		"nil base stats treated as zero": {
			baseStats: nil,
			wantSTR:   0,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			char := &assets.Character{Name: "T", BaseStats: tc.baseStats}
			ci, _ := NewCharacterInstance(storage.NewResolvedSmartIdentifier("c", char), nil, newTestRoom("r"))
			if tc.perks != nil {
				ci.SetOwn(tc.perks)
			}
			stats := ci.EffectiveStats()
			if got := int(stats[assets.StatSTR]); got != tc.wantSTR {
				t.Errorf("STR = %d, want %d", got, tc.wantSTR)
			}
		})
	}
}

func TestCharacterInstance_SetTitle(t *testing.T) {
	tests := map[string]struct {
		title string
	}{
		"sets title on asset": {title: "the Brave"},
		"empty title clears":  {title: ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			ci.SetTitle(tc.title)
			if got := ci.Asset().Title; got != tc.title {
				t.Errorf("Title = %q, want %q", got, tc.title)
			}
		})
	}
}

func TestCharacterInstance_Gain(t *testing.T) {
	hpPerk := assets.Perk{
		Type:  assets.PerkTypeModifier,
		Key:   assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectMax),
		Value: 10,
	}
	tests := map[string]struct {
		startLevel int
		startHP    int
		wantLevel  int
		wantFullHP bool
	}{
		"level increments and HP restored to max": {
			startLevel: 1, startHP: 0,
			wantLevel: 2, wantFullHP: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			char := &assets.Character{Name: "T", Level: tc.startLevel}
			ci, _ := NewCharacterInstance(storage.NewResolvedSmartIdentifier("c", char), nil, newTestRoom("r"))
			ci.SetOwn([]assets.Perk{hpPerk})
			ci.initResources()
			ci.setResourceCurrent(assets.ResourceHp, tc.startHP)

			ci.Gain()

			if char.Level != tc.wantLevel {
				t.Errorf("Level = %d, want %d", char.Level, tc.wantLevel)
			}
			if tc.wantFullHP {
				cur, mx := ci.Resource(assets.ResourceHp)
				if cur != mx {
					t.Errorf("HP after Gain = %d/%d, want full", cur, mx)
				}
			}
		})
	}
}

func TestCharacterInstance_Equip(t *testing.T) {
	fingerPerk := assets.Perk{Type: assets.PerkTypeGrant, Key: assets.PerkGrantWearSlot, Arg: "finger"}
	tests := map[string]struct {
		perks    []assets.Perk
		preEquip int
		slot     string
		wantErr  error
	}{
		"no slot grant returns ErrNoSuchSlot": {
			slot:    "finger",
			wantErr: ErrNoSuchSlot,
		},
		"all slots occupied returns ErrSlotFull": {
			perks:    []assets.Perk{fingerPerk},
			preEquip: 1,
			slot:     "finger",
			wantErr:  ErrSlotFull,
		},
		"available slot returns nil": {
			perks:    []assets.Perk{fingerPerk, fingerPerk},
			preEquip: 1,
			slot:     "finger",
			wantErr:  nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			if tc.perks != nil {
				ci.SetOwn(tc.perks)
			}
			for i := 0; i < tc.preEquip; i++ {
				ci.equipment.equip(tc.slot, newTestObj("ring"))
			}
			if err := ci.Equip(tc.slot, newTestObj("ring2")); err != tc.wantErr {
				t.Errorf("Equip() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestCharacterInstance_GainXP(t *testing.T) {
	tests := map[string]struct {
		startLevel     int
		startXP        int
		gainXP         int
		wantXP         int
		wantLevel      int
		wantCanAdvance bool
	}{
		"zero xp is a no-op": {
			startLevel: 1, startXP: 0, gainXP: 0,
			wantXP: 0, wantLevel: 1,
		},
		"negative xp is a no-op": {
			startLevel: 1, startXP: 0, gainXP: -10,
			wantXP: 0, wantLevel: 1,
		},
		"positive xp below level threshold": {
			startLevel: 1, startXP: 0, gainXP: 50,
			wantXP: 50, wantLevel: 1,
		},
		"xp reaching level threshold returns advance flag": {
			startLevel: 1, startXP: 0, gainXP: ExpForLevel(2),
			wantXP: ExpForLevel(2), wantLevel: 1, wantCanAdvance: true,
		},
		"at max level xp accumulates but no advance flag": {
			startLevel: MaxLevel, startXP: 0, gainXP: 1000,
			wantXP: 1000, wantLevel: MaxLevel,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			char := &assets.Character{Name: "Tester", Level: tc.startLevel, Experience: tc.startXP}
			ci, _ := NewCharacterInstance(
				storage.NewResolvedSmartIdentifier("test-char", char),
				nil, newTestRoom("r"),
			)

			canAdvance := ci.GainXP(tc.gainXP)

			if char.Experience != tc.wantXP {
				t.Errorf("Experience = %d, want %d", char.Experience, tc.wantXP)
			}
			if char.Level != tc.wantLevel {
				t.Errorf("Level = %d, want %d", char.Level, tc.wantLevel)
			}
			if canAdvance != tc.wantCanAdvance {
				t.Errorf("GainXP() canAdvance = %v, want %v", canAdvance, tc.wantCanAdvance)
			}
		})
	}
}


func TestCharacterInstance_CombatTarget(t *testing.T) {
	tests := map[string]struct {
		setup func(ci *CharacterInstance) Actor
	}{
		"nil when not in combat": {
			setup: func(ci *CharacterInstance) Actor { return nil },
		},
		"returns sticky target set via combatTargetId": {
			setup: func(ci *CharacterInstance) Actor {
				sticky := newEnemyMI("sticky")
				other := newEnemyMI("other")
				ci.EnsureThreat(sticky.Id(), sticky)
				ci.EnsureThreat(other.Id(), other)
				ci.AddThreatFrom(other.Id(), 9999)
				ci.combatTargetId = sticky.Id()
				return sticky
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			want := tc.setup(ci)
			if got := ci.CombatTarget(); got != want {
				t.Errorf("CombatTarget() = %v, want %v", got, want)
			}
		})
	}
}

func TestCharacterInstance_Notify(t *testing.T) {
	tests := map[string]struct {
		nilMsgs bool
		msg     string
		wantMsg bool
	}{
		"nil msgs channel does not panic":   {nilMsgs: true, msg: "hi"},
		"buffered channel receives message": {msg: "hello", wantMsg: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var msgs chan []byte
			if !tc.nilMsgs {
				msgs = make(chan []byte, 1)
			}
			char := storage.NewResolvedSmartIdentifier("c", &assets.Character{Name: "T"})
			ci, _ := NewCharacterInstance(char, msgs, newTestRoom("r"))

			ci.Notify(tc.msg)

			if tc.wantMsg {
				select {
				case got := <-msgs:
					if string(got) != tc.msg {
						t.Errorf("Notify sent %q, want %q", got, tc.msg)
					}
				default:
					t.Error("expected message in channel, got none")
				}
			}
		})
	}
}

func TestCharacterInstance_flushTickMessages(t *testing.T) {
	tests := map[string]struct {
		msgs     []string
		wantSend bool
		wantBody string
	}{
		"empty buffer sends nothing":           {wantSend: false},
		"single message is sent":               {msgs: []string{"hello"}, wantSend: true, wantBody: "hello"},
		"multiple messages joined with newline": {msgs: []string{"a", "b", "c"}, wantSend: true, wantBody: "a\nb\nc"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ch := make(chan []byte, 10)
			char := storage.NewResolvedSmartIdentifier("c", &assets.Character{Name: "T"})
			ci, _ := NewCharacterInstance(char, ch, newTestRoom("r"))
			for _, m := range tc.msgs {
				ci.QueueTickMsg(m)
			}

			ci.flushTickMessages()

			if tc.wantSend {
				select {
				case got := <-ch:
					if string(got) != tc.wantBody {
						t.Errorf("flushed %q, want %q", got, tc.wantBody)
					}
				default:
					t.Error("expected message in channel, got none")
				}
			} else {
				select {
				case got := <-ch:
					t.Errorf("expected no message, got %q", got)
				default:
				}
			}
		})
	}
}

func TestCharacterInstance_Move(t *testing.T) {
	tests := map[string]struct {
		name string
	}{
		"moves player from one room to another": {},
	}
	for name := range tests {
		t.Run(name, func(t *testing.T) {
			from := newTestRoom("from")
			to := newTestRoom("to")
			char := storage.NewResolvedSmartIdentifier("mover", &assets.Character{Name: "Mover"})
			ci, _ := NewCharacterInstance(char, nil, from)
			from.AddPlayer("mover", ci)

			ci.Move(from, to)

			if _, inFrom := from.players["mover"]; inFrom {
				t.Error("player still in from-room after Move")
			}
			if _, inTo := to.players["mover"]; !inTo {
				t.Error("player not in to-room after Move")
			}
			if ci.Room() != to {
				t.Error("ci.Room() does not point to to-room after Move")
			}
		})
	}
}

func TestCharacterInstance_sweepDeadEnemies(t *testing.T) {
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
			ci := newTestCharacterInstance()
			room.AddPlayer(ci.Character.Id(), ci)

			enemy := newEnemyMI("enemy")
			if tc.killEnemy {
				enemy.setResourceCurrent(assets.ResourceHp, 0)
			}
			room.AddMob(enemy)

			ci.EnsureThreat("enemy", enemy)
			ci.sweepDeadEnemies()

			if got := ci.HasThreatFrom("enemy"); got != tc.wantInTable {
				t.Errorf("HasThreatFrom(enemy) = %v, want %v", got, tc.wantInTable)
			}
			if got := ci.IsInCombat(); got != tc.wantInCombat {
				t.Errorf("IsInCombat() = %v, want %v", got, tc.wantInCombat)
			}
		})
	}
}

func TestCharacterInstance_combatTick(t *testing.T) {
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
			ci := newTestCharacterInstance()
			ci.SetCommander(fc)

			if tc.hasTarget {
				ci.SetOwn([]assets.Perk{autoUsePerk})
				ci.EnsureThreat("target", newEnemyMI("target"))
			}

			ci.combatTick(ctx, "")

			if got := ci.IsInCombat(); got != tc.wantInCombat {
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

func TestCharacterInstance_Tick(t *testing.T) {
	ctx := context.Background()
	apPerk := assets.Perk{Type: assets.PerkTypeModifier, Key: assets.PerkKeyActionPointsMax, Value: 3}
	hpPerk := assets.Perk{
		Type:  assets.PerkTypeModifier,
		Key:   assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectMax),
		Value: 20,
	}
	regenPerk := assets.Perk{
		Type:  assets.PerkTypeModifier,
		Key:   assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectRegen),
		Value: 5,
	}
	tests := map[string]struct {
		inCombat   bool
		wantAPReset bool
	}{
		"out of combat resets AP":    {inCombat: false, wantAPReset: true},
		"in combat resets AP":        {inCombat: true, wantAPReset: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fc := &fakeCommander{}
			ci := newTestCharacterInstance()
			ci.SetCommander(fc)
			ci.SetOwn([]assets.Perk{apPerk, hpPerk, regenPerk})
			ci.initResources()
			ci.currentAP = 0 // exhaust AP before tick
			if tc.inCombat {
				ci.EnsureThreat("enemy", newEnemyMI("enemy"))
			}

			ci.Tick(ctx)

			if tc.wantAPReset {
				if ci.currentAP != 3 {
					t.Errorf("currentAP after Tick = %d, want 3", ci.currentAP)
				}
			}
		})
	}
}

func TestCharacterInstance_StatSections(t *testing.T) {
	hpPerk := assets.Perk{
		Type:  assets.PerkTypeModifier,
		Key:   assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectMax),
		Value: 20,
	}
	tests := map[string]struct {
		setup        func(*CharacterInstance)
		wantInFirst  string
		wantSections []string
	}{
		"base: sections include identity, stats, combat, experience": {
			setup:        func(ci *CharacterInstance) {},
			wantInFirst:  "Aragorn",
			wantSections: []string{"Stats", "Combat", "Experience"},
		},
		"with title: name+title in first section": {
			setup: func(ci *CharacterInstance) {
				ci.Asset().Title = "the Brave"
			},
			wantInFirst:  "Aragorn the Brave",
			wantSections: []string{"Stats"},
		},
		"with race: race name in identity line": {
			setup: func(ci *CharacterInstance) {
				ci.Asset().Race = storage.NewResolvedSmartIdentifier("human", &assets.Race{Name: "Human"})
			},
			wantInFirst:  "Human",
			wantSections: []string{"Stats"},
		},
		"with vitals: Vitals section present": {
			setup: func(ci *CharacterInstance) {
				ci.SetOwn([]assets.Perk{hpPerk})
				ci.initResources()
			},
			wantInFirst:  "Aragorn",
			wantSections: []string{"Stats", "Vitals"},
		},
		"at max level: experience shows MAX LEVEL": {
			setup: func(ci *CharacterInstance) {
				ci.Asset().Level = MaxLevel
			},
			wantInFirst:  "Aragorn",
			wantSections: []string{"Experience"},
		},
		"with grant perk: Perks section present": {
			setup: func(ci *CharacterInstance) {
				ci.SetOwn([]assets.Perk{{Type: assets.PerkTypeGrant, Key: "some_ability", Arg: "fireball"}})
			},
			wantInFirst:  "Aragorn",
			wantSections: []string{"Perks"},
		},
		"with non-stat modifier: Modifiers section present": {
			setup: func(ci *CharacterInstance) {
				ci.SetOwn([]assets.Perk{{Type: assets.PerkTypeModifier, Key: "custom.modifier", Value: 5}})
			},
			wantInFirst:  "Aragorn",
			wantSections: []string{"Modifiers"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			char := &assets.Character{Name: "Aragorn", Level: 5}
			ci, _ := NewCharacterInstance(storage.NewResolvedSmartIdentifier("c", char), nil, newTestRoom("r"))
			tc.setup(ci)

			sections := ci.StatSections()

			if len(sections) < 2 {
				t.Fatalf("StatSections() returned %d sections, want >= 2", len(sections))
			}

			// wantInFirst appears somewhere in first section
			found := false
			for _, line := range sections[0].Lines {
				if strings.Contains(line.Value, tc.wantInFirst) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%q not found in first section lines: %v", tc.wantInFirst, sections[0].Lines)
			}

			headers := make(map[string]bool)
			for _, s := range sections {
				headers[s.Header] = true
			}
			for _, want := range tc.wantSections {
				if !headers[want] {
					t.Errorf("missing section header %q; got headers: %v", want, headers)
				}
			}
		})
	}
}

func TestCharacterInstance_SaveCharacter(t *testing.T) {
	tests := map[string]struct {
		addInventory    bool
		addDecay        bool
		addContainer    bool // container with a nested item
		wantInventory   int
	}{
		"empty character saves cleanly":                   {},
		"permanent inventory item is saved":               {addInventory: true, wantInventory: 1},
		"decayable inventory item is excluded":            {addDecay: true, wantInventory: 0},
		"container with contents saved with nested spawn": {addContainer: true, wantInventory: 1},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			char := &assets.Character{Name: "Hero"}
			ci, _ := NewCharacterInstance(
				storage.NewResolvedSmartIdentifier("hero", char),
				nil,
				newTestRoom("r"),
			)

			if tc.addInventory {
				ci.inventory.AddObj(newTestObj("sword"))
			}
			if tc.addDecay {
				ci.inventory.AddObj(newTestObj("potion", 5))
			}
			if tc.addContainer {
				containerObj := storage.NewResolvedSmartIdentifier("box", &assets.Object{
					Aliases: []string{"box"}, ShortDesc: "a box", Flags: []string{"container"},
				})
				containerInst, _ := NewObjectInstance(containerObj)
				containerInst.Contents.AddObj(newTestObj("coin"))
				ci.inventory.AddObj(containerInst)
			}

			store := newFakeStore[*assets.Character](nil)
			if err := ci.SaveCharacter(store); err != nil {
				t.Fatalf("SaveCharacter: %v", err)
			}

			saved, ok := store.saved["hero"]
			if !ok {
				t.Fatal("character not saved to store")
			}
			if len(saved.Inventory) != tc.wantInventory {
				t.Errorf("Inventory len = %d, want %d", len(saved.Inventory), tc.wantInventory)
			}
			if tc.addContainer && len(saved.Inventory) > 0 && len(saved.Inventory[0].Contents) != 1 {
				t.Errorf("container Contents len = %d, want 1", len(saved.Inventory[0].Contents))
			}
		})
	}
}
