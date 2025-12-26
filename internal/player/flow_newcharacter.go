package player

import (
	"fmt"
	"io"
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
	pSelector *selector[*Pronoun]
	rSelector *selector[*Race]
}

func (f *characterCreationFlow) Run(rw io.ReadWriter, char *Character) error {

	for {
		if char.Pronoun != "" {
			break
		}

		sel, err := f.pSelector.Prompt(rw, "What are your pronouns?")
		if err != nil {
			return fmt.Errorf("selecting pronouns: %w", err)
		}

		char.Pronoun = sel
	}

	for {
		if char.Race != "" {
			break
		}

		sel, err := f.rSelector.Prompt(rw, "What is your race?")
		if err != nil {
			return fmt.Errorf("selecting race: %w", err)
		}

		char.Race = sel
	}

	return nil
}
