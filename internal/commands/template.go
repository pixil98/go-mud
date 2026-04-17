package commands

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// templateFuncs provides utility functions for templates.
var templateFuncs = sprig.TxtFuncMap()

// CompiledTemplate is a pre-parsed template ready for repeated execution.
// Strings with no "{{" delimiters are stored as a literal fast path, avoiding
// the parser + executor on the hot dispatch path.
type CompiledTemplate struct {
	literal string
	tmpl    *template.Template
}

// CompileTemplate parses tmplStr once. Empty input returns a nil *CompiledTemplate
// so callers can treat "no template" as a nil sentinel.
func CompileTemplate(tmplStr string) (*CompiledTemplate, error) {
	if tmplStr == "" {
		return nil, nil
	}
	if !strings.Contains(tmplStr, "{{") {
		return &CompiledTemplate{literal: tmplStr}, nil
	}
	t, err := template.New("").Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	return &CompiledTemplate{tmpl: t}, nil
}

// Execute renders the template with data. Safe to call on a nil receiver
// (returns empty string).
func (ct *CompiledTemplate) Execute(data any) (string, error) {
	if ct == nil {
		return "", nil
	}
	if ct.tmpl == nil {
		return ct.literal, nil
	}
	var buf bytes.Buffer
	if err := ct.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buf.String(), nil
}

// ExpandTemplate compiles and executes a template in one step. Retained for
// call sites that see each template only once (e.g. test helpers, ad-hoc
// expansions). Hot paths should use CompileTemplate at load time and Execute
// per call.
func ExpandTemplate(tmplStr string, data any) (string, error) {
	ct, err := CompileTemplate(tmplStr)
	if err != nil {
		return "", err
	}
	return ct.Execute(data)
}
