package player

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/storage"
)

const (
	newCharStepBegin int = iota
	newCharStepConfirm
	newCharStepPasswordOne
	newCharStepPasswordTwo
	newCharStepPronouns
	newCharStepRace
)

type characterCreationFlow struct {
	step     int
	charname string
	password string

	pSelector *selector[*Pronoun]
	pronoun   *Pronoun

	rSelector *selector[*Race]
	race      *Race

	cStore storage.Storer[*Character]
}

func NewCharacterCreationFlow(s storage.Storer[*Character], p storage.Storer[*Pronoun], r storage.Storer[*Race]) *characterCreationFlow {
	return &characterCreationFlow{
		cStore:    s,
		step:      newCharStepBegin,
		pSelector: NewSelector[*Pronoun](p.GetAll()),
		rSelector: NewSelector[*Race](r.GetAll()),
	}
}

func (f *characterCreationFlow) Name() string {
	return "character-creation"
}

func (f *characterCreationFlow) Run(str string, state *State, w io.Writer) (bool, error) {
	switch f.step {
	case newCharStepBegin:
		f.charname = str
		w.Write([]byte(fmt.Sprintf("Did I get that right, %s (Y/N)? ", f.charname)))
		f.step = newCharStepConfirm
		return false, nil
	case newCharStepConfirm:
		switch strings.ToLower(str) {
		case "y", "yes":
			w.Write([]byte(fmt.Sprintf("Give me a password for %s: ", f.charname)))
			f.step = newCharStepPasswordOne
			return false, nil

		case "n", "no":
			return true, nil
		default:
			w.Write([]byte("Please type Yes or No: "))
			return false, nil
		}
	case newCharStepPasswordOne:
		if len(str) == 0 || strings.EqualFold(str, f.charname) {
			w.Write([]byte("Illegal Password.\nPassword: "))
			return false, nil
		}

		f.password = str
		w.Write([]byte("Please retype password: "))
		f.step = newCharStepPasswordTwo
		return false, nil

	case newCharStepPasswordTwo:
		if f.password != str {
			w.Write([]byte("Passwords don't match... start over.\nPassword: "))
			f.step = newCharStepPasswordOne
			return false, nil
		}

		w.Write([]byte("What are your pronouns?\n"))
		f.pSelector.Prompt(w)
		f.step = newCharStepPronouns
		return false, nil

	case newCharStepPronouns:

		i, err := strconv.Atoi(str)
		if err == nil {
			f.pronoun = f.pSelector.Select(i)
		}

		if f.pronoun == nil {
			w.Write([]byte("Invalid selection!\n"))
			return false, nil
		}

		w.Write([]byte("What is your race?\n"))
		f.rSelector.Prompt(w)
		f.step = newCharStepRace
		return false, nil

	case newCharStepRace:
		i, err := strconv.Atoi(str)
		if err == nil {
			f.race = f.rSelector.Select(i)
		}

		if f.race == nil {
			w.Write([]byte("Invalid selection!\n"))
			return false, nil
		}

		state.char = f.newCharacter()
		f.cStore.Save(strings.ToLower(state.char.Name), state.char)
	}

	return true, nil
}

func (f *characterCreationFlow) newCharacter() *Character {
	return &Character{
		Name:     f.charname,
		Password: f.password,
		Pronoun:  f.pronoun.Selector(),
		Race:     f.race.Selector(),
	}
}
