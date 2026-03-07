package game

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// Dictionary holds all game definition stores. It provides a single
// reference that can be passed to resolution methods so they all
// share the same signature.
type Dictionary struct {
	Characters storage.Storer[*assets.Character]
	Zones      storage.Storer[*assets.Zone]
	Rooms      storage.Storer[*assets.Room]
	Mobiles    storage.Storer[*assets.Mobile]
	Objects    storage.Storer[*assets.Object]
	Pronouns   storage.Storer[*assets.Pronoun]
	Races      storage.Storer[*assets.Race]
	Trees      storage.Storer[*assets.Tree]
	Abilities  storage.Storer[*assets.Ability]
}

// Resolve resolves all foreign key references on non-character asset types.
// Characters are resolved at login time instead.
func (d *Dictionary) Resolve() error {
	for id, mob := range d.Mobiles.GetAll() {
		if err := mob.Resolve(d.Objects); err != nil {
			return fmt.Errorf("mobile %s: %w", id, err)
		}
	}

	for id, obj := range d.Objects.GetAll() {
		if err := obj.Resolve(d.Objects); err != nil {
			return fmt.Errorf("object %s: %w", id, err)
		}
	}

	for id, room := range d.Rooms.GetAll() {
		if err := room.Resolve(d.Zones, d.Rooms, d.Mobiles, d.Objects); err != nil {
			return fmt.Errorf("room %s: %w", id, err)
		}
	}
	return nil
}
