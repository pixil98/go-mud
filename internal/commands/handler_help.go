package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pixil98/go-mud/internal/storage"
)

// HelpHandlerFactory creates handlers that display command help.
type HelpHandlerFactory struct {
	commands storage.Storer[*Command]
	pub      Publisher
}

// NewHelpHandlerFactory creates a new HelpHandlerFactory.
func NewHelpHandlerFactory(commands storage.Storer[*Command], pub Publisher) *HelpHandlerFactory {
	return &HelpHandlerFactory{commands: commands, pub: pub}
}

func (f *HelpHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "command", Required: false},
		},
	}
}

func (f *HelpHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *HelpHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		command := cmdCtx.Config["command"]
		if command != "" {
			return f.showCommand(command, cmdCtx.Session.CharId)
		}

		return f.listCommands(cmdCtx.Session.CharId)
	}, nil
}

// listCommands displays all commands grouped by category.
func (f *HelpHandlerFactory) listCommands(charId storage.Identifier) error {
	all := f.commands.GetAll()

	// Group commands by category
	groups := make(map[string][]string)
	for id, cmd := range all {
		category := cmd.Category
		if category == "" {
			category = "other"
		}
		groups[category] = append(groups[category], string(id))
	}

	// Sort categories and commands within each category
	categories := make([]string, 0, len(groups))
	for cat := range groups {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	lines := []string{"Available commands:"}
	for _, cat := range categories {
		cmds := groups[cat]
		sort.Strings(cmds)
		label := strings.ToUpper(cat[:1]) + cat[1:]
		lines = append(lines, fmt.Sprintf("  %s: %s", label, strings.Join(cmds, ", ")))
	}

	if f.pub != nil {
		return f.pub.PublishToPlayer(charId, []byte(strings.Join(lines, "\n")))
	}
	return nil
}

// showCommand displays detailed help for a specific command.
func (f *HelpHandlerFactory) showCommand(name string, charId storage.Identifier) error {
	cmd := f.commands.Get(strings.ToLower(name))
	if cmd == nil {
		return NewUserError(fmt.Sprintf("Command %q is unknown.", name))
	}

	cmdName := strings.ToLower(name)

	lines := []string{
		"NAME",
		fmt.Sprintf("    %s - %s", cmdName, cmd.Description),
	}

	if len(cmd.Aliases) > 0 {
		lower := make([]string, len(cmd.Aliases))
		for i, a := range cmd.Aliases {
			lower[i] = strings.ToLower(a)
		}
		lines = append(lines, "", "ALIASES", fmt.Sprintf("    %s", strings.Join(lower, ", ")))
	}

	if len(cmd.Inputs) > 0 {
		parts := []string{cmdName}
		for _, input := range cmd.Inputs {
			if input.Required {
				parts = append(parts, fmt.Sprintf("<%s>", input.Name))
			} else {
				parts = append(parts, fmt.Sprintf("[%s]", input.Name))
			}
		}
		lines = append(lines, "", "SYNOPSIS", fmt.Sprintf("    %s", strings.Join(parts, " ")))
	}

	if f.pub != nil {
		return f.pub.PublishToPlayer(charId, []byte(strings.Join(lines, "\n")))
	}
	return nil
}
