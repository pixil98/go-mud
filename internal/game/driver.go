package game

import (
	"context"
	"time"
)

const (
	// DefaultTickLength is the default interval between game world ticks.
	DefaultTickLength = time.Second * 2
)

// Ticker is implemented by anything that advances game state on each world tick.
type Ticker interface {
	Tick(context.Context) error
}

// MudDriver runs the main game loop, dispatching periodic ticks to registered handlers.
type MudDriver struct {
	tickLength time.Duration
	handlers   []Ticker
}

// NewMudDriver creates a MudDriver with the given tick handlers and options.
func NewMudDriver(h []Ticker, opts ...MudDriverOpt) *MudDriver {
	d := &MudDriver{
		tickLength: DefaultTickLength,
		handlers:   h,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Start runs the game loop, ticking at regular intervals until ctx is canceled.
func (d *MudDriver) Start(ctx context.Context) error {
	ticker := time.NewTicker(d.tickLength)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := d.Tick(ctx)
			if err != nil {
				return err
			}
		}
	}
}

// Tick advances all registered handlers by one game tick.
func (d *MudDriver) Tick(ctx context.Context) error {
	for _, m := range d.handlers {
		err := m.Tick(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
