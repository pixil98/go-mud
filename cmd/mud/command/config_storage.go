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
	Characters AssetConfig[*game.Character]   `json:"characters"`
	Commands   AssetConfig[*commands.Command] `json:"commands"`
	Zones      AssetConfig[*game.Zone]        `json:"zones"`
	Rooms      AssetConfig[*game.Room]        `json:"rooms"`
	Mobiles    AssetConfig[*game.Mobile]      `json:"mobiles"`
	Objects    AssetConfig[*game.Object]      `json:"objects"`
	Pronouns   AssetConfig[*game.Pronoun]     `json:"pronouns"`
	Races      AssetConfig[*game.Race]        `json:"races"`
}

func (c *StorageConfig) validate() error {
	el := errors.NewErrorList()
	el.Add(c.Characters.Validate("characters"))
	el.Add(c.Commands.Validate("commands"))
	el.Add(c.Zones.Validate("zones"))
	el.Add(c.Rooms.Validate("rooms"))
	el.Add(c.Mobiles.Validate("mobiles"))
	el.Add(c.Objects.Validate("objects"))
	el.Add(c.Pronouns.Validate("pronouns"))
	el.Add(c.Races.Validate("races"))
	return el.Err()
}

type AssetConfig[T storage.ValidatingSpec] struct {
	Path string `json:"path"`
}

func (c *AssetConfig[T]) Validate(name string) error {
	if c.Path == "" {
		return fmt.Errorf("%s: path is required", name)
	}
	_, err := os.Stat(c.Path)
	if err != nil {
		return fmt.Errorf("%s: invalid path %q: %w", name, c.Path, err)
	}

	return nil
}

func (c *AssetConfig[T]) BuildFileStore() (*storage.FileStore[T], error) {
	return storage.NewFileStore[T](c.Path)
}
