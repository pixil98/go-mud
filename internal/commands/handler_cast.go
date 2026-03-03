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
	return func(ctx context.Context, in *CommandInput) error {
		spellName := in.Config["spell"]
		argsStr := in.Config["args"]

		id, ability := f.findSpell(spellName)
		if ability == nil {
			return NewUserError("You don't know a spell called '" + spellName + "'.")
		}

		// Check if the actor has unlocked this spell
		if !in.Char.HasAbility(id) {
			return NewUserError("You don't know a spell called '" + spellName + "'.")
		}

		// Parse remaining args against spell's inputs
		var rawArgs []string
		if argsStr != "" {
			rawArgs = strings.Fields(argsStr)
		}
		inputMap, err := parseInputs(ability.Command.Inputs, rawArgs)
		if err != nil {
			return err
		}

		// Resolve targets
		resolver := NewTargetResolver(NewWorldScopes(f.world))
		targets, err := resolver.ResolveSpecs(ability.Command.Targets, inputMap, in.Char)
		if err != nil {
			return err
		}

		return executeAbility(ability, in, targets, f.world, f.pub, f.effects[ability.Handler])
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
