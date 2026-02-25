package storage

import (
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/pixil98/go-mud/internal"
)

const (
	defaultSelectorRowLength = 80
	defaultSelectorRowCount  = 5
)

type validatingSelectable interface {
	ValidatingSpec
	Selector() string
}

type SelectableStorer[T validatingSelectable] struct {
	Storer[T]

	options []option[T]
	output  []string
}

type option[T validatingSelectable] struct {
	id  string
	val T
}

func NewSelectableStorer[T validatingSelectable](st Storer[T]) *SelectableStorer[T] {
	s := &SelectableStorer[T]{Storer: st}

	for id, val := range s.GetAll() {
		s.options = append(s.options, option[T]{id: id, val: val})
	}
	slices.SortFunc(s.options, func(a, b option[T]) int {
		sa, sb := a.val.Selector(), b.val.Selector()
		if sa < sb {
			return -1
		}
		if sa > sb {
			return 1
		}
		return 0
	})
	s.build()

	return s
}

func (s *SelectableStorer[T]) build() {
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

func (s *SelectableStorer[T]) Prompt(rw io.ReadWriter, prompt string) (string, error) {

	_, err := fmt.Fprintf(rw, "%s\n", prompt)
	if err != nil {
		return "", err
	}

	for _, str := range s.output {
		if len(str) > 0 {
			_, err = fmt.Fprintf(rw, "%s\n", str)
			if err != nil {
				return "", err
			}
		}
	}

	selection, err := internal.Prompt(rw, "Make your selection: ", internal.WithValidator(
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

func (s *SelectableStorer[T]) Select(i int) string {
	if i < 1 || i > len(s.options) {
		return ""
	}
	return s.options[i-1].id
}
