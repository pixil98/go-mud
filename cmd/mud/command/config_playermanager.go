package command

import (
	"fmt"
	"time"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/player"
)

type PlayerManagerConfig struct {
	DefaultZone     string `json:"default_zone"`
	DefaultRoom     string `json:"default_room"`
	LinklessTimeout string `json:"linkless_timeout,omitempty"`
	IdleTimeout     string `json:"idle_timeout,omitempty"`
}

func (c *PlayerManagerConfig) validate() error {
	el := errors.NewErrorList()

	if c.DefaultZone == "" {
		el.Add(fmt.Errorf("default_zone is required"))
	}
	if c.DefaultRoom == "" {
		el.Add(fmt.Errorf("default_room is required"))
	}

	return el.Err()
}

func (c *PlayerManagerConfig) BuildPlayerManager(cmdHandler *commands.Handler, world *game.WorldState, dict *game.Dictionary) (*player.PlayerManager, error) {
	var opts []player.PlayerManagerOpt

	if c.LinklessTimeout != "" {
		d, err := time.ParseDuration(c.LinklessTimeout)
		if err != nil {
			return nil, fmt.Errorf("parsing linkless_timeout %q: %w", c.LinklessTimeout, err)
		}
		opts = append(opts, player.WithLinklessTimeout(d))
	}

	if c.IdleTimeout != "" {
		d, err := time.ParseDuration(c.IdleTimeout)
		if err != nil {
			return nil, fmt.Errorf("parsing idle_timeout %q: %w", c.IdleTimeout, err)
		}
		opts = append(opts, player.WithIdleTimeout(d))
	}

	return player.NewPlayerManager(cmdHandler, world, dict, c.DefaultZone, c.DefaultRoom, opts...), nil
}
