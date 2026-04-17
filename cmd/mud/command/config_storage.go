package command

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
	"golang.org/x/sync/errgroup"
)

// StorageConfig holds file paths for all game asset stores.
type StorageConfig struct {
	/* Core Parts */
	Characters AssetConfig[*assets.Character] `json:"characters"`
	Commands   AssetConfig[*assets.Command]   `json:"commands"`
	Zones      AssetConfig[*assets.Zone]      `json:"zones"`
	Rooms      AssetConfig[*assets.Room]      `json:"rooms"`
	Mobiles    AssetConfig[*assets.Mobile]    `json:"mobiles"`
	Objects    AssetConfig[*assets.Object]    `json:"objects"`
	Pronouns   AssetConfig[*assets.Pronoun]   `json:"pronouns"`
	Races      AssetConfig[*assets.Race]      `json:"races"`
	Trees      AssetConfig[*assets.Tree]      `json:"trees"`
	Abilities  AssetConfig[*assets.Ability]   `json:"abilities"`
}

// BuildDictionary creates and resolves all asset stores into a game.Dictionary.
// Store loads run concurrently since each walks an independent directory.
func (c *StorageConfig) BuildDictionary() (*game.Dictionary, error) {
	var (
		chars     *storage.FileStore[*assets.Character]
		zones     *storage.FileStore[*assets.Zone]
		rooms     *storage.FileStore[*assets.Room]
		mobiles   *storage.FileStore[*assets.Mobile]
		objects   *storage.FileStore[*assets.Object]
		pronouns  *storage.FileStore[*assets.Pronoun]
		races     *storage.FileStore[*assets.Race]
		trees     *storage.FileStore[*assets.Tree]
		abilities *storage.FileStore[*assets.Ability]
	)

	build := func(g *errgroup.Group, name string, run func() error) {
		g.Go(func() error {
			start := time.Now()
			if err := run(); err != nil {
				return fmt.Errorf("creating %s store: %w", name, err)
			}
			slog.Info("loaded asset store", "name", name, "duration", time.Since(start))
			return nil
		})
	}

	var g errgroup.Group
	build(&g, "character", func() (err error) { chars, err = c.Characters.BuildFileStore(); return })
	build(&g, "zone", func() (err error) { zones, err = c.Zones.BuildFileStore(); return })
	build(&g, "room", func() (err error) { rooms, err = c.Rooms.BuildFileStore(); return })
	build(&g, "mobile", func() (err error) { mobiles, err = c.Mobiles.BuildFileStore(); return })
	build(&g, "object", func() (err error) { objects, err = c.Objects.BuildFileStore(); return })
	build(&g, "pronoun", func() (err error) { pronouns, err = c.Pronouns.BuildFileStore(); return })
	build(&g, "race", func() (err error) { races, err = c.Races.BuildFileStore(); return })
	build(&g, "tree", func() (err error) { trees, err = c.Trees.BuildFileStore(); return })
	build(&g, "ability", func() (err error) { abilities, err = c.Abilities.BuildFileStore(); return })

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dict := &game.Dictionary{
		Characters: chars,
		Zones:      zones,
		Rooms:      rooms,
		Mobiles:    mobiles,
		Objects:    objects,
		Pronouns:   pronouns,
		Races:      races,
		Trees:      trees,
		Abilities:  abilities,
	}

	if err := dict.Resolve(); err != nil {
		return nil, fmt.Errorf("resolving references: %w", err)
	}

	return dict, nil
}

func (c *StorageConfig) validate() error {
	var errs []error
	errs = append(errs, c.Characters.Validate("characters"))
	errs = append(errs, c.Commands.Validate("commands"))
	errs = append(errs, c.Zones.Validate("zones"))
	errs = append(errs, c.Rooms.Validate("rooms"))
	errs = append(errs, c.Mobiles.Validate("mobiles"))
	errs = append(errs, c.Objects.Validate("objects"))
	errs = append(errs, c.Pronouns.Validate("pronouns"))
	errs = append(errs, c.Races.Validate("races"))
	errs = append(errs, c.Trees.Validate("trees"))
	errs = append(errs, c.Abilities.Validate("abilities"))
	return errors.Join(errs...)
}

// AssetConfig holds the file path for a single asset type.
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
