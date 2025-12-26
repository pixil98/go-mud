package storage

import (
	"fmt"
	"regexp"

	"github.com/pixil98/go-errors/errors"
)

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

	is_alphanumeric := regexp.MustCompile(`^[a-zA-Z0-9-]*$`).MatchString(a.Identifier.String())
	if !is_alphanumeric {
		el.Add(fmt.Errorf("id must be alphanumeric"))
	}

	el.Add(a.Spec.Validate())

	return el.Err()
}
