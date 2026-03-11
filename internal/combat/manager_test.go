package combat

import (
	"context"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
)

// --- mock helpers ---

type mockCombatant struct {
	id             string
	name           string
	inCombat       bool
	alive          bool
	hp             int
	hpMax          int
	level          int
	modifiers      map[string]int
	grants         map[string][]string
	combatTargetId     string
	zoneId, roomId     string
	deathCalled        bool
}

func (c *mockCombatant) Id() string   { return c.id }
func (c *mockCombatant) Name() string { return c.name }

func (c *mockCombatant) IsInCombat() bool   { return c.inCombat }
func (c *mockCombatant) SetInCombat(v bool) { c.inCombat = v }

func (c *mockCombatant) IsAlive() bool { return c.alive }

func (c *mockCombatant) Resource(name string) (int, int) {
	if name == assets.ResourceHp {
		return c.hp, c.hpMax
	}
	return 0, 0
}

func (c *mockCombatant) AdjustResource(name string, delta int) {
	if name == assets.ResourceHp {
		c.hp += delta
		if c.hp < 0 {
			c.hp = 0
		}
		if c.hp > c.hpMax {
			c.hp = c.hpMax
		}
		c.alive = c.hp > 0
	}
}

func (c *mockCombatant) SpendAP(_ int) bool              { return true }
func (c *mockCombatant) HasGrant(_, _ string) bool        { return false }
func (c *mockCombatant) AddTimedPerks(_ string, _ []assets.Perk, _ int) {}

func (c *mockCombatant) ModifierValue(key string) int {
	return c.modifiers[key]
}

func (c *mockCombatant) GrantArgs(key string) []string {
	return c.grants[key]
}

func (c *mockCombatant) CombatTargetId() string          { return c.combatTargetId }
func (c *mockCombatant) SetCombatTargetId(id string)     { c.combatTargetId = id }
func (c *mockCombatant) Location() (string, string)      { return c.zoneId, c.roomId }
func (c *mockCombatant) Level() int                      { return c.level }
func (c *mockCombatant) OnDeath() []any { c.deathCalled = true; return nil }

func newMC(id string) *mockCombatant {
	return &mockCombatant{
		id:        id,
		name:      id,
		alive:     true,
		hp:        100,
		hpMax:     100,
		modifiers: make(map[string]int),
		grants:    make(map[string][]string),
	}
}

type mockCombatPub struct {
	count int
}

func (p *mockCombatPub) Publish(_ game.PlayerGroup, _ []string, _ []byte) error {
	p.count++
	return nil
}

type nilZones struct{}

func (n *nilZones) GetZone(string) *game.ZoneInstance { return nil }

func newTestManager() (*Manager, *mockCombatPub) {
	pub := &mockCombatPub{}
	return NewManager(pub, &nilZones{}), pub
}

// --- StartCombat tests ---

func TestManager_StartCombat(t *testing.T) {
	tests := map[string]struct {
		aAlive  bool
		bAlive  bool
		wantErr bool
	}{
		"both alive: success":  {aAlive: true, bAlive: true},
		"dead attacker: error": {aAlive: false, bAlive: true, wantErr: true},
		"dead target: error":   {aAlive: true, bAlive: false, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m, _ := newTestManager()
			a := newMC("a")
			b := newMC("b")
			a.alive = tc.aAlive
			b.alive = tc.bAlive

			err := m.StartCombat(a, b)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !a.inCombat {
				t.Error("attacker not set in combat")
			}
			if !b.inCombat {
				t.Error("target not set in combat")
			}

			m.mu.Lock()
			aState := m.combatants["a"]
			bState := m.combatants["b"]
			m.mu.Unlock()

			if aState == nil || bState == nil {
				t.Fatal("combatants not registered")
			}
			if aState.threat["b"] != 1 {
				t.Errorf("attacker threat toward target = %d, want 1", aState.threat["b"])
			}
			if bState.threat["a"] != 1 {
				t.Errorf("target threat toward attacker = %d, want 1", bState.threat["a"])
			}
		})
	}
}

func TestManager_StartCombat_Idempotent(t *testing.T) {
	m, _ := newTestManager()
	a := newMC("a")
	b := newMC("b")

	if err := m.StartCombat(a, b); err != nil {
		t.Fatalf("first StartCombat: %v", err)
	}

	// Simulate prior combat: raise threat beyond initial 1.
	m.mu.Lock()
	m.combatants["a"].threat["b"] = 50
	m.mu.Unlock()

	if err := m.StartCombat(a, b); err != nil {
		t.Fatalf("second StartCombat: %v", err)
	}

	m.mu.Lock()
	got := m.combatants["a"].threat["b"]
	m.mu.Unlock()

	if got != 50 {
		t.Errorf("idempotent StartCombat overwrote existing threat: got %d, want 50", got)
	}
}

