package combat

import (
	"context"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
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
	autoUsesCalled     bool
	autoUsesTargetId   string
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

func (c *mockCombatant) ModifierValue(key string) int {
	return c.modifiers[key]
}

func (c *mockCombatant) GrantArgs(key string) []string {
	return c.grants[key]
}

func (c *mockCombatant) CombatTargetId() string          { return c.combatTargetId }
func (c *mockCombatant) SetCombatTargetId(id string)     { c.combatTargetId = id }
func (c *mockCombatant) AutoUses(targetId string) []string {
	c.autoUsesCalled = true
	c.autoUsesTargetId = targetId
	return nil
}
func (c *mockCombatant) Location() (string, string)      { return c.zoneId, c.roomId }
func (c *mockCombatant) Level() int                      { return c.level }
func (c *mockCombatant) OnDeath() []*game.ObjectInstance { c.deathCalled = true; return nil }

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
		"with threat mod": {sourceThreatMod: 5, amount: 10, want: 15},
		"zero amount":     {amount: 0, want: 0},
		"accumulates":     {amount: 7, want: 7},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m, _ := newTestManager()
			source := newMC("source")
			target := newMC("target")
			source.modifiers[assets.PerkKeyCombatThreatMod] = tc.sourceThreatMod

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

// --- PerformAttack tests ---

func TestPerformAttack(t *testing.T) {
	tests := map[string]struct {
		attackMod   int
		grants      []string // attack grant args
		targetHP    int
		wantHit     bool
		wantHPDrop  bool // target HP decreased
		wantResults int  // number of AttackResult entries
	}{
		"guaranteed hit": {
			attackMod:   100,
			grants:      []string{"1d4"},
			targetHP:    100,
			wantHit:     true,
			wantHPDrop:  true,
			wantResults: 1,
		},
		"guaranteed miss": {
			attackMod:   -100,
			grants:      []string{"1d4"},
			targetHP:    100,
			wantHit:     false,
			wantHPDrop:  false,
			wantResults: 1,
		},
		"two attack grants: two results": {
			attackMod:   100,
			grants:      []string{"1d4", "1d4"},
			targetHP:    1000,
			wantHit:     true,
			wantHPDrop:  true,
			wantResults: 2,
		},
		"no grants: fallback 1d4": {
			attackMod:   100,
			grants:      nil,
			targetHP:    100,
			wantHit:     true,
			wantHPDrop:  true,
			wantResults: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			attacker := newMC("a")
			attacker.modifiers[assets.PerkKeyCombatAttackMod] = tc.attackMod
			attacker.grants[assets.PerkGrantAttack] = tc.grants

			target := newMC("b")
			target.hp = tc.targetHP
			target.hpMax = tc.targetHP

			results := PerformAttack(attacker, target)

			if len(results) != tc.wantResults {
				t.Errorf("results count = %d, want %d", len(results), tc.wantResults)
			}
			for i, r := range results {
				if r.Hit != tc.wantHit {
					t.Errorf("results[%d].Hit = %v, want %v", i, r.Hit, tc.wantHit)
				}
			}
			if tc.wantHPDrop && target.hp >= tc.targetHP {
				t.Errorf("target HP should have dropped: was %d, now %d", tc.targetHP, target.hp)
			}
			if !tc.wantHPDrop && target.hp != tc.targetHP {
				t.Errorf("target HP should be unchanged: was %d, now %d", tc.targetHP, target.hp)
			}
		})
	}
}

// --- Tick tests ---

func TestManager_Tick_CallsAutoUses(t *testing.T) {
	m, _ := newTestManager()
	a := newMC("a")
	b := newMC("b")

	if err := m.StartCombat(a, b); err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	if err := m.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if !a.autoUsesCalled {
		t.Error("AutoUses was not called on combatant a")
	}
	if a.autoUsesTargetId != "b" {
		t.Errorf("AutoUses target = %q, want %q", a.autoUsesTargetId, "b")
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

func TestManager_Tick_EmptyThreatCleanup(t *testing.T) {
	m, _ := newTestManager()
	a := newMC("a")

	m.mu.Lock()
	m.combatants["a"] = &combatantState{c: a, threat: make(map[string]int)}
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
