package internal

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type promptValidator func(string) (bool, string)

type promptConfig struct {
	tries     int
	validator promptValidator
}

type promptOption func(*promptConfig)

func WithValidator(v promptValidator) promptOption {
	return func(cfg *promptConfig) {
		cfg.validator = v
	}
}

func WithMaxTries(i int) promptOption {
	return func(cfg *promptConfig) {
		cfg.tries = i
	}
}

func Prompt(rw io.ReadWriter, prompt string, opts ...promptOption) (string, error) {
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
