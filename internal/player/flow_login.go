package player

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/pixil98/go-mud/internal/storage"
)

const (
	loginStepBegin int = iota
	loginStepUsername
	loginStepPassword
	loginStepInNewCharFlow
)

type loginFlow struct {
	cStore        storage.Storer[*Character]
	step          int
	passwordTries int

	ncFlow Flow
}

func NewLoginFlow(c storage.Storer[*Character], ncFlow Flow) *loginFlow {
	return &loginFlow{
		cStore: c,
		step:   loginStepBegin,
		ncFlow: ncFlow,
	}
}

func (f *loginFlow) Name() string {
	if f.step == loginStepInNewCharFlow {
		return fmt.Sprintf("login -> %s", f.ncFlow.Name())
	}

	return "login"
}

func (f *loginFlow) Run(str string, state *State, w io.Writer) (bool, error) {
	switch f.step {
	case loginStepBegin:
		w.Write([]byte("Welcome to GoMud!\n"))
		w.Write([]byte("By what name do you wish to be known? "))
		f.step = loginStepUsername
		return false, nil

	case loginStepUsername:
		// Ensure non-null name
		if len(str) == 0 {
			w.Write([]byte("Invalid name, please try another.\nName: "))
			return false, nil
		}

		// Ensure name is only letters
		for _, r := range str {
			if !unicode.IsLetter(r) {
				w.Write([]byte("Invalid name, please try another.\nName: "))
				return false, nil
			}
		}

		// Look up the character
		state.char = f.cStore.Get(strings.ToLower(str))

		// If character doesn't exist invoke new-character flow
		if state.char == nil {
			f.step = loginStepInNewCharFlow
			return f.Run(str, state, w)
		}

		w.Write([]byte("Password: "))
		f.step = loginStepPassword
		return false, nil

	case loginStepPassword:
		if str != state.char.Password {
			w.Write([]byte("Wrong password.\nPassword: "))
			f.passwordTries++

			if f.passwordTries > 2 {
				w.Write([]byte("\nWrong password... disconnecting.\n"))
				return false, fmt.Errorf("too many incorrect password attempts")
			}

			return false, nil
		}

		return true, nil

	case loginStepInNewCharFlow:
		done, err := f.ncFlow.Run(str, state, w)
		if err != nil {
			return true, err
		}

		if done && state.char == nil {
			f.step = loginStepBegin
			return f.Run(str, state, w)
		}

		return done, err
	}

	return true, nil
}
