package combat

import "testing"

func Test_parseAttackDice(t *testing.T) {
	tests := map[string]struct {
		expr      string
		wantDice  int
		wantSides int
		wantBonus int
		wantOk    bool
	}{
		"simple":         {expr: "2d6", wantDice: 2, wantSides: 6, wantOk: true},
		"with bonus":     {expr: "1d8+3", wantDice: 1, wantSides: 8, wantBonus: 3, wantOk: true},
		"single die":     {expr: "1d4", wantDice: 1, wantSides: 4, wantOk: true},
		"large dice":     {expr: "4d10+5", wantDice: 4, wantSides: 10, wantBonus: 5, wantOk: true},
		"empty":          {expr: "", wantOk: false},
		"invalid format": {expr: "sword", wantOk: false},
		"zero dice":      {expr: "0d6", wantOk: false},
		"zero sides":     {expr: "2d0", wantOk: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dice, sides, bonus, ok := parseAttackDice(tc.expr)
			if ok != tc.wantOk {
				t.Errorf("ok: got %v, want %v", ok, tc.wantOk)
			}
			if !ok {
				return
			}
			if dice != tc.wantDice {
				t.Errorf("dice: got %d, want %d", dice, tc.wantDice)
			}
			if sides != tc.wantSides {
				t.Errorf("sides: got %d, want %d", sides, tc.wantSides)
			}
			if bonus != tc.wantBonus {
				t.Errorf("bonus: got %d, want %d", bonus, tc.wantBonus)
			}
		})
	}
}
