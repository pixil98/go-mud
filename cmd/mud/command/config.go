package command

import (
	"fmt"
	"time"

	"github.com/pixil98/go-errors"
)

type Config struct {
	TickInterval  string              `json:"tick_interval"`
	Listeners     []ListenerConfig    `json:"listeners"`
	Storage       StorageConfig       `json:"storage"`
	Nats          NatsConfig          `json:"nats"`
	PlayerManager PlayerManagerConfig `json:"player_manager"`
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
	el.Add(c.PlayerManager.Validate())

	return el.Err()
}

type PlayerManagerConfig struct {
	DefaultZone string `json:"default_zone"`
}

func (c *PlayerManagerConfig) Validate() error {
	el := errors.NewErrorList()

	if c.DefaultZone == "" {
		el.Add(fmt.Errorf("default_zone is required"))
	}

	return el.Err()
}
