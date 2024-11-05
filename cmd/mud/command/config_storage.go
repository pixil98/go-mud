package command

import (
	"context"
	"fmt"
	"os"

	"github.com/pixil98/go-errors/errors"
	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/player"
	"github.com/pixil98/go-mud/internal/storage"
)

type StorageConfig struct {
	/* Player Parts */
	Characters AssetConfig[*player.Character] `json:"characters"`
	Commands   AssetConfig[*commands.Command] `json:"commands"`
	Pronouns   AssetConfig[*player.Pronoun]   `json:"pronouns"`
	Races      AssetConfig[*player.Race]      `json:"races"`
}

func (c *StorageConfig) Validate() error {
	el := errors.NewErrorList()
	el.Add(c.Characters.Validate())
	el.Add(c.Commands.Validate())
	el.Add(c.Pronouns.Validate())
	el.Add(c.Races.Validate())
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

func (c *AssetConfig[T]) NewFileStore(ctx context.Context) (*storage.FileStore[T], error) {
	store := storage.NewFileStore[T](c.Path)

	err := store.Load(ctx)
	if err != nil {
		return nil, err
	}

	return store, nil
}
