package commands

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// CommandActor is the minimal interface for the character executing a command.
// Handlers that need more define their own interface extending CommandActor
// and use Adapt to get compile-time type safety.
type CommandActor interface {
	Id() string
	Name() string
}

// CommandInput is what handlers receive after config processing.
type CommandInput struct {
	Char    CommandActor          // Active character
	Targets map[string]*TargetRef // Resolved targets by name
	Config  map[string]string     // Expanded config values (all templates resolved)
}

// CommandFunc is the signature for compiled command functions.
type CommandFunc func(ctx context.Context, in *CommandInput) error

// HandlerFunc is a typed command handler that receives a handler-specific
// actor interface. Use Adapt to wrap it into a CommandFunc for dispatch.
type HandlerFunc[A CommandActor] func(ctx context.Context, char A, in *CommandInput) error

// Adapt wraps a typed HandlerFunc into an untyped CommandFunc for the dispatch
// map. The type assertion is safe: each handler file includes a compile-time
// assertion (var _ XxxActor = (*game.CharacterInstance)(nil)) guaranteeing the
// concrete type satisfies A.
func Adapt[A CommandActor](fn HandlerFunc[A]) CommandFunc {
	return func(ctx context.Context, in *CommandInput) error {
		return fn(ctx, in.Char.(A), in)
	}
}

// TargetRequirement describes an expected target for a handler.
type TargetRequirement struct {
	Name     string     // Target name (must match JSON "name" field)
	Type     targetType // Expected type: targetTypePlayer, targetTypeObject, etc.
	Required bool       // If true, target must exist in command JSON
}

// ConfigRequirement describes an expected config key for a handler.
type ConfigRequirement struct {
	Name     string // Config key name
	Required bool   // If true, config key must exist in command JSON
}

// HandlerSpec describes the expected targets and config for a handler.
// Used for validation at command load time.
type HandlerSpec struct {
	Targets []TargetRequirement
	Config  []ConfigRequirement
}

// PlayerLookup finds a player by character ID.
type PlayerLookup interface {
	GetPlayer(charId string) *game.CharacterInstance
}

// ZoneLocator finds a zone instance by zone ID.
type ZoneLocator interface {
	GetZone(zoneId string) *game.ZoneInstance
}

// WorldView provides read-only access to the game world for command handlers
// that need more than just zone lookup.
type WorldView interface {
	ZoneLocator
	Instances() map[string]*game.ZoneInstance
	ForEachPlayer(func(string, *game.CharacterInstance))
}

// HandlerFactory creates CommandFuncs from command configurations.
// Implementations should expose their expected config structure.
// Factories that need a Publisher should accept it in their constructor.
type HandlerFactory interface {
	// Spec returns the handler's requirements for validation.
	// Return nil to skip spec-based validation.
	Spec() *HandlerSpec
	// ValidateConfig performs custom validation on the config.
	// Called after spec validation. Use for conditional logic that Spec can't express.
	ValidateConfig(config map[string]any) error
	// Create creates a CommandFunc. Handlers read expanded config from CommandInput.
	Create() (CommandFunc, error)
}

// compiledCommand holds a command that's been validated and compiled.
type compiledCommand struct {
	cmd     *assets.Command
	cmdFunc CommandFunc
}

type Handler struct {
	factories map[string]HandlerFactory
	compiled  map[string]*compiledCommand
	effects   map[string]EffectHandler
	combat    CombatManager
}

