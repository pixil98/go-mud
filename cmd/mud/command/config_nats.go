package command

import (
	"fmt"
	"time"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/messaging"
)

// NatsConfig holds NATS server configuration.
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

func (n *NatsConfig) buildNatsServer() (*messaging.NatsServer, error) {
	var opts []messaging.NatsServerOpt
	if n.StartTimeout != "" {
		d, err := time.ParseDuration(n.StartTimeout)
		if err != nil {
			return nil, fmt.Errorf("parsing start_timeout: %w", err)
		}
		opts = append(opts, messaging.WithStartTimeout(d))
	}
	if n.Host != "" {
		opts = append(opts, messaging.WithHost(n.Host))
	}
	if n.Port != 0 {
		opts = append(opts, messaging.WithPort(n.Port))
	}

	s, err := messaging.NewNatsServer(opts...)
	if err != nil {
		return nil, err
	}

	return s, nil
}
