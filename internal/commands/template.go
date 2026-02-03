package commands

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// templateFuncs provides utility functions for templates.
var templateFuncs = sprig.TxtFuncMap()

// TemplateData is the root data structure passed to templates.
type TemplateData struct {
	Actor *game.Character
	State *game.PlayerState
	Args  map[string]any
}

// NewTemplateData creates a TemplateData from world state and parsed arguments.
func NewTemplateData(world *game.WorldState, charId storage.Identifier, args []ParsedArg) *TemplateData {
	argsMap := make(map[string]any, len(args))
	for _, arg := range args {
		argsMap[arg.Spec.Name] = arg.Value
	}

	return &TemplateData{
		Actor: world.Characters().Get(string(charId)),
		State: world.GetPlayer(charId),
		Args:  argsMap,
	}
}

// ExpandTemplate expands a template string using the provided data.
func ExpandTemplate(tmplStr string, data *TemplateData) (string, error) {
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