// --- AddThreat tests ---

func TestManager_AddThreat(t *testing.T) {
	tests := map[string]struct {
		sourceThreatMod int
		amount          int
		want            int
	}{
		"basic threat":    {amount: 10, want: 10},
		"with threat mod": {sourceThreatMod: 5, amount: 10, want: 15}, // flat bonus
		"zero amount":     {amount: 0, want: 0},
		"accumulates":     {amount: 7, want: 7},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m, _ := newTestManager()
			source := newMC("source")
			target := newMC("target")
			source.modifiers[assets.BuildKey(assets.CombatThreatPrefix, assets.ModSuffixFlat)] = tc.sourceThreatMod

			m.AddThreat(source, target, tc.amount)

			m.mu.Lock()
			got := m.combatants["target"].threat["source"]
			m.mu.Unlock()

			if got != tc.want {
				t.Errorf("threat = %d, want %d", got, tc.want)
			}
		})
	}
}

// --- SetThreat tests ---

func TestManager_SetThreat(t *testing.T) {
	tests := map[string]struct {
		initial int
		set     int
		want    int
	}{
		"set from zero":    {initial: 0, set: 50, want: 50},
		"overwrite higher": {initial: 100, set: 25, want: 25},
		"set to zero":      {initial: 30, set: 0, want: 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m, _ := newTestManager()
			source := newMC("source")
			target := newMC("target")

			if tc.initial > 0 {
				m.AddThreat(source, target, tc.initial)
			}

			m.SetThreat(source, target, tc.set)

			m.mu.Lock()
			got := m.combatants["target"].threat["source"]
			m.mu.Unlock()

			if got != tc.want {
				t.Errorf("threat = %d, want %d", got, tc.want)
			}
		})
	}
}

// --- TopThreat tests ---

func TestManager_TopThreat(t *testing.T) {
	tests := map[string]struct {
		existing map[string]int // pre-set threat entries on target
		want     int            // expected threat for "source" after TopThreat
	}{
		"empty table": {
			existing: map[string]int{},
			want:     1,
		},
		"already highest": {
			existing: map[string]int{"source": 50, "other": 30},
			want:     51,
		},
		"below other": {
			existing: map[string]int{"source": 10, "other": 100},
			want:     101,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m, _ := newTestManager()
			source := newMC("source")
			target := newMC("target")

			// Pre-populate threat table.
			m.mu.Lock()
			state := m.register(target)
			for k, v := range tc.existing {
				m.register(newMC(k))
				state.threat[k] = v
			}
			m.mu.Unlock()

			m.TopThreat(source, target)

			m.mu.Lock()
			got := m.combatants["target"].threat["source"]
			m.mu.Unlock()

			if got != tc.want {
				t.Errorf("threat = %d, want %d", got, tc.want)
			}
		})
	}
}

// --- Tick tests ---

type mockAbilityHandler struct {
	calls []struct{ abilityId, actorId, targetId string }
}

func (h *mockAbilityHandler) ExecCombatAbility(abilityId string, actor, target shared.Actor) (string, error) {
	h.calls = append(h.calls, struct{ abilityId, actorId, targetId string }{abilityId, actor.Id(), target.Id()})
	return "", nil
}

func TestManager_Tick_CallsAutoUses(t *testing.T) {
	m, _ := newTestManager()
	ah := &mockAbilityHandler{}
	m.SetAbilityHandler(ah)

	a := newMC("a")
	a.grants[assets.PerkGrantAutoUse] = []string{"fireball"}
	b := newMC("b")

	if err := m.StartCombat(a, b); err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	if err := m.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(ah.calls) == 0 {
		t.Fatal("ExecCombatAbility was not called")
	}
	call := ah.calls[0]
	if call.abilityId != "fireball" {
		t.Errorf("abilityId = %q, want %q", call.abilityId, "fireball")
	}
	if call.actorId != "a" {
		t.Errorf("actorId = %q, want %q", call.actorId, "a")
	}
	if call.targetId != "b" {
		t.Errorf("targetId = %q, want %q", call.targetId, "b")
	}
}

