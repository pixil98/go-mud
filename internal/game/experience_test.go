package game

import "testing"

func TestBaseExpForLevel(t *testing.T) {
	tests := map[string]struct {
		level int
		want  int
	}{
		"level 1":              {level: 1, want: 60},
		"level 5":              {level: 5, want: 300},
		"level 10":             {level: 10, want: 1050},
		"level 0 clamps to 1":  {level: 0, want: 60},
		"negative clamps to 1": {level: -5, want: 60},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := BaseExpForLevel(tc.level)
			if got != tc.want {
				t.Errorf("BaseExpForLevel(%d) = %d, want %d", tc.level, got, tc.want)
			}
		})
	}
}

func TestLevelDiffMultiplier(t *testing.T) {
	tests := map[string]struct {
		playerLevel int
		mobLevel    int
		want        float64
	}{
		"mob 3+ above player":  {playerLevel: 5, mobLevel: 8, want: 1.5},
		"mob 2 above player":   {playerLevel: 5, mobLevel: 7, want: 1.2},
		"mob 1 above player":   {playerLevel: 5, mobLevel: 6, want: 1.1},
		"same level":           {playerLevel: 5, mobLevel: 5, want: 1.0},
		"mob 1 below player":   {playerLevel: 5, mobLevel: 4, want: 0.7},
		"mob 2 below player":   {playerLevel: 5, mobLevel: 3, want: 0.6},
		"mob 3 below player":   {playerLevel: 5, mobLevel: 2, want: 0.42},
		"mob 6 below player":   {playerLevel: 10, mobLevel: 4, want: 0.1},
		"mob 10+ below player": {playerLevel: 15, mobLevel: 4, want: 0.0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := LevelDiffMultiplier(tc.playerLevel, tc.mobLevel)
			// Allow small floating point tolerance
			diff := got - tc.want
			if diff < -0.001 || diff > 0.001 {
				t.Errorf("LevelDiffMultiplier(%d, %d) = %v, want %v",
					tc.playerLevel, tc.mobLevel, got, tc.want)
			}
		})
	}
}

func TestExpToNextLevel(t *testing.T) {
	tests := map[string]struct {
		level      int
		experience int
		want       int
	}{
		"fresh level 1 needs 320":      {level: 1, experience: 0, want: 320},
		"level 1 halfway to 2":         {level: 1, experience: 160, want: 160},
		"exactly at level 2 threshold": {level: 1, experience: 320, want: 0},
		"excess XP returns 0":          {level: 1, experience: 500, want: 0},
		"max level returns 0":          {level: MaxLevel, experience: 0, want: 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := ExpToNextLevel(tc.level, tc.experience)
			if got != tc.want {
				t.Errorf("ExpToNextLevel(%d, %d) = %d, want %d",
					tc.level, tc.experience, got, tc.want)
			}
		})
	}
}
