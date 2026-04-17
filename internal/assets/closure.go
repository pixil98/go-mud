package assets

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/storage"
)

// ClosureAction values drive the closure command handler's dispatch.
// Commands declare one of these as the `action` config field.
const (
	ClosureActionOpen   = "open"
	ClosureActionClose  = "close"
	ClosureActionLock   = "lock"
	ClosureActionUnlock = "unlock"
)

// Closure defines an openable/closeable barrier on an exit or container.
// A Closure without a Lock is closeable but not lockable.
// Name is required for exit closures but optional for container closures,
// where it falls back to the object's ShortDesc.
type Closure struct {
	// Name is the display label for the barrier (e.g., "door", "gate", "lid").
	Name string `json:"name,omitempty"`

	// Closed is whether the barrier starts closed. Default: open (false).
	Closed bool `json:"closed,omitempty"`

	// Lock optionally makes this closure lockable with a key.
	Lock *Lock `json:"lock,omitempty"`
}

// Validate checks that any lock is valid and consistent with the closure state.
func (c *Closure) Validate() error {
	var errs []error
	if c.Name != "" && strings.Contains(c.Name, " ") {
		errs = append(errs, fmt.Errorf("closure name %q must be a single word", c.Name))
	}
	if c.Lock != nil {
		errs = append(errs, c.Lock.Validate())
		if c.Lock.Locked && !c.Closed {
			errs = append(errs, fmt.Errorf("locked closure must also be closed"))
		}
	}
	return errors.Join(errs...)
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

	// Pickproof means the lock can only be opened with the key, not picked.
	Pickproof bool `json:"pickproof,omitempty"`
}

// Validate checks that the key reference is set.
func (l *Lock) Validate() error {
	return l.KeyId.Validate()
}

// Resolve resolves the key's object reference.
func (l *Lock) Resolve(objs storage.Storer[*Object]) error {
	return l.KeyId.Resolve(objs)
}
