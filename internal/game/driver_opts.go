package game

import "time"

// MudDriverOpt is a functional option for configuring a MudDriver.
type MudDriverOpt func(*MudDriver)

// WithTickLength sets the interval between game world ticks.
func WithTickLength(tickLength time.Duration) MudDriverOpt {
	return func(d *MudDriver) {
		d.tickLength = tickLength
	}
}
