package game

// Character represents a player character in the game.
type Character struct {
	Entity
	Password string `json:"password"` //TODO make this okay to save
}

// Validate satisfies storage.ValidatingSpec
func (c *Character) Validate() error {
	return nil
}
