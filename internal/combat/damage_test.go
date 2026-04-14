package combat

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
)

type mockPerkReader map[string]int

func (m mockPerkReader) ModifierValue(key string) int { return m[key] }

func TestDamageMessagesSorted(t *testing.T) {
	for i := 1; i < len(damageMessages); i++ {
		if damageMessages[i].maxDamage <= damageMessages[i-1].maxDamage {
			t.Errorf("damageMessages[%d].maxDamage (%d) <= damageMessages[%d].maxDamage (%d)",
				i, damageMessages[i].maxDamage, i-1, damageMessages[i-1].maxDamage)
		}
	}
}

func TestDamageVerb(t *testing.T) {
	tests := map[string]struct {
		damage  int
		exp2nd  string
		exp3rd  string
	}{
		"zero":           {damage: 0, exp2nd: "miss", exp3rd: "misses"},
		"low boundary":   {damage: 2, exp2nd: "barely scratch", exp3rd: "barely scratches"},
		"above low":      {damage: 3, exp2nd: "tickle", exp3rd: "tickles"},
		"mid":            {damage: 10, exp2nd: "hit", exp3rd: "hits"},
		"high boundary":  {damage: 80, exp2nd: "annihilate", exp3rd: "annihilates"},
		"above max":      {damage: 81, exp2nd: "do UNSPEAKABLE things to", exp3rd: "does UNSPEAKABLE things to"},
		"well above max": {damage: 999, exp2nd: "do UNSPEAKABLE things to", exp3rd: "does UNSPEAKABLE things to"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := damageVerb2nd(tt.damage); got != tt.exp2nd {
				t.Errorf("damageVerb2nd(%d) = %q, want %q", tt.damage, got, tt.exp2nd)
			}
			if got := damageVerb3rd(tt.damage); got != tt.exp3rd {
				t.Errorf("damageVerb3rd(%d) = %q, want %q", tt.damage, got, tt.exp3rd)
			}
		})
	}
}

