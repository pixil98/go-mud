package command

import (
	"fmt"
	"time"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/nats"
)

type NatsConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	StartTimeout string `json:"start_timeout"`
}

func (n *NatsConfig) validate() error {
	el := errors.NewErrorList()

	if n.StartTimeout != "" {
		_, err := time.ParseDuration(n.StartTimeout)
		if err != nil {
			el.Add(fmt.Errorf("parsing start_timeout: %w", err))
		}
	}

	return el.Err()
}

func (c *NatsConfig) buildNatsServer() (*nats.NatsServer, error) {
	var opts []nats.NatsServerOpt
	if c.StartTimeout != "" {
		d, err := time.ParseDuration(c.StartTimeout)
		if err != nil {
			return nil, fmt.Errorf("parsing start_timeout: %w", err)
		}
		opts = append(opts, nats.WithStartTimeout(d))
	}
	if c.Host != "" {
		opts = append(opts, nats.WithHost(c.Host))
	}
	if c.Port != 0 {
		opts = append(opts, nats.WithPort(c.Port))
	}

	s, err := nats.NewNatsServer(opts...)
	if err != nil {
		return nil, err
	}

	return s, nil
}
