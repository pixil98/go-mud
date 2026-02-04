package game

import (
	"fmt"

	"github.com/pixil98/go-errors"
)

// Zone represents a region in the game world that contains rooms.
type Zone struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Validate satisfies storage.ValidatingSpec.
func (z *Zone) Validate() error {
	el := errors.NewErrorList()

	if z.Name == "" {
		el.Add(fmt.Errorf("zone name is required"))
	}

	return el.Err()
}
