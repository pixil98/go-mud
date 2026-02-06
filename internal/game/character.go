package game

// Character represents a player character in the game.
type Character struct {
	Name     string `json:"name"`
	Password string `json:"password"` //TODO make this okay to save
	Title    string `json:"title,omitempty"`

	Entity
}

// Validate satisfies storage.ValidatingSpec
// TODO: We should validate some things here
func (c *Character) Validate() error {
	return nil
}
