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

// ExpandTemplate expands a template string using the provided data.
// The data can be any struct - templates access fields via {{ .FieldName }}.
func ExpandTemplate(tmplStr string, data any) (string, error) {
	tmpl, err := template.New("").Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

// expandInputTemplate expands a template string using InputContext (Pass 1).
// This substitutes input values into config strings before handler execution.
func expandInputTemplate(tmplStr string, ctx *InputContext) (string, error) {
	// Quick check: if no template markers, return as-is
	if !strings.Contains(tmplStr, "{{") {
		return tmplStr, nil
	}

	tmpl, err := template.New("").Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, ctx)
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}
