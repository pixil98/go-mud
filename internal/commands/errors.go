package commands

// UserError represents an error that should be displayed to the user.
// These are not system failures - just invalid input or usage.
type UserError struct {
	Message string
}

func (e *UserError) Error() string {
	return e.Message
}

// NewUserError creates a user-facing error.
func NewUserError(msg string) *UserError {
	return &UserError{Message: msg}
}
