package display

import (
	"strings"

	"github.com/muesli/reflow/wordwrap"
)

// DefaultWidth is the default terminal column width used for word-wrapping output.
const DefaultWidth = 80

// Wrap word-wraps text to DefaultWidth, preserving ANSI escape sequences.
func Wrap(text string) string {
	return wordwrap.String(text, DefaultWidth)
}

// Capitalize returns s with its first character uppercased.
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
