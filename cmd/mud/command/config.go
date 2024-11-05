package command

import (
	"fmt"
	"time"

	"github.com/pixil98/go-errors/errors"
)

type Config struct {
	TickInterval string           `json:"tick_interval"`
	Listeners    []ListenerConfig `json:"listeners"`
	Storage      StorageConfig    `json:"storage"`
	Nats         NatsConfig       `json:"nats"`
}

func (c *Config) Validate() error {
	el := errors.NewErrorList()

	d, err := time.ParseDuration(c.TickInterval)
	if err != nil {
		el.Add(fmt.Errorf("parsing tick_interval: %w", err))
	} else if d < time.Second {
		el.Add(fmt.Errorf("tick_interval must be at least 1 second"))
	}

	for i, l := range c.Listeners {
		err := l.Validate()
		if err != nil {
			el.Add(fmt.Errorf("listener %d: %w", i, err))
		}
	}

	el.Add(c.Storage.Validate())
	el.Add(c.Nats.Validate())

	return el.Err()
}
