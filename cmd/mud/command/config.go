package command

import (
	"errors"
	"fmt"
	"time"
)

// Config holds the top-level server configuration.
type Config struct {
	TickInterval  string              `json:"tick_interval"`
	Listeners     []ListenerConfig    `json:"listeners"`
	Storage       StorageConfig       `json:"storage"`
	Nats          NatsConfig          `json:"nats"`
	PlayerManager PlayerManagerConfig `json:"player_manager"`
}

// Validate checks all sub-configs and ensures tick_interval is a valid duration of at least one second.
func (c *Config) Validate() error {
	var errs []error

	d, err := time.ParseDuration(c.TickInterval)
	if err != nil {
		errs = append(errs, fmt.Errorf("parsing tick_interval: %w", err))
	} else if d < time.Second {
		errs = append(errs, errors.New("tick_interval must be at least 1 second"))
	}

	for i, l := range c.Listeners {
		err := l.validate()
		if err != nil {
			errs = append(errs, fmt.Errorf("listener %d: %w", i, err))
		}
	}

	errs = append(errs, c.Storage.validate())
	errs = append(errs, c.Nats.validate())
	errs = append(errs, c.PlayerManager.validate())

	return errors.Join(errs...)
}
