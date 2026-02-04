package command

import (
	"fmt"
	"os"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

type StorageConfig struct {
	/* Core Parts */
	Characters AssetConfig[*game.Character]  `json:"characters"`
	Commands   AssetConfig[*commands.Command] `json:"commands"`
	Zones      AssetConfig[*game.Zone]        `json:"zones"`
}

func (c *StorageConfig) Validate() error {
	el := errors.NewErrorList()
	el.Add(c.Characters.Validate())
	el.Add(c.Commands.Validate())
	el.Add(c.Zones.Validate())
	return el.Err()
}

type AssetConfig[T storage.ValidatingSpec] struct {
	Path string `json:"path"`
}

func (c *AssetConfig[T]) Validate() error {
	_, err := os.Stat(c.Path)
	if err != nil {
		return fmt.Errorf("invalid asset path: %w", err)
	}

	return nil
}

func (c *AssetConfig[T]) NewFileStore() (*storage.FileStore[T], error) {
	return storage.NewFileStore[T](c.Path)
}