func NewHandler(cmds storage.Storer[*assets.Command], dict *game.Dictionary, publisher game.Publisher, world *game.WorldState, combat CombatManager) (*Handler, error) {
	h := &Handler{
		factories: make(map[string]HandlerFactory),
		compiled:  make(map[string]*compiledCommand),
		effects:   make(map[string]EffectHandler),
		combat:    combat,
	}

	// Register effect handlers
	h.effects["attack"] = &attackEffect{combat: combat}
	h.effects["damage"] = &damageEffect{combat: combat}
	h.effects["actor_buff"] = &actorBuffEffect{}
	h.effects["room_buff"] = &roomBuffEffect{world: world}
	h.effects["zone_buff"] = &zoneBuffEffect{world: world}
	h.effects["world_buff"] = &worldBuffEffect{world: world}

	// Register built-in handlers
	h.RegisterFactory("assist", NewAssistHandlerFactory(combat, world, world, publisher))
	h.RegisterFactory("cast", NewCastHandlerFactory(dict.Abilities, world, publisher, h.effects))
	h.RegisterFactory("closure", NewClosureHandlerFactory(world, publisher))
	h.RegisterFactory("equipment", NewEquipmentHandlerFactory(publisher))
	h.RegisterFactory("follow", NewFollowHandlerFactory(world, publisher))
	h.RegisterFactory("gain", NewGainHandlerFactory(publisher))
	h.RegisterFactory("group", NewGroupHandlerFactory(world, publisher))
	h.RegisterFactory("ungroup", NewUngroupHandlerFactory(world, publisher))
	h.RegisterFactory("help", NewHelpHandlerFactory(cmds, publisher))
	h.RegisterFactory("inventory", NewInventoryHandlerFactory(publisher))
	h.RegisterFactory("look", NewLookHandlerFactory(world, publisher))
	h.RegisterFactory("message", NewMessageHandlerFactory(world, publisher))
	h.RegisterFactory("move", NewMoveHandlerFactory(world, publisher))
	h.RegisterFactory("move_obj", NewMoveObjHandlerFactory(world, publisher))
	h.RegisterFactory("quit", NewQuitHandlerFactory())
	h.RegisterFactory("save", NewSaveHandlerFactory(dict.Characters, publisher))
	h.RegisterFactory("score", NewScoreHandlerFactory(publisher))
	h.RegisterFactory("title", NewTitleHandlerFactory(publisher))
	h.RegisterFactory("trees", NewTreesHandlerFactory(dict.Trees, publisher))
	h.RegisterFactory("wear", NewWearHandlerFactory(world, publisher))
	h.RegisterFactory("who", NewWhoHandlerFactory(world, publisher))

	// Compile commands
	for id, cmd := range cmds.GetAll() {
		err := h.compile(id, cmd)
		if err != nil {
			return nil, fmt.Errorf("compiling command %q: %w", id, err)
		}
	}

	// Auto-register skill abilities as top-level commands
	for id, ability := range dict.Abilities.GetAll() {
		if ability.Type == assets.AbilityTypeSkill {
			if err := h.registerSkill(id, ability, world, publisher); err != nil {
				return nil, fmt.Errorf("registering skill %q: %w", id, err)
			}
		}
	}

	return h, nil
}

// registerSkill registers a skill ability as a top-level command.
// The skill's embedded Command provides input/target specs. The compiled
// handler checks the actor's perks for unlock and sends ability messages.
func (h *Handler) registerSkill(id string, ability *assets.Ability, world WorldView, pub game.Publisher) error {
	cmd := ability.Command // value copy
	cmd.Handler = ability.Handler

	effect, ok := h.effects[ability.Handler]
	if !ok {
		return fmt.Errorf("unknown effect handler %q", ability.Handler)
	}
	cmdFunc := func(ctx context.Context, in *CommandInput) error {
		actor, ok := in.Char.(AbilityActor)
		if !ok {
			return fmt.Errorf("character does not support abilities")
		}
		if !actor.HasGrant(assets.PerkGrantUnlockAbility, id) {
			return NewUserError("You don't know how to do that.")
		}
		return executeAbility(ability, actor, in, in.Targets, world, pub, effect)
	}

	cc := &compiledCommand{cmd: &cmd, cmdFunc: cmdFunc}

	if _, exists := h.compiled[id]; exists {
		return fmt.Errorf("%q conflicts with an existing command or alias", id)
	}
	h.compiled[id] = cc

	for _, alias := range cmd.Aliases {
		aliasId := strings.ToLower(alias)
		if _, exists := h.compiled[aliasId]; exists {
			return fmt.Errorf("alias %q conflicts with an existing command or alias", alias)
		}
		h.compiled[aliasId] = cc
	}

	return nil
}

// RegisterFactory registers a handler factory by name.
// The name must match the "handler" field in command JSON definitions.
func (h *Handler) RegisterFactory(name string, factory HandlerFactory) error {
	if name == "" {
		return fmt.Errorf("handler name cannot be empty")
	}
	if factory == nil {
		return fmt.Errorf("handler factory cannot be nil")
	}
	if _, exists := h.factories[name]; exists {
		return fmt.Errorf("handler factory %q already registered", name)
	}
	h.factories[name] = factory
	return nil
}

