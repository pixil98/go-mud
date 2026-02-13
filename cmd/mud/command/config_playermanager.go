package command

import (
	"fmt"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/player"
	"github.com/pixil98/go-mud/internal/storage"
)

type PlayerManagerConfig struct {
	DefaultZone string `json:"default_zone"`
	DefaultRoom string `json:"default_room"`
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

func (c *PlayerManagerConfig) BuildPlayerManager(
	cmdHandler *commands.Handler,
	world *game.WorldState,
	pronouns *storage.SelectableStorer[*game.Pronoun],
	races *storage.SelectableStorer[*game.Race],
) *player.PlayerManager {
	return player.NewPlayerManager(cmdHandler, world, pronouns, races, c.DefaultZone, c.DefaultRoom)
}
