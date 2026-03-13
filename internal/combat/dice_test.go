package combat

import (
	"testing"
)

func TestParseDice(t *testing.T) {
	tests := map[string]struct {
		expr    string
		want    DiceRoll
		wantErr bool
	}{
		"simple 2d6":      {expr: "2d6", want: DiceRoll{Count: 2, Sides: 6, Mod: 0}},
		"1d4 explicit":    {expr: "1d4", want: DiceRoll{Count: 1, Sides: 4, Mod: 0}},
		"d6 omitted count": {expr: "d6", want: DiceRoll{Count: 1, Sides: 6, Mod: 0}},
		"1d4+3 pos mod":   {expr: "1d4+3", want: DiceRoll{Count: 1, Sides: 4, Mod: 3}},
		"2d6-1 neg mod":   {expr: "2d6-1", want: DiceRoll{Count: 2, Sides: 6, Mod: -1}},
		"1d4+0 zero mod":  {expr: "1d4+0", want: DiceRoll{Count: 1, Sides: 4, Mod: 0}},
		"uppercase D":     {expr: "1D8", want: DiceRoll{Count: 1, Sides: 8, Mod: 0}},
		"whitespace":      {expr: " 2d6 ", want: DiceRoll{Count: 2, Sides: 6, Mod: 0}},

		"flat number":    {expr: "25", want: DiceRoll{Count: 0, Sides: 1, Mod: 25}},
		"flat one":       {expr: "1", want: DiceRoll{Count: 0, Sides: 1, Mod: 1}},
		"flat zero":      {expr: "0", want: DiceRoll{Count: 0, Sides: 1, Mod: 0}},

		"empty expr":    {expr: "", wantErr: true},
		"not a number":  {expr: "abc", wantErr: true},
		"zero count":    {expr: "0d6", wantErr: true},
		"negative count": {expr: "-1d6", wantErr: true},
		"zero sides":    {expr: "1d0", wantErr: true},
		"no sides":      {expr: "2d", wantErr: true},
		"invalid mod":   {expr: "1d4+abc", wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ParseDice(tc.expr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseDice(%q): expected error, got nil", tc.expr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDice(%q): unexpected error: %v", tc.expr, err)
			}
			if got != tc.want {
				t.Errorf("ParseDice(%q) = %+v, want %+v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestDiceRoll_Roll_MinimumOne(t *testing.T) {
	// Large negative mod should still return at least 1.
	d := DiceRoll{Count: 1, Sides: 1, Mod: -1000}
	for range 20 {
		if got := d.Roll(); got < 1 {
			t.Errorf("Roll() = %d, want >= 1", got)
		}
	}
}

func TestDiceRoll_Roll_Range(t *testing.T) {
	// 2d6 must always fall in [2, 12].
	d := DiceRoll{Count: 2, Sides: 6, Mod: 0}
	for range 200 {
		got := d.Roll()
		if got < 2 || got > 12 {
			t.Errorf("2d6 Roll() = %d, want [2..12]", got)
		}
	}
}
