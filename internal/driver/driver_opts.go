package driver

import "time"

type MudDriverOpt func(*MudDriver)

func WithTickLength(tickLength time.Duration) MudDriverOpt {
	return func(d *MudDriver) {
		d.tickLength = tickLength
	}
}
