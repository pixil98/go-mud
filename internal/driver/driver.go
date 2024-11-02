package driver

import (
	"context"
	"time"
)

const (
	DefaultTickLength = time.Second * 2
)

type TickHandler interface {
	Tick(context.Context) error
}

type MudDriver struct {
	tickLength time.Duration
	handlers   []TickHandler
}

func NewMudDriver(h []TickHandler, opts ...MudDriverOpt) *MudDriver {
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
		if err := m.Tick(ctx); err != nil {
			return err
		}
	}
	return nil
}
