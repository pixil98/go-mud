package combat

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

// DiceRoll describes a dice expression of the form NdS[+/-M].
type DiceRoll struct {
	Count int
	Sides int
	Mod   int
}

// Roll executes the dice roll and returns the total (minimum 1).
func (d DiceRoll) Roll() int {
	total := d.Mod
	for range d.Count {
		total += rand.IntN(d.Sides) + 1
	}
	if total < 1 {
		total = 1
	}
	return total
}

// ParseDice parses a dice expression such as "2d6", "d8", "1d4+3", or "2d6-1".
// Count defaults to 1 if omitted (e.g. "d6").
func ParseDice(expr string) (DiceRoll, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return DiceRoll{}, fmt.Errorf("empty dice expression")
	}

	dIdx := strings.IndexByte(expr, 'd')
	if dIdx < 0 {
		dIdx = strings.IndexByte(expr, 'D')
	}
	if dIdx < 0 {
		return DiceRoll{}, fmt.Errorf("invalid dice expression %q: missing 'd'", expr)
	}

	var roll DiceRoll
	roll.Count = 1

	if dIdx > 0 {
		n, err := strconv.Atoi(expr[:dIdx])
		if err != nil || n <= 0 {
			return DiceRoll{}, fmt.Errorf("invalid dice expression %q: invalid count", expr)
		}
		roll.Count = n
	}

	rest := expr[dIdx+1:]

	// Split sides from optional +/- modifier.
	// Use LastIndex for '-' to handle negative mod (e.g. "2d6-1" not confused with "d-6").
	plusIdx := strings.IndexByte(rest, '+')
	minusIdx := strings.LastIndexByte(rest, '-')

	modStart := -1
	if plusIdx >= 0 {
		modStart = plusIdx
	} else if minusIdx >= 0 {
		modStart = minusIdx
	}

	var sidesStr string
	if modStart >= 0 {
		sidesStr = rest[:modStart]
		mod, err := strconv.Atoi(rest[modStart:])
		if err != nil {
			return DiceRoll{}, fmt.Errorf("invalid dice expression %q: invalid modifier", expr)
		}
		roll.Mod = mod
	} else {
		sidesStr = rest
	}

	sides, err := strconv.Atoi(sidesStr)
	if err != nil || sides <= 0 {
		return DiceRoll{}, fmt.Errorf("invalid dice expression %q: invalid sides", expr)
	}
	roll.Sides = sides

	return roll, nil
}
