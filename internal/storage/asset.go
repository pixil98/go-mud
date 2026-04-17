package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9-]*$`)

// ValidatingSpec is implemented by any asset spec that can validate its own fields.
type ValidatingSpec interface {
	Validate() error
}

// Asset is the on-disk envelope wrapping a versioned, identified spec value.
type Asset[T ValidatingSpec] struct {
	Version    uint   `json:"version"`
	Identifier string `json:"id"`
	Spec       T      `json:"spec"`
}

func (a *Asset[T]) Id() string {
	return a.Identifier
}

func (a *Asset[T]) Validate() error {
	var errs []error

	if a.Version == 0 {
		errs = append(errs, fmt.Errorf("version must be set"))
	}

	if a.Identifier == "" {
		errs = append(errs, fmt.Errorf("id must be set"))
	}

	if !identifierPattern.MatchString(a.Identifier) {
		errs = append(errs, fmt.Errorf("id must be alphanumeric"))
	}

	errs = append(errs, a.Spec.Validate())

	return errors.Join(errs...)
}

// SmartIdentifier holds an asset key and its resolved value, supporting lazy resolution from a Storer.
type SmartIdentifier[T ValidatingSpec] struct {
	key string
	val T
}

// NewSmartIdentifier creates an unresolved SmartIdentifier with the given key.
func NewSmartIdentifier[T ValidatingSpec](key string) SmartIdentifier[T] {
	return SmartIdentifier[T]{key: key}
}

// NewResolvedSmartIdentifier creates a SmartIdentifier that is already resolved to the given value.
func NewResolvedSmartIdentifier[T ValidatingSpec](key string, val T) SmartIdentifier[T] {
	return SmartIdentifier[T]{key: key, val: val}
}

func (id *SmartIdentifier[T]) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &id.key)
}

func (id SmartIdentifier[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.key)
}

func (id SmartIdentifier[T]) Validate() error {
	if id.key == "" {
		var zero T
		return fmt.Errorf("%s identifier is required", reflect.TypeOf(zero).Elem().Name())
	}
	return nil
}

func (id *SmartIdentifier[T]) Resolve(st Storer[T]) error {
	id.val = st.Get(id.key)
	if reflect.ValueOf(id.val).IsNil() {
		var zero T
		return fmt.Errorf("%s %q not found", reflect.TypeOf(zero).Elem().Name(), id.key)
	}
	return nil
}

func (id SmartIdentifier[T]) Id() string {
	return id.key
}

func (id SmartIdentifier[T]) Get() T {
	return id.val
}
