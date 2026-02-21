package display

import (
	"github.com/muesli/reflow/wordwrap"
)

const DefaultWidth = 80

// Wrap word-wraps text to DefaultWidth, preserving ANSI escape sequences.
func Wrap(text string) string {
	return wordwrap.String(text, DefaultWidth)
}
