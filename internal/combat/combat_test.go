package combat

import "testing"

// mockCombatant implements Combatant for testing.
type mockCombatant struct {
	name    string
	hp      int
	ac      int
	attacks []Attack
	threatMod int
}

func (m *mockCombatant) CombatID() string    { return m.name }
func (m *mockCombatant) CombatName() string  { return m.name }
func (m *mockCombatant) IsAlive() bool       { return m.hp > 0 }
func (m *mockCombatant) AC() int             { return m.ac }
func (m *mockCombatant) Attacks() []Attack   { return m.attacks }
func (m *mockCombatant) AdjustHP(delta int)  { m.hp += delta }
func (m *mockCombatant) SetInCombat(v bool)  {}
func (m *mockCombatant) Level() int          { return 1 }
func (m *mockCombatant) ThreatModifier() int { return m.threatMod }

func TestScaledThreat(t *testing.T) {
	tests := map[string]struct {
		amount   int
		modifier int
		want     int
	}{
		"zero amount returns 0":           {amount: 0, modifier: 0, want: 0},
		"negative amount returns 0":       {amount: -5, modifier: 0, want: 0},
		"no modifier passes through":      {amount: 100, modifier: 0, want: 100},
		"+50% modifier":                   {amount: 100, modifier: 50, want: 150},
		"-50% modifier":                   {amount: 100, modifier: -50, want: 50},
		"small amount floors to 1":        {amount: 1, modifier: -90, want: 1},
		"modifier reduces below 1 → 1":   {amount: 5, modifier: -99, want: 1},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := scaledThreat(tc.amount, tc.modifier)
			if got != tc.want {
				t.Errorf("scaledThreat(%d, %d) = %d, want %d", tc.amount, tc.modifier, got, tc.want)
			}
		})
	}
}

func TestExecuteAttacks_HitAndMiss(t *testing.T) {
	// Target with very high AC ensures misses; target with AC=0 ensures hits.
	tests := map[string]struct {
		targetAC      int
		expectDamage  bool
	}{
		"impossible AC always misses": {targetAC: 100, expectDamage: false},
		"AC 0 always hits":            {targetAC: 0, expectDamage: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			attacker := &mockCombatant{
				name: "attacker",
				hp:   100,
				attacks: []Attack{
					{Mod: 0, DamageDice: 1, DamageSides: 4, DamageMod: 0},
				},
			}
			target := &mockCombatant{
				name: "target",
				hp:   100,
				ac:   tc.targetAC,
			}
			msgs, dmg := executeAttacks(attacker, target)
			if len(msgs) == 0 {
				t.Error("expected at least one attack message")
			}
			if tc.expectDamage && dmg == 0 {
				t.Errorf("expected damage > 0 with AC=%d", tc.targetAC)
			}
			if !tc.expectDamage && dmg != 0 {
				t.Errorf("expected no damage with AC=%d, got %d", tc.targetAC, dmg)
			}
		})
	}
}

func TestExecuteAttacks_StopsWhenTargetDies(t *testing.T) {
	attacker := &mockCombatant{
		name: "attacker",
		hp:   100,
		attacks: []Attack{
			{Mod: 100, DamageDice: 1, DamageSides: 1, DamageMod: 100}, // always hits, big damage
			{Mod: 100, DamageDice: 1, DamageSides: 1, DamageMod: 100},
		},
	}
	// Target with 1 HP dies on first hit; second attack should not fire.
	target := &mockCombatant{name: "target", hp: 1, ac: 0}
	msgs, _ := executeAttacks(attacker, target)
	// Should get exactly 1 message (the first hit kills target, second is skipped)
	if len(msgs) != 1 {
		t.Errorf("expected 1 attack message (stopped after kill), got %d", len(msgs))
	}
}

func TestExecuteAttacks_MultipleAttacks(t *testing.T) {
	attacker := &mockCombatant{
		name: "attacker",
		hp:   100,
		attacks: []Attack{
			{Mod: 100, DamageDice: 1, DamageSides: 1, DamageMod: 0},
			{Mod: 100, DamageDice: 1, DamageSides: 1, DamageMod: 0},
			{Mod: 100, DamageDice: 1, DamageSides: 1, DamageMod: 0},
		},
	}
	target := &mockCombatant{name: "target", hp: 1000, ac: 0}
	msgs, _ := executeAttacks(attacker, target)
	if len(msgs) != 3 {
		t.Errorf("expected 3 attack messages, got %d", len(msgs))
	}
}