func TestManager_Tick_DeathCleanup(t *testing.T) {
	m, _ := newTestManager()
	a := newMC("a")
	b := newMC("b")

	if err := m.StartCombat(a, b); err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	// Simulate b being killed (e.g. by an auto-use ability) before tick processes deaths.
	b.hp = 0
	b.alive = false

	if err := m.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if !b.deathCalled {
		t.Error("target.OnDeath() not called")
	}
	if b.inCombat {
		t.Error("dead target should not be in combat")
	}

	m.mu.Lock()
	_, targetExists := m.combatants["b"]
	_, attackerExists := m.combatants["a"]
	m.mu.Unlock()

	if targetExists {
		t.Error("dead target should be removed from combatants")
	}
	// After b dies, a's threat table is empty → a is also cleaned up.
	if attackerExists {
		t.Error("attacker with empty threat table should be removed from combatants")
	}
}

func TestManager_Tick_AutoUseCooldown(t *testing.T) {
	m, _ := newTestManager()
	ah := &mockAbilityHandler{}
	m.SetAbilityHandler(ah)

	a := newMC("a")
	a.grants[assets.PerkGrantAutoUse] = []string{"fireball:3"}
	b := newMC("b")

	if err := m.StartCombat(a, b); err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	// Tick 1: should fire (cooldown starts at 0).
	if err := m.Tick(context.Background()); err != nil {
		t.Fatalf("Tick 1: %v", err)
	}
	if len(ah.calls) != 1 {
		t.Fatalf("after tick 1: calls = %d, want 1", len(ah.calls))
	}

	// Ticks 2 and 3: cooldown, should NOT fire.
	for tick := 2; tick <= 3; tick++ {
		if err := m.Tick(context.Background()); err != nil {
			t.Fatalf("Tick %d: %v", tick, err)
		}
		if len(ah.calls) != 1 {
			t.Fatalf("after tick %d: calls = %d, want 1", tick, len(ah.calls))
		}
	}

	// Tick 4: cooldown elapsed, should fire again.
	if err := m.Tick(context.Background()); err != nil {
		t.Fatalf("Tick 4: %v", err)
	}
	if len(ah.calls) != 2 {
		t.Fatalf("after tick 4: calls = %d, want 2", len(ah.calls))
	}
}

func TestManager_Tick_DuplicateAutoUseGrants(t *testing.T) {
	m, _ := newTestManager()
	ah := &mockAbilityHandler{}
	m.SetAbilityHandler(ah)

	a := newMC("a")
	a.grants[assets.PerkGrantAutoUse] = []string{"attack", "attack"}
	b := newMC("b")

	if err := m.StartCombat(a, b); err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	if err := m.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(ah.calls) != 2 {
		t.Fatalf("calls = %d, want 2 (one per duplicate grant)", len(ah.calls))
	}
}

func TestManager_Tick_AddDuplicateGrantMidCombat(t *testing.T) {
	m, _ := newTestManager()
	ah := &mockAbilityHandler{}
	m.SetAbilityHandler(ah)

	a := newMC("a")
	a.grants[assets.PerkGrantAutoUse] = []string{"attack"}
	b := newMC("b")

	if err := m.StartCombat(a, b); err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	// Tick 1: one grant, should fire once.
	if err := m.Tick(context.Background()); err != nil {
		t.Fatalf("Tick 1: %v", err)
	}
	if len(ah.calls) != 1 {
		t.Fatalf("after tick 1: calls = %d, want 1", len(ah.calls))
	}

	// Equip second ring mid-combat — adds a duplicate grant.
	a.grants[assets.PerkGrantAutoUse] = []string{"attack", "attack"}

	// Tick 2: both grants should fire.
	if err := m.Tick(context.Background()); err != nil {
		t.Fatalf("Tick 2: %v", err)
	}
	if len(ah.calls) != 3 {
		t.Fatalf("after tick 2: calls = %d, want 3", len(ah.calls))
	}
}

func TestManager_Tick_EmptyThreatCleanup(t *testing.T) {
	m, _ := newTestManager()
	a := newMC("a")

	m.mu.Lock()
	m.combatants["a"] = &combatantState{c: a, threat: make(map[string]int), cooldown: make(map[string][]int)}
	m.mu.Unlock()

	if err := m.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	m.mu.Lock()
	_, exists := m.combatants["a"]
	m.mu.Unlock()

	if exists {
		t.Error("combatant with empty threat table should be removed")
	}
	if a.inCombat {
		t.Error("removed combatant should have inCombat cleared")
	}
}
