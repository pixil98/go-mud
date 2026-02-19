package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

const scoreBoxWidth = 40

// ScoreHandlerFactory creates handlers that display character/mobile stats.
type ScoreHandlerFactory struct {
	pub Publisher
}

func NewScoreHandlerFactory(pub Publisher) *ScoreHandlerFactory {
	return &ScoreHandlerFactory{pub: pub}
}

func (f *ScoreHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypePlayer | TargetTypeMobile, Required: false},
		},
	}
}

func (f *ScoreHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *ScoreHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		sections, err := f.resolveSections(cmdCtx)
		if err != nil {
			return err
		}

		output := renderBox(sections, scoreBoxWidth)
		if f.pub != nil {
			return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte(output))
		}
		return nil
	}, nil
}

func (f *ScoreHandlerFactory) resolveSections(cmdCtx *CommandContext) ([]game.StatSection, error) {
	if target := cmdCtx.Targets["target"]; target != nil {
		switch target.Type {
		case TargetTypePlayer:
			return target.Player.session.Character.StatSections(), nil
		case TargetTypeMobile:
			return target.Mob.instance.Mobile.Get().StatSections(), nil
		}
	}

	return cmdCtx.Actor.StatSections(), nil
}

// --- Box rendering ---

func renderBox(sections []game.StatSection, width int) string {
	var lines []string
	lines = append(lines, boxBorder(width))
	for i, section := range sections {
		if i > 0 {
			lines = append(lines, boxBorder(width))
		}
		if section.Header != "" {
			lines = append(lines, boxLine(section.Header, width))
		}
		for _, line := range section.Lines {
			if line.Center {
				lines = append(lines, boxLineCenter(line.Value, width))
			} else {
				lines = append(lines, boxLine(line.Value, width))
			}
		}
	}
	lines = append(lines, boxBorder(width))
	return strings.Join(lines, "\n")
}

func boxBorder(width int) string {
	return "+" + strings.Repeat("-", width-2) + "+"
}

func boxLine(text string, width int) string {
	inner := width - 4
	if len(text) > inner {
		text = text[:inner]
	}
	return fmt.Sprintf("| %-*s |", inner, text)
}

func boxLineCenter(text string, width int) string {
	inner := width - 4
	if len(text) > inner {
		text = text[:inner]
	}
	pad := (inner - len(text)) / 2
	return fmt.Sprintf("| %*s%-*s |", pad+len(text), text, inner-pad-len(text), "")
}
