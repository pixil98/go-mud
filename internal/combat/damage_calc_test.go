package combat

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
)

type mockPerkReader map[string]int

func (m mockPerkReader) ModifierValue(key string) int { return m[key] }

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
