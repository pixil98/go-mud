package game

import (
	"fmt"

	"github.com/pixil98/go-errors"
	"github.com/pixil98/go-mud/internal/storage"
)

// Closure defines an openable/closeable barrier on an exit or container.
// A Closure without a Lock is closeable but not lockable.
// Name is required for exit closures but optional for container closures,
// where it falls back to the object's ShortDesc.
type Closure struct {
	// Name is the display label for the barrier (e.g., "door", "gate", "lid").
	// Required on exits; optional on containers.
	Name string `json:"name,omitempty"`

	// Closed is whether the barrier starts closed. Default: open (false).
	Closed bool `json:"closed,omitempty"`

	// Lock optionally makes this closure lockable with a key.
	Lock *Lock `json:"lock,omitempty"`
}

// Validate checks that any lock is valid and consistent with the closure state.
func (c *Closure) Validate() error {
	el := errors.NewErrorList()
	if c.Lock != nil {
		el.Add(c.Lock.Validate())
		if c.Lock.Locked && !c.Closed {
			el.Add(fmt.Errorf("locked closure must also be closed"))
		}
	}
	return el.Err()
}

// Resolve resolves the lock's key reference if present.
func (c *Closure) Resolve(objs storage.Storer[*Object]) error {
	if c.Lock != nil {
		return c.Lock.Resolve(objs)
	}
	return nil
}

// Lock defines a key-based lock on a Closure.
type Lock struct {
	// KeyId references the object definition that serves as the key.
	KeyId storage.SmartIdentifier[*Object] `json:"key_id"`

	// Locked is whether the lock starts locked. Default: unlocked (false).
	Locked bool `json:"locked,omitempty"`
}

// Validate checks that the key reference is set.
func (l *Lock) Validate() error {
	return l.KeyId.Validate()
}

// Resolve resolves the key's object reference.
func (l *Lock) Resolve(objs storage.Storer[*Object]) error {
	return l.KeyId.Resolve(objs)
}
