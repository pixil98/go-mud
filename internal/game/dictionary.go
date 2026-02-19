package game

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/storage"
)

// Dictionary holds all game definition stores. It provides a single
// reference that can be passed to resolution methods so they all
// share the same signature.
type Dictionary struct {
	Characters storage.Storer[*Character]
	Zones      storage.Storer[*Zone]
	Rooms      storage.Storer[*Room]
	Mobiles    storage.Storer[*Mobile]
	Objects    storage.Storer[*Object]
	Pronouns   storage.Storer[*Pronoun]
	Races      storage.Storer[*Race]
}

// Resolve resolves all foreign key references on non-character asset types.
// Characters are resolved at login time instead.
func (d *Dictionary) Resolve() error {
	for id, mob := range d.Mobiles.GetAll() {
		if err := mob.Resolve(d); err != nil {
			return fmt.Errorf("mobile %s: %w", id, err)
		}
	}

	for id, room := range d.Rooms.GetAll() {
		if err := room.Resolve(d); err != nil {
			return fmt.Errorf("mobile %s: %w", id, err)
		}
	}
	return nil
}
