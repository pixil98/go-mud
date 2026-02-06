package player

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/pixil98/go-mud/internal"
	"github.com/pixil98/go-mud/internal/game"
)

const maxPasswordTries = 3

type loginFlow struct {
	world *game.WorldState
}

func (f *loginFlow) Run(rw io.ReadWriter) (*game.Character, error) {
	_, err := rw.Write([]byte("Welcome to GoMud!\n"))
	if err != nil {
		return nil, err
	}

	for {
		// Get username
		username, err := internal.Prompt(rw, "By what name do you wish to be known? ",
			internal.WithValidator(func(str string) (bool, string) {
				// Ensure non-null name
				if len(str) == 0 {
					return false, "Invalid name, please try another.\n"
				}

				// Ensure name is only letters
				for _, r := range str {
					if !unicode.IsLetter(r) {
						return false, "Invalid name, please try another.\n"
					}
				}

				return true, ""
			},
			))
		if err != nil {
			return nil, err
		}

		// Look up the character
		char := f.world.Characters().Get(strings.ToLower(username))

		// Must be a new character
		if char == nil {
			char, err = f.newCharacter(rw, username)
			if err != nil {
				return nil, err
			}
			if char == nil {
				continue
			}

			// Existing user
		} else {
			_, err = internal.Prompt(rw, "Password: ", internal.WithMaxTries(maxPasswordTries), internal.WithValidator(
				func(str string) (bool, string) {
					if char.Password != str {
						return false, ""
					}

					return true, ""
				},
			))
			if err != nil {
				return nil, err
			}
		}

		return char, nil
	}
}

func (f *loginFlow) newCharacter(rw io.ReadWriter, username string) (*game.Character, error) {
	ok, err := internal.PromptYN(rw, fmt.Sprintf("Did I get that right, %s (Y/N)? ", username))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	for {
		passOne, err := internal.Prompt(rw, fmt.Sprintf("Give me a password for %s: ", username), internal.WithValidator(
			func(str string) (bool, string) {
				if len(str) == 0 || strings.EqualFold(str, username) {
					return false, "Illegal Password.\n"
				}

				return true, ""
			},
		))
		if err != nil {
			return nil, err
		}

		passTwo, err := internal.Prompt(rw, "Please retype password: ")
		if err != nil {
			return nil, err
		}

		if passOne != passTwo {
			_, err = rw.Write([]byte("Passwords don't match... start over.\n"))
			if err != nil {
				return nil, err
			}
			continue
		}

		return &game.Character{
			Name:     username,
			Password: passOne,
			Entity: game.Entity{
				DetailedDesc: "A plain unremarkable entity.",
			},
		}, nil
	}
}