func (h *Handler) compile(id string, cmd *assets.Command) error {
	factory, ok := h.factories[cmd.Handler]
	if !ok {
		return fmt.Errorf("unknown handler %q", cmd.Handler)
	}

	// Validate against handler spec if provided
	if spec := factory.Spec(); spec != nil {
		if err := h.validateSpec(cmd, spec); err != nil {
			return fmt.Errorf("validating spec: %w", err)
		}
	}

	// Run custom validation
	if err := factory.ValidateConfig(cmd.Config); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	cmdFunc, err := factory.Create()
	if err != nil {
		return fmt.Errorf("creating handler: %w", err)
	}

	cc := &compiledCommand{
		cmd:     cmd,
		cmdFunc: cmdFunc,
	}
	if _, exists := h.compiled[id]; exists {
		return fmt.Errorf("command %q conflicts with an already registered command or alias", id)
	}
	h.compiled[id] = cc

	for _, alias := range cmd.Aliases {
		aliasId := strings.ToLower(alias)
		if _, exists := h.compiled[aliasId]; exists {
			return fmt.Errorf("alias %q conflicts with an existing command or alias", alias)
		}
		h.compiled[aliasId] = cc
	}

	return nil
}

// validateSpec validates a command against a handler's spec.
func (h *Handler) validateSpec(cmd *assets.Command, spec *HandlerSpec) error {
	// Build maps for quick lookup
	cmdTargets := make(map[string]assets.TargetSpec)
	for _, t := range cmd.Targets {
		cmdTargets[t.Name] = t
	}

	cmdConfig := make(map[string]bool)
	for k := range cmd.Config {
		cmdConfig[k] = true
	}

	// Check required targets exist and types match
	specTargets := make(map[string]bool)
	for _, req := range spec.Targets {
		specTargets[req.Name] = true

		target, exists := cmdTargets[req.Name]
		if !exists {
			if req.Required {
				return fmt.Errorf("missing required target %q", req.Name)
			}
			continue
		}

		// Validate command types are a subset of spec types
		tt := parseTargetType(target.Types)
		if tt&req.Type != tt {
			return fmt.Errorf("target %q: expected type %s, got %s", req.Name, req.Type, tt)
		}
	}

	// Check for unknown targets
	for name := range cmdTargets {
		if !specTargets[name] {
			return fmt.Errorf("unknown target %q", name)
		}
	}

	// Check required config keys exist
	specConfig := make(map[string]bool)
	for _, req := range spec.Config {
		specConfig[req.Name] = true

		if !cmdConfig[req.Name] && req.Required {
			return fmt.Errorf("missing required config key %q", req.Name)
		}
	}

	// Check for unknown config keys
	for name := range cmdConfig {
		if !specConfig[name] {
			return fmt.Errorf("unknown config key %q", name)
		}
	}

	return nil
}

// resolve finds the compiled command for a given input string.
// It tries an exact match first, then falls back to prefix matching
// with priority-based disambiguation.
func (h *Handler) resolve(input string) (*compiledCommand, error) {
	id := strings.ToLower(input)

	// Exact match always wins.
	if compiled, ok := h.compiled[id]; ok {
		return compiled, nil
	}

	// Prefix match: find all commands whose ID starts with the input.
	type match struct {
		id       string
		compiled *compiledCommand
	}
	var matches []match
	bestPriority := 0
	first := true

	for cmdId, compiled := range h.compiled {
		if strings.HasPrefix(cmdId, id) {
			p := compiled.cmd.Priority
			if first || p > bestPriority {
				bestPriority = p
				first = false
			}
			matches = append(matches, match{id: cmdId, compiled: compiled})
		}
	}

	if len(matches) == 0 {
		return nil, NewUserError(fmt.Sprintf("Command %q is unknown.", input))
	}

	// Filter to only the highest-priority matches.
	var best []match
	for _, m := range matches {
		if m.compiled.cmd.Priority == bestPriority {
			best = append(best, m)
		}
	}

	if len(best) == 1 {
		return best[0].compiled, nil
	}

	// Ambiguous — list the matching commands alphabetically.
	names := make([]string, len(best))
	for i, m := range best {
		names[i] = m.id
	}
	sort.Strings(names)
	return nil, NewUserError(fmt.Sprintf("Did you mean: %s?", strings.Join(names, ", ")))
}

