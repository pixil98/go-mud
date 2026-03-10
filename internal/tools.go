package internal

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// PromptValidator validates prompt input, returning (ok, errorMessage).
type PromptValidator func(string) (bool, string)

type promptConfig struct {
	tries     int
	validator PromptValidator
}

// PromptOption configures the behaviour of a Prompt call.
type PromptOption func(*promptConfig)

// WithValidator sets a custom input validation function on a Prompt.
func WithValidator(v PromptValidator) PromptOption {
	return func(cfg *promptConfig) {
		cfg.validator = v
	}
}

// WithMaxTries limits the number of input retries before Prompt returns an error.
func WithMaxTries(i int) PromptOption {
	return func(cfg *promptConfig) {
		cfg.tries = i
	}
}

// Prompt writes a prompt to rw and reads one line of input, retrying on validation failure.
func Prompt(rw io.ReadWriter, prompt string, opts ...PromptOption) (string, error) {
	config := &promptConfig{}
	for _, opt := range opts {
		opt(config)
	}

	scanner := bufio.NewScanner(rw)

	tries := 0
	for {
		_, err := rw.Write([]byte(prompt))
		if err != nil {
			return "", err
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", err
			}
			return "", io.EOF
		}
		input := strings.TrimRight(scanner.Text(), "\r")

		if config.validator != nil {
			ok, msg := config.validator(input)
			if !ok {
				_, err = rw.Write([]byte(msg))
				if err != nil {
					return "", err
				}

				tries++
				if config.tries > 0 && config.tries == tries {
					_, err := rw.Write([]byte("too many tries"))
					if err != nil {
						return "", err
					}
					return "", fmt.Errorf("too many tries") //TODO: should this error?
				}

				continue
			}
		}

		return input, nil
	}
}

// PromptYN prompts the user for a yes/no answer and returns true for yes.
func PromptYN(rw io.ReadWriter, prompt string) (bool, error) {
	str, err := Prompt(rw, prompt, WithValidator(
		func(str string) (bool, string) {
			switch strings.ToLower(str) {
			case "y", "yes":
				return true, ""

			case "n", "no":
				return true, ""
			default:
				return false, "enter 'yes' or 'no'\n"
			}
		},
	))
	if err != nil {
		return false, err
	}

	switch strings.ToLower(str) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}

}