func TestCalcDamage(t *testing.T) {
	const dmgType = "test-type"

	// Helper to build keys concisely.
	dmgKey := func(typ, suffix string) string {
		return assets.BuildKey(assets.DamagePrefix, typ, suffix)
	}
	defKey := func(typ, category, suffix string) string {
		return assets.BuildKey(assets.DefensePrefix, typ, category, suffix)
	}

	tests := map[string]struct {
		raw         int
		attacker    mockPerkReader
		target      mockPerkReader
		wantDamage  int
		wantReflect int
	}{
		// Baseline.
		"no modifiers":      {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{}, wantDamage: 10},
		"raw 0 floors at 1": {raw: 0, attacker: mockPerkReader{}, target: mockPerkReader{}, wantDamage: 1},

		// Attacker damage pct.
		"dmg pct type-specific": {raw: 10, attacker: mockPerkReader{
			dmgKey(dmgType, assets.ModSuffixPct): 50,
		}, target: mockPerkReader{}, wantDamage: 15},
		"dmg pct all": {raw: 10, attacker: mockPerkReader{
			dmgKey(assets.DamageTypeAll, assets.ModSuffixPct): 50,
		}, target: mockPerkReader{}, wantDamage: 15},
		"dmg pct type + all stack": {raw: 10, attacker: mockPerkReader{
			dmgKey(dmgType, assets.ModSuffixPct):              25,
			dmgKey(assets.DamageTypeAll, assets.ModSuffixPct): 25,
		}, target: mockPerkReader{}, wantDamage: 15},
		"dmg pct negative": {raw: 10, attacker: mockPerkReader{
			dmgKey(dmgType, assets.ModSuffixPct): -50,
		}, target: mockPerkReader{}, wantDamage: 5},

		// Attacker flat damage bonus.
		"flat type-specific": {raw: 10, attacker: mockPerkReader{
			dmgKey(dmgType, assets.ModSuffixFlat): 3,
		}, target: mockPerkReader{}, wantDamage: 13},
		"flat all": {raw: 10, attacker: mockPerkReader{
			dmgKey(assets.DamageTypeAll, assets.ModSuffixFlat): 3,
		}, target: mockPerkReader{}, wantDamage: 13},
		"flat type + all stack": {raw: 10, attacker: mockPerkReader{
			dmgKey(dmgType, assets.ModSuffixFlat):              2,
			dmgKey(assets.DamageTypeAll, assets.ModSuffixFlat): 3,
		}, target: mockPerkReader{}, wantDamage: 15},
		"flat negative": {raw: 10, attacker: mockPerkReader{
			dmgKey(dmgType, assets.ModSuffixFlat): -5,
		}, target: mockPerkReader{}, wantDamage: 5},
		"flat floors at 1": {raw: 10, attacker: mockPerkReader{
			dmgKey(dmgType, assets.ModSuffixFlat): -100,
		}, target: mockPerkReader{}, wantDamage: 1},

		// Pipeline order: pct before flat.
		// raw=10, pct=50 → 15, flat=2 → 17
		"pct then flat ordering": {raw: 10, attacker: mockPerkReader{
			dmgKey(dmgType, assets.ModSuffixPct):  50,
			dmgKey(dmgType, assets.ModSuffixFlat): 2,
		}, target: mockPerkReader{}, wantDamage: 17},

		// Target pct absorb.
		"absorb pct type-specific": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(dmgType, assets.DefenseCategoryAbsorb, assets.ModSuffixPct): 50,
		}, wantDamage: 5},
		"absorb pct all": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(assets.DamageTypeAll, assets.DefenseCategoryAbsorb, assets.ModSuffixPct): 50,
		}, wantDamage: 5},
		"absorb pct type + all stack": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(dmgType, assets.DefenseCategoryAbsorb, assets.ModSuffixPct):              25,
			defKey(assets.DamageTypeAll, assets.DefenseCategoryAbsorb, assets.ModSuffixPct): 25,
		}, wantDamage: 5},

		// Target flat absorb.
		"absorb flat type-specific": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(dmgType, assets.DefenseCategoryAbsorb, assets.ModSuffixFlat): 3,
		}, wantDamage: 7},
		"absorb flat all": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(assets.DamageTypeAll, assets.DefenseCategoryAbsorb, assets.ModSuffixFlat): 3,
		}, wantDamage: 7},
		"absorb flat type + all stack": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(dmgType, assets.DefenseCategoryAbsorb, assets.ModSuffixFlat):              3,
			defKey(assets.DamageTypeAll, assets.DefenseCategoryAbsorb, assets.ModSuffixFlat): 2,
		}, wantDamage: 5},
		"absorb flat floors at 1": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(dmgType, assets.DefenseCategoryAbsorb, assets.ModSuffixFlat): 100,
		}, wantDamage: 1},

		// Pipeline order: pct absorb before flat absorb.
		// raw=10, absorbPct=50 → 5, flatAbsorb=2 → 3
		"absorb pct then flat ordering": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(dmgType, assets.DefenseCategoryAbsorb, assets.ModSuffixPct):  50,
			defKey(dmgType, assets.DefenseCategoryAbsorb, assets.ModSuffixFlat): 2,
		}, wantDamage: 3},

		// Reflect.
		"reflect less than damage": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(dmgType, assets.DefenseCategoryReflect, assets.ModSuffixFlat): 3,
		}, wantDamage: 10, wantReflect: 3},
		"reflect equals damage": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(dmgType, assets.DefenseCategoryReflect, assets.ModSuffixFlat): 10,
		}, wantDamage: 10, wantReflect: 10},
		"reflect capped at damage": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(dmgType, assets.DefenseCategoryReflect, assets.ModSuffixFlat): 50,
		}, wantDamage: 10, wantReflect: 10},
		"reflect type + all stack": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(dmgType, assets.DefenseCategoryReflect, assets.ModSuffixFlat):              3,
			defKey(assets.DamageTypeAll, assets.DefenseCategoryReflect, assets.ModSuffixFlat): 2,
		}, wantDamage: 10, wantReflect: 5},
		"reflect capped after absorb": {raw: 10, attacker: mockPerkReader{}, target: mockPerkReader{
			defKey(dmgType, assets.DefenseCategoryAbsorb, assets.ModSuffixFlat):  3,
			defKey(dmgType, assets.DefenseCategoryReflect, assets.ModSuffixFlat): 10,
		}, wantDamage: 7, wantReflect: 7},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			damage, reflected := CalcDamage(tc.raw, dmgType, tc.attacker, tc.target)
			if damage != tc.wantDamage {
				t.Errorf("damage = %d, want %d", damage, tc.wantDamage)
			}
			if reflected != tc.wantReflect {
				t.Errorf("reflected = %d, want %d", reflected, tc.wantReflect)
			}
		})
	}
}

func TestCalcThreat(t *testing.T) {
	threatKey := func(suffix string) string {
		return assets.BuildKey(assets.CombatThreatPrefix, suffix)
	}

	tests := map[string]struct {
		raw   int
		actor mockPerkReader
		want  int
	}{
		"no modifiers": {raw: 10, actor: mockPerkReader{}, want: 10},
		"flat bonus": {raw: 10, actor: mockPerkReader{
			threatKey(assets.ModSuffixFlat): 5,
		}, want: 15},
		"flat reduction": {raw: 10, actor: mockPerkReader{
			threatKey(assets.ModSuffixFlat): -3,
		}, want: 7},
		"pct bonus": {raw: 10, actor: mockPerkReader{
			threatKey(assets.ModSuffixPct): 50,
		}, want: 15},
		"pct reduction": {raw: 10, actor: mockPerkReader{
			threatKey(assets.ModSuffixPct): -50,
		}, want: 5},
		"pct then flat": {raw: 10, actor: mockPerkReader{
			threatKey(assets.ModSuffixPct):  50,
			threatKey(assets.ModSuffixFlat): 2,
		}, want: 17},
		"floors at 0": {raw: 10, actor: mockPerkReader{
			threatKey(assets.ModSuffixFlat): -100,
		}, want: 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := CalcThreat(tc.raw, tc.actor)
			if got != tc.want {
				t.Errorf("CalcThreat(%d) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}
