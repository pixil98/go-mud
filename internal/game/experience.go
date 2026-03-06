package game

// MaxLevel is the highest level a character can reach.
const MaxLevel = 40

// ExpForLevel returns the cumulative XP required to reach the given level.
// Uses a cubic + quadratic formula: 30*L³ + 20*L². Level 1 requires 0 XP.
//
// Sample values:
//
//	Level  5:    4,250
//	Level 10:   32,000
//	Level 15:  108,750
//	Level 20:  248,000
//	Level 30:  828,000
//	Level 40: 1,952,000
func ExpForLevel(level int) int {
	if level <= 1 {
		return 0
	}
	if level > MaxLevel {
		level = MaxLevel
	}
	return 30*level*level*level + 20*level*level
}

// ExpToNextLevel returns the remaining XP needed to reach the next level.
func ExpToNextLevel(level, experience int) int {
	if level >= MaxLevel {
		return 0
	}
	remaining := ExpForLevel(level+1) - experience
	if remaining < 0 {
		return 0
	}
	return remaining
}

// BaseExpForLevel returns the base XP reward for killing a mob of the given level.
func BaseExpForLevel(level int) int {
	if level < 1 {
		level = 1
	}
	return 50 + level*level*10
}

// LevelDiffMultiplier returns a multiplier based on the difference between
// the player's level and the mob's level:
//
//	mob 3+ above:  1.5  (bonus for punching up)
//	mob 1-2 above: 1.1-1.2
//	mob same:      1.0
//	mob 1-2 below: 0.6-0.7
//	mob 3-5 below: ~0.25-0.5
//	mob 6-9 below: 0.1  (trivial)
//	mob 10+ below: 0.0  (grey con, no XP)
func LevelDiffMultiplier(playerLevel, mobLevel int) float64 {
	diff := mobLevel - playerLevel // positive = mob is higher
	switch {
	case diff >= 3:
		return 1.5
	case diff >= 1:
		return 1.0 + float64(diff)*0.1
	case diff == 0:
		return 1.0
	case diff >= -2:
		return 0.8 + float64(diff)*0.1
	case diff >= -5:
		return 0.5 + float64(diff+2)*0.08
	case diff >= -9:
		return 0.1
	default:
		return 0.0
	}
}
