package commands

import (
	"bytes"
	"fmt"
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
