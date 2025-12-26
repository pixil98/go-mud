package player

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/storage"
)

const (
	defaultSelectorRowLength = 80
	defaultSelectorRowCount  = 5
)

type Selectable interface {
	Selector() string
}

type selector[T Selectable] struct {
	options []option[T]
	output  []string
}

type option[T Selectable] struct {
	id  storage.Identifier
	val T
}

func NewSelector[T Selectable](v map[storage.Identifier]T) *selector[T] {
	s := &selector[T]{
		options: []option[T]{},
	}

	for id, val := range v {
		s.options = append(s.options, option[T]{id: id, val: val})
	}
	s.build()

	return s
}

func (s *selector[T]) Prompt(rw io.ReadWriter, prompt string) (storage.Identifier, error) {

	rw.Write([]byte(fmt.Sprintf("%s\n", prompt)))

	for _, str := range s.output {
		if len(str) > 0 {
			rw.Write([]byte(fmt.Sprintf("%s\n", str)))
		}
	}

	selection, err := Prompt(rw, "Make your selection: ", WithValidator(
		func(str string) (bool, string) {
			i, err := strconv.Atoi(str)
			if err != nil {
				return false, "Invalid selection!\n"
			}

			if s.Select(i) == "" {
				return false, "Invalid selection!\n"
			}

			return true, ""
		},
	))
	if err != nil {
		return "", err
	}

	i, err := strconv.Atoi(selection)
	if err != nil {
		return "", err
	}

	return s.Select(i), nil
}

func (s *selector[T]) Select(i int) storage.Identifier {
	if i < 1 || i > len(s.options) {
		return ""
	}
	return s.options[i-1].id
}

func (s *selector[T]) build() {
	// Calculate column width
	colWidth := 1
	for _, v := range s.options {
		l := len(v.val.Selector()) + 7 // Plus 7 for number and spacing (nn. <val>  )
		if l > colWidth {
			colWidth = l
		}
	}

	// Figure out the number number of columns and rows. We want to fill columns
	// first, left to right, but we might need more rows than the default number
	// if there isn't enough space.
	numVals := len(s.options)
	numCols := defaultSelectorRowLength / colWidth
	numRows := numVals / numCols
	if numRows < defaultSelectorRowCount {
		numRows = defaultSelectorRowCount
	}

	count := 0
	rows := make([]string, numRows)
	for _, v := range s.options {
		rows[count%numRows] = rows[count%numRows] + fmt.Sprintf("%2d. %-*s  ", count+1, colWidth-5, v.val.Selector())
		count++
	}

	s.output = rows
}

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

	br := bufio.NewReader(rw)

	tries := 0
	var input []byte
	for {
		_, err := rw.Write([]byte(prompt))
		if err != nil {
			return "", err
		}

		//TODO: I'm pretty sure this shouldn't be using ReadLine
		input, _, err = br.ReadLine()
		if err != nil {
			return "", err
		}

		if config.validator != nil {
			ok, msg := config.validator(string(input))
			if !ok {
				rw.Write([]byte(msg))

				tries++
				if config.tries > 0 && config.tries == tries {
					rw.Write([]byte("too many tries"))
					return "", fmt.Errorf("too many tries") //TODO: should this error?
				}

				continue
			}
		}

		return string(input), nil
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
