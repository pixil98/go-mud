package storage

import (
	"fmt"
	"regexp"

	"github.com/pixil98/go-errors/errors"
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

	if a.Identifier == "" {
		el.Add(fmt.Errorf("id must be set"))
	}

	if !identifierPattern.MatchString(a.Identifier.String()) {
		el.Add(fmt.Errorf("id must be alphanumeric"))
	}

	el.Add(a.Spec.Validate())

	return el.Err()
}
