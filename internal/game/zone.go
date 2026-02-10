package game

import (
	"fmt"
	"time"

	"github.com/pixil98/go-errors"
)

const (
	ZoneResetNever    = "never"    // Zone never resets
	ZoneResetLifespan = "lifespan" // Zone resets when lifespan is reached
	ZoneResetEmpty    = "empty"    // Zone resets when lifespan is reached and is empty
)

// Zone represents a region in the game world that contains rooms.
type Zone struct {
	Lifespan  string `json:"lifespan"` // duration string (e.g., "1m", "30s", "2h")
	ResetMode string `json:"reset_mode"`

	NextReset        time.Time     `json:"-"` // when zone should next reset (runtime only)
	lifespanDuration time.Duration // parsed lifespan
}

// Validate satisfies storage.ValidatingSpec.
func (z *Zone) Validate() error {
	el := errors.NewErrorList()

	// Validate reset mode is specified and valid
	switch z.ResetMode {
	case ZoneResetNever, ZoneResetLifespan, ZoneResetEmpty:
		// valid
	case "":
		el.Add(fmt.Errorf("reset_mode is required (must be %s, %s, or %s)",
			ZoneResetNever, ZoneResetLifespan, ZoneResetEmpty))
	default:
		el.Add(fmt.Errorf("invalid reset_mode: %s (must be %s, %s, or %s)",
			z.ResetMode, ZoneResetNever, ZoneResetLifespan, ZoneResetEmpty))
	}

	// Parse and validate lifespan for time-based reset modes
	if z.ResetMode == ZoneResetLifespan || z.ResetMode == ZoneResetEmpty {
		if z.Lifespan == "" {
			el.Add(fmt.Errorf("lifespan is required for reset_mode %s", z.ResetMode))
		} else {
			d, err := time.ParseDuration(z.Lifespan)
			if err != nil {
				el.Add(fmt.Errorf("invalid lifespan %q: %w", z.Lifespan, err))
			} else if d <= 0 {
				el.Add(fmt.Errorf("lifespan must be positive for reset_mode %s", z.ResetMode))
			} else {
				z.lifespanDuration = d
			}
		}
	}

	return el.Err()
}

// LifespanDuration returns the parsed lifespan duration.
func (z *Zone) LifespanDuration() time.Duration {
	return z.lifespanDuration
}
