package game

import (
	"context"
	"time"
)

const (
	DefaultTickLength = time.Second * 2
)

type Ticker interface {
	Tick(context.Context) error
}

type MudDriver struct {
	tickLength time.Duration
	handlers   []Ticker
}

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

func (d *MudDriver) Tick(ctx context.Context) error {
	for _, m := range d.handlers {
		err := m.Tick(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
