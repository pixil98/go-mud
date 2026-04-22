package game

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeTicker records how many times Tick was called.
type fakeTicker struct {
	count int
	err   error
}

func (ft *fakeTicker) Tick(_ context.Context) error {
	ft.count++
	return ft.err
}

func TestNewMudDriver(t *testing.T) {
	tests := map[string]struct {
		tickLength time.Duration
		handlers   int
	}{
		"defaults to DefaultTickLength with no options": {tickLength: DefaultTickLength, handlers: 0},
		"WithTickLength overrides default":              {tickLength: 50 * time.Millisecond, handlers: 1},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			handlers := make([]Ticker, tc.handlers)
			for i := range handlers {
				handlers[i] = &fakeTicker{}
			}
			d := NewMudDriver(handlers, WithTickLength(tc.tickLength))
			if d.tickLength != tc.tickLength {
				t.Errorf("tickLength = %v, want %v", d.tickLength, tc.tickLength)
			}
			if len(d.handlers) != tc.handlers {
				t.Errorf("len(handlers) = %d, want %d", len(d.handlers), tc.handlers)
			}
		})
	}
}

func TestMudDriver_Tick(t *testing.T) {
	tests := map[string]struct {
		handlerErr error
		wantErr    bool
		wantCount  int
	}{
		"all handlers called when no error": {wantCount: 1},
		"error from handler is returned":   {handlerErr: errors.New("boom"), wantErr: true, wantCount: 1},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ft := &fakeTicker{err: tc.handlerErr}
			d := NewMudDriver([]Ticker{ft})

			err := d.Tick(context.Background())

			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if ft.count != tc.wantCount {
				t.Errorf("handler called %d times, want %d", ft.count, tc.wantCount)
			}
		})
	}
}

func TestMudDriver_Start(t *testing.T) {
	ft := &fakeTicker{}
	d := NewMudDriver([]Ticker{ft}, WithTickLength(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := d.Start(ctx); err != nil {
		t.Errorf("Start with cancelled context: %v", err)
	}
}
