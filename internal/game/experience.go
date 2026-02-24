package game

// MaxLevel is the highest level a character can reach.
const MaxLevel = 20

// levelTable holds the cumulative XP required to reach each level.
// Index 0 = level 1 (0 XP), index 1 = level 2 (300 XP), etc.
var levelTable = [MaxLevel]int{
	0,      // Level 1
	300,    // Level 2
	900,    // Level 3
	2700,   // Level 4
	6500,   // Level 5
	14000,  // Level 6
	23000,  // Level 7
	34000,  // Level 8
	48000,  // Level 9
	64000,  // Level 10
	85000,  // Level 11
	100000, // Level 12
	120000, // Level 13
	140000, // Level 14
	165000, // Level 15
	195000, // Level 16
	225000, // Level 17
	265000, // Level 18
	305000, // Level 19
	355000, // Level 20
}

// ExpForLevel returns the cumulative XP required to reach the given level.
func ExpForLevel(level int) int {
	if level < 1 {
		return 0
	}
	if level > MaxLevel {
		return levelTable[MaxLevel-1]
	}
	return levelTable[level-1]
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

// GroupDivisor returns the XP divisor for N participants. Uses a sub-linear
// curve so grouping is slightly rewarded:
//
//	1 player: /1.0, 2: /1.5, 3: /2.0, 4: /2.5
func GroupDivisor(n int) float64 {
	if n <= 1 {
		return 1.0
	}
	return 1.0 + float64(n-1)*0.5
}

// XPParticipant describes a player who participated in a mob kill.
type XPParticipant struct {
	CombatID string
	Level    int
	Damage   int
}

// XPAward is the XP earned by a single participant.
type XPAward struct {
	CombatID string
	Amount   int
}

// effectiveGroupLevel calculates the damage-weighted average level of
// participants. If a high-level player deals most of the damage, the effective
// level is pulled toward theirs, which crushes the XP via LevelDiffMultiplier.
// This prevents power-leveling: bringing a strong friend makes the mob
// worth much less to everyone.
func effectiveGroupLevel(participants []XPParticipant) float64 {
	totalDamage := 0
	for _, p := range participants {
		totalDamage += p.Damage
	}
	if totalDamage == 0 {
		// No damage recorded (shouldn't happen), use max level as fallback.
		maxLvl := 0
		for _, p := range participants {
			if p.Level > maxLvl {
				maxLvl = p.Level
			}
		}
		return float64(maxLvl)
	}
	weighted := 0.0
	for _, p := range participants {
		weighted += float64(p.Level) * float64(p.Damage)
	}
	return weighted / float64(totalDamage)
}

// CalculateXPAwards determines the XP each participant earns from a mob kill.
// If baseExp is 0, it is calculated from mobLevel.
//
// Anti-power-leveling: the level used for LevelDiffMultiplier is the
// damage-weighted average of all participants' levels, not each player's
// individual level. If a high-level player does most of the damage, the
// effective level is high, making the mob worth little XP for everyone.
func CalculateXPAwards(mobLevel int, baseExp int, participants []XPParticipant) []XPAward {
	if len(participants) == 0 {
		return nil
	}

	if baseExp <= 0 {
		baseExp = BaseExpForLevel(mobLevel)
	}

	groupDiv := GroupDivisor(len(participants))
	effLevel := effectiveGroupLevel(participants)

	// Use a single level-diff multiplier based on the group's effective level.
	mult := LevelDiffMultiplier(int(effLevel+0.5), mobLevel)

	awards := make([]XPAward, 0, len(participants))
	for _, p := range participants {
		xp := int(float64(baseExp) * mult / groupDiv)
		if xp < 1 && mult > 0 {
			xp = 1
		}
		awards = append(awards, XPAward{CombatID: p.CombatID, Amount: xp})
	}
	return awards
}