// Exec executes a command with the given arguments.
func (h *Handler) Exec(ctx context.Context, world *game.WorldState, charId string, cmdName string, rawArgs ...string) error {
	compiled, err := h.resolve(cmdName)
	if err != nil {
		return err
	}

	// Parse inputs
	inputMap, err := parseInputs(compiled.cmd.Inputs, rawArgs)
	if err != nil {
		return err
	}

	char := world.GetPlayer(charId)

	// Resolve targets from targets section
	resolver := NewTargetResolver(NewWorldScopes(world))
	targets, err := resolver.ResolveSpecs(compiled.cmd.Targets, inputMap, char)
	if err != nil {
		return err
	}

	// Expand config templates
	expandedConfig, err := h.expandConfig(compiled.cmd.Config, char, targets, inputMap)
	if err != nil {
		return err
	}

	return compiled.cmdFunc(ctx, &CommandInput{
		Char:    char,
		Targets: targets,
		Config:  expandedConfig,
	})
}

// parseInputs validates raw string arguments against input specs and returns
// a map of input name to parsed value. Optional inputs are pre-populated with
// empty strings so templates don't render "<no value>".
func parseInputs(specs []assets.InputSpec, rawArgs []string) (map[string]any, error) {
	// Count required inputs
	requiredCount := 0
	for _, spec := range specs {
		if spec.Required {
			requiredCount++
		}
	}

	if len(rawArgs) < requiredCount {
		if specs[len(rawArgs)].Missing != "" {
			return nil, NewUserError(specs[len(rawArgs)].Missing)
		}
		return nil, NewUserError(fmt.Sprintf("Expected at least %d argument(s), got %d.", requiredCount, len(rawArgs)))
	}

	// If no rest input, check we don't have too many args
	hasRest := len(specs) > 0 && specs[len(specs)-1].Rest
	if !hasRest && len(rawArgs) > len(specs) {
		return nil, NewUserError(fmt.Sprintf("Expected at most %d argument(s), got %d.", len(specs), len(rawArgs)))
	}

	result := make(map[string]any, len(specs))
	argIndex := 0

	for i := range specs {
		spec := &specs[i]

		if argIndex >= len(rawArgs) {
			// No more input - this input must be optional
			if spec.Required {
				if spec.Missing != "" {
					return nil, NewUserError(spec.Missing)
				}
				return nil, NewUserError(fmt.Sprintf("Parameter %q is required.", spec.Name))
			}
			result[spec.Name] = ""
			continue
		}

		var raw string
		if spec.Rest {
			// Consume all remaining args joined with spaces
			raw = strings.Join(rawArgs[argIndex:], " ")
			argIndex = len(rawArgs)
		} else {
			raw = rawArgs[argIndex]
			argIndex++
		}

		value, err := parseValue(spec.Type, raw)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", spec.Name, err)
		}

		result[spec.Name] = value
	}

	return result, nil
}

// parseValue parses a raw string into the appropriate type.
func parseValue(inputType string, raw string) (any, error) {
	switch inputType {
	case assets.InputTypeString:
		return raw, nil

	case assets.InputTypeNumber:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, NewUserError(fmt.Sprintf("%q is not a valid number.", raw))
		}
		return n, nil

	default:
		return nil, fmt.Errorf("unknown parameter type %q", inputType)
	}
}

// templateContext holds data for template expansion.
type templateContext struct {
	Actor   *assets.Character
	Targets map[string]*TargetRef
	Inputs  map[string]any
	Color   *display.Palette
}

// expandConfig expands all template strings in config and returns map[string]string.
func (h *Handler) expandConfig(config map[string]any, char *game.CharacterInstance, targets map[string]*TargetRef, inputs map[string]any) (map[string]string, error) {
	if config == nil {
		return make(map[string]string), nil
	}

	tmplCtx := &templateContext{
		Actor:   char.Character.Get(),
		Targets: targets,
		Inputs:  inputs,
		Color:   display.Color,
	}

	expanded := make(map[string]string, len(config))
	for key, value := range config {
		str, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("config key %q: expected string, got %T", key, value)
		}
		expandedStr, err := ExpandTemplate(str, tmplCtx)
		if err != nil {
			return nil, fmt.Errorf("expanding config key %q: %w", key, err)
		}
		expanded[key] = expandedStr
	}

	return expanded, nil
}
