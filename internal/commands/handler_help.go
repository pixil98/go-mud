package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// Helpable is implemented by anything that can provide help text.
type Helpable interface {
	Help(name string) string
	HelpSummary() (category, description string)
}

// HelpHandlerFactory creates handlers that display command help.
type HelpHandlerFactory struct {
	entries map[string]Helpable // primary name → helpable
	aliases map[string]string   // alias → primary name
}

// NewHelpHandlerFactory creates a new HelpHandlerFactory by indexing all
// commands and abilities.
func NewHelpHandlerFactory(cmds storage.Storer[*assets.Command], abilities storage.Storer[*assets.Ability]) *HelpHandlerFactory {
	entries := make(map[string]Helpable)
	aliases := make(map[string]string)

	for id, cmd := range cmds.GetAll() {
		entries[id] = cmd
		for _, alias := range cmd.Aliases {
			aliases[strings.ToLower(alias)] = id
		}
	}
	for id, ability := range abilities.GetAll() {
		entries[id] = ability
		for _, alias := range ability.Command.Aliases {
			aliases[strings.ToLower(alias)] = id
		}
	}

	return &HelpHandlerFactory{entries: entries, aliases: aliases}
}

// Spec returns the handler's target and config requirements.
func (f *HelpHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "command", Required: false},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *HelpHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *HelpHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, in *CommandInput) error {
		command := in.Config["command"]
		if command != "" {
			return f.showCommand(command, in.Actor)
		}

		return f.listCommands(in.Actor)
	}, nil
}

// listCommands displays all commands grouped by category.
func (f *HelpHandlerFactory) listCommands(actor game.Actor) error {
	groups := make(map[string][]string)
	for name, h := range f.entries {
		category, _ := h.HelpSummary()
		if category == "" {
			category = "other"
		}
		groups[category] = append(groups[category], name)
	}

	categories := make([]string, 0, len(groups))
	for cat := range groups {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	lines := []string{"Available commands:"}
	for _, cat := range categories {
		cmds := groups[cat]
		sort.Strings(cmds)
		label := display.Capitalize(cat)
		lines = append(lines, fmt.Sprintf("  %s: %s", label, strings.Join(cmds, ", ")))
	}

	actor.Notify(strings.Join(lines, "\n"))
	return nil
}

// showCommand displays detailed help for a specific command.
func (f *HelpHandlerFactory) showCommand(name string, actor game.Actor) error {
	lower := strings.ToLower(name)

	h, ok := f.entries[lower]
	if !ok {
		if primary, found := f.aliases[lower]; found {
			h = f.entries[primary]
			lower = primary
		}
	}
	if h == nil {
		return NewUserError(fmt.Sprintf("Command %q is unknown.", name))
	}

	actor.Notify(h.Help(lower))
	return nil
}
