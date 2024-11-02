package player

import (
	"fmt"
	"io"
)

const (
	defaultSelectorRowLength = 80
	defaultSelectorRowCount  = 5
)

type Selectable interface {
	Selector() string
}

type selector[T Selectable] struct {
	values []T
	output []string
}

func NewSelector[T Selectable](v []T) *selector[T] {
	s := &selector[T]{
		values: v,
	}
	s.build()

	return s
}

func (s *selector[T]) Prompt(w io.Writer) error {

	for _, str := range s.output {
		if len(str) > 0 {
			w.Write([]byte(fmt.Sprintf("%s\n", str)))
		}
	}

	w.Write([]byte("\nMake your selection: "))
	return nil
}

func (s *selector[T]) Select(i int) T {
	if i < 1 || i > len(s.values) {
		var zero T
		return zero
	}
	return s.values[i-1]
}

func (s *selector[T]) build() {
	// Calculate column width
	colWidth := 1
	for _, v := range s.values {
		l := len(v.Selector()) + 7 // Plus 7 for number and spacing (nn. <val>  )
		if l > colWidth {
			colWidth = l
		}
	}

	// Figure out the number number of columns and rows. We want to fill columns
	// first, left to right, but we might need more rows than the default number
	// if there isn't enough space.
	numVals := len(s.values)
	numCols := defaultSelectorRowLength / colWidth
	numRows := numVals / numCols
	if numRows < defaultSelectorRowCount {
		numRows = defaultSelectorRowCount
	}

	count := 0
	rows := make([]string, numRows)
	for _, v := range s.values {
		rows[count%numRows] = rows[count%numRows] + fmt.Sprintf("%2d. %-*s  ", count+1, colWidth-5, v.Selector())
		count++
	}

	s.output = rows
}
