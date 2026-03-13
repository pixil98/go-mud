package game

import (
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// newTestObj creates a minimal ObjectInstance for testing. An optional lifetime
// (in ticks) can be provided; zero or omitted means permanent.
func newTestObj(id string, lifetime ...int) *ObjectInstance {
	lt := 0
	if len(lifetime) > 0 {
		lt = lifetime[0]
	}
	obj := storage.NewResolvedSmartIdentifier(id, &assets.Object{
		Aliases:   []string{id},
		ShortDesc: id,
		Lifetime:  lt,
	})
	oi, _ := NewObjectInstance(obj)
	return oi
}
