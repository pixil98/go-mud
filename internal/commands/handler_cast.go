package commands

import (
	"context"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// CastHandlerFactory creates the handler for the "cast" command.
// It looks up a spell by name, verifies the actor has it unlocked,
// parses remaining arguments against the spell's embedded Command,
// resolves targets, and executes the ability.
type CastHandlerFactory struct {
	abilities storage.Storer[*assets.Ability]
	world     WorldView
	pub       game.Publisher
	effects   map[string]EffectHandler
}

func NewCastHandlerFactory(abilities storage.Storer[*assets.Ability], world WorldView, pub game.Publisher, effects map[string]EffectHandler) *CastHandlerFactory {
	return &CastHandlerFactory{abilities: abilities, world: world, pub: pub, effects: effects}
}

func (f *CastHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "spell", Required: true},
			{Name: "args", Required: false},
		},
	}
}

func (f *CastHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *CastHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		spellName := cmdCtx.Config["spell"]
		argsStr := cmdCtx.Config["args"]

		id, ability := f.findSpell(spellName)
		if ability == nil {
			return NewUserError("You don't know a spell called '" + spellName + "'.")
		}

		// Check if the actor has unlocked this spell
		if !cmdCtx.Session.HasAbility(id) {
			return NewUserError("You don't know a spell called '" + spellName + "'.")
		}

		// Parse remaining args against spell's inputs
		var rawArgs []string
		if argsStr != "" {
			rawArgs = strings.Fields(argsStr)
		}
		inputs, err := parseInputs(ability.Command.Inputs, rawArgs)
		if err != nil {
			return err
		}

		// Build input map
		inputMap := make(map[string]any, len(ability.Command.Inputs))
		for _, spec := range ability.Command.Inputs {
			if !spec.Required {
				inputMap[spec.Name] = ""
			}
		}
		for _, input := range inputs {
			inputMap[input.Spec.Name] = input.Value
		}

		// Resolve targets
		resolver := NewTargetResolver(NewWorldScopes(f.world))
		targets, err := resolver.ResolveSpecs(ability.Command.Targets, inputMap, cmdCtx.Session)
		if err != nil {
			return err
		}

		return executeAbility(ability, cmdCtx, targets, f.world, f.pub, f.effects[ability.Handler])
	}, nil
}

// findSpell looks up a spell by ID first, then by name case-insensitively.
// Returns the store ID and ability, or ("", nil) if not found.
func (f *CastHandlerFactory) findSpell(name string) (string, *assets.Ability) {
	lower := strings.ToLower(name)
	if a := f.abilities.Get(lower); a != nil && a.Type == assets.AbilityTypeSpell {
		return lower, a
	}
	for id, a := range f.abilities.GetAll() {
		if a.Type == assets.AbilityTypeSpell && strings.ToLower(a.Name) == lower {
			return id, a
		}
	}
	return "", nil
}
