package storage

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	"github.com/pixil98/go-errors"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9-]*$`)

type ValidatingSpec interface {
	Validate() error
}

type Identifier string

func (id Identifier) String() string {
	return string(id)
}

type Asset[T ValidatingSpec] struct {
	Version    uint       `json:"version"`
	Identifier Identifier `json:"id"`
	Spec       T          `json:"spec"`
}

func (c *Asset[T]) Id() Identifier {
	return c.Identifier
}

func (a *Asset[T]) Validate() error {
	el := errors.NewErrorList()

	if a.Version == 0 {
		el.Add(fmt.Errorf("version must be set"))
	}

	if a.Identifier == "" {
		el.Add(fmt.Errorf("id must be set"))
	}

	if !identifierPattern.MatchString(a.Identifier.String()) {
		el.Add(fmt.Errorf("id must be alphanumeric"))
	}

	el.Add(a.Spec.Validate())

	return el.Err()
}

type SmartIdentifier[T ValidatingSpec] struct {
	key string
	val T
}

func NewSmartIdentifier[T ValidatingSpec](key string) SmartIdentifier[T] {
	return SmartIdentifier[T]{key: key}
}

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

func (id SmartIdentifier[T]) Get() string {
	return id.key
}

func (id SmartIdentifier[T]) Id() T {
	return id.val
}
