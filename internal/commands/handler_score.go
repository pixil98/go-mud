package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

const scoreBoxWidth = 40

// ScoreActor provides the character state needed by the score handler.
type ScoreActor interface {
	Id() string
	Publish(data []byte, exclude []string)
	StatSections() []game.StatSection
}

var _ ScoreActor = (*game.CharacterInstance)(nil)

// ScoreHandlerFactory creates handlers that display character/mobile stats.
type ScoreHandlerFactory struct{}

// NewScoreHandlerFactory creates a handler factory for score display commands.
func NewScoreHandlerFactory() *ScoreHandlerFactory {
	return &ScoreHandlerFactory{}
}

// Spec returns the handler's target and config requirements.
func (f *ScoreHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer | targetTypeMobile, Required: false},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *ScoreHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *ScoreHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[ScoreActor](f.handle), nil
}

func (f *ScoreHandlerFactory) handle(ctx context.Context, char ScoreActor, in *CommandInput) error {
	sections, err := f.resolveSections(char, in)
	if err != nil {
		return err
	}

	char.Publish([]byte(renderBox(sections, scoreBoxWidth)), nil)
	return nil
}

// TODO: Remove StatSections from CharacterInstance/MobileInstance and build the
// score display entirely from game.Actor and perks.
func (f *ScoreHandlerFactory) resolveSections(char ScoreActor, in *CommandInput) ([]game.StatSection, error) {
	if target := in.FirstTarget("target"); target != nil {
		return target.Actor.Actor().StatSections(), nil
	}
	return char.StatSections(), nil
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
