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
		playerChannel := fmt.Sprintf("player-%s", strings.ToLower(cmdCtx.Actor.Name))

		command := cmdCtx.Config["command"]
		if command != "" {
			return f.showCommand(command, playerChannel)
		}

		return f.listCommands(playerChannel)
	}, nil
}

// listCommands displays all commands grouped by category.
func (f *HelpHandlerFactory) listCommands(channel string) error {
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
		_ = f.pub.Publish(channel, []byte(strings.Join(lines, "\n")))
	}
	return nil
}

// showCommand displays detailed help for a specific command.
func (f *HelpHandlerFactory) showCommand(name string, channel string) error {
	cmd := f.commands.Get(strings.ToLower(name))
	if cmd == nil {
		return NewUserError(fmt.Sprintf("Command %q is unknown.", name))
	}

	lines := []string{fmt.Sprintf("%s: %s", strings.ToLower(name), cmd.Description)}

	// Build usage line from inputs
	if len(cmd.Inputs) > 0 {
		parts := []string{strings.ToLower(name)}
		for _, input := range cmd.Inputs {
			if input.Required {
				parts = append(parts, fmt.Sprintf("<%s>", input.Name))
			} else {
				parts = append(parts, fmt.Sprintf("[%s]", input.Name))
			}
		}
		lines = append(lines, fmt.Sprintf("Usage: %s", strings.Join(parts, " ")))
	}

	if f.pub != nil {
		_ = f.pub.Publish(channel, []byte(strings.Join(lines, "\n")))
	}
	return nil
}
