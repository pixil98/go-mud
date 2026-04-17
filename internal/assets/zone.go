package assets

import (
	"errors"
	"fmt"
	"time"
)

// Zone reset mode constants.
const (
	ZoneResetNever    = "never"    // Zone never resets
	ZoneResetLifespan = "lifespan" // Zone resets when lifespan is reached
	ZoneResetEmpty    = "empty"    // Zone resets when lifespan is reached and is empty
)

// Zone represents a region in the game world that contains rooms.
type Zone struct {
	Lifespan  string `json:"lifespan"` // duration string (e.g., "1m", "30s", "2h")
	ResetMode string `json:"reset_mode"`
	Perks     []Perk `json:"perks,omitempty"`
}

// Validate satisfies storage.ValidatingSpec.
func (z *Zone) Validate() error {
	var errs []error

	// Validate reset mode is specified and valid
	switch z.ResetMode {
	case ZoneResetNever, ZoneResetLifespan, ZoneResetEmpty:
		// valid
	case "":
		errs = append(errs, fmt.Errorf("reset_mode is required (must be %s, %s, or %s)",
			ZoneResetNever, ZoneResetLifespan, ZoneResetEmpty))
	default:
		errs = append(errs, fmt.Errorf("invalid reset_mode: %s (must be %s, %s, or %s)",
			z.ResetMode, ZoneResetNever, ZoneResetLifespan, ZoneResetEmpty))
	}

	// Parse and validate lifespan for time-based reset modes
	if z.ResetMode == ZoneResetLifespan || z.ResetMode == ZoneResetEmpty {
		if z.Lifespan == "" {
			errs = append(errs, fmt.Errorf("lifespan is required for reset_mode %s", z.ResetMode))
		} else {
			d, err := time.ParseDuration(z.Lifespan)
			if err != nil {
				errs = append(errs, fmt.Errorf("invalid lifespan %q: %w", z.Lifespan, err))
			} else if d <= 0 {
				errs = append(errs, fmt.Errorf("lifespan must be positive for reset_mode %s", z.ResetMode))
			}
		}
	}

	return errors.Join(errs...)
}
