package commands

import "fmt"

type Command struct {
	Handler string   `json:"handler"`
	Params  []string `json:"params"`
}

func (c *Command) Validate() error {
	if c.Handler == "" {
		return fmt.Errorf("command handler not set")
	}

	return nil
}
