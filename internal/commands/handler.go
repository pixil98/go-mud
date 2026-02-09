package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/plugins"
	"github.com/pixil98/go-mud/internal/storage"
)

// ParsedInput represents a validated and parsed command input.
type ParsedInput struct {
	Spec  *InputSpec
	Raw   string // Original player input
	Value any    // Parsed value: int for number, string for string/direction
}

// CommandContext is what handlers receive after config processing.
type CommandContext struct {
	Actor   *game.Character   // Character data (name, title, etc.)
	Session *game.PlayerState // Current session state (location, quit flag, etc.)
	World   *game.WorldState
	Config  map[string]any // Config with $resolve directives processed
	Inputs  map[string]any // Parsed input values keyed by name
}

// CommandFunc is the signature for compiled command functions.
// It receives CommandContext with fully expanded config.
type CommandFunc func(ctx context.Context, cmdCtx *CommandContext) error

// HandlerFactory creates CommandFuncs from command configurations.
// Implementations should expose their expected config structure.
// Factories that need a Publisher should accept it in their constructor.
type HandlerFactory interface {
	// ValidateConfig validates that the config contains required fields.
	ValidateConfig(config map[string]any) error
	// Create creates a CommandFunc. Handlers read expanded config from CommandContext.
	Create() (CommandFunc, error)
}

// compiledCommand holds a command that's been validated and compiled.
type compiledCommand struct {
	cmd     *Command
	cmdFunc CommandFunc
}

// Publisher provides the ability to publish messages to subjects
type Publisher interface {
	Publish(subject string, data []byte) error
}

type Handler struct {
	factories map[string]HandlerFactory
	compiled  map[storage.Identifier]*compiledCommand
}

func NewHandler(c storage.Storer[*Command], publisher Publisher, world *game.WorldState, charInfo plugins.CharacterInfoProvider) (*Handler, error) {
	h := &Handler{
		factories: make(map[string]HandlerFactory),
		compiled:  make(map[storage.Identifier]*compiledCommand),
	}

	// Register built-in handlers
	h.RegisterFactory("message", NewMessageHandlerFactory(publisher))
	h.RegisterFactory("quit", &QuitHandlerFactory{})
	h.RegisterFactory("move", NewMoveHandlerFactory(world, publisher))
	h.RegisterFactory("look", NewLookHandlerFactory(world, publisher))
	h.RegisterFactory("who", NewWhoHandlerFactory(world, publisher, charInfo))
	h.RegisterFactory("title", NewTitleHandlerFactory(publisher))

	// Compile commands
	for id, cmd := range c.GetAll() {
		err := h.compile(id, cmd)
		if err != nil {
			return nil, fmt.Errorf("compiling command %q: %w", id, err)
		}
	}

	return h, nil
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

func (h *Handler) compile(id storage.Identifier, cmd *Command) error {
	factory, ok := h.factories[cmd.Handler]
	if !ok {
		return fmt.Errorf("unknown handler %q", cmd.Handler)
	}

	if err := factory.ValidateConfig(cmd.Config); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	cmdFunc, err := factory.Create()
	if err != nil {
		return fmt.Errorf("creating handler: %w", err)
	}

	h.compiled[id] = &compiledCommand{
		cmd:     cmd,
		cmdFunc: cmdFunc,
	}
	return nil
}

// Exec executes a command with the given arguments.
func (h *Handler) Exec(ctx context.Context, world *game.WorldState, charId storage.Identifier, cmdName string, rawArgs ...string) error {
	compiled, ok := h.compiled[storage.Identifier(strings.ToLower(cmdName))]
	if !ok {
		return NewUserError(fmt.Sprintf("Unknown command: %s", cmdName))
	}

	// Validate and parse inputs
	inputs, err := h.parseInputs(compiled.cmd.Inputs, rawArgs)
	if err != nil {
		return err
	}

	// Build input map for template expansion
	inputMap := make(map[string]any, len(inputs))
	for _, input := range inputs {
		inputMap[input.Spec.Name] = input.Value
	}

	// Process $resolve directives in config
	actor := world.Characters().Get(string(charId))
	session := world.GetPlayer(charId)
	resolver := NewResolver(world)

	processedConfig, err := h.expandConfig(compiled.cmd.Config, inputMap, session, resolver)
	if err != nil {
		return err
	}

	// Build context for template expansion
	cmdCtx := &CommandContext{
		Actor:   actor,
		Session: session,
		World:   world,
		Config:  processedConfig,
		Inputs:  inputMap,
	}

	// Expand all template strings in config
	expandedConfig, err := h.expandTemplates(processedConfig, cmdCtx)
	if err != nil {
		return err
	}
	cmdCtx.Config = expandedConfig

	return compiled.cmdFunc(ctx, cmdCtx)
}

// parseInputs validates raw string arguments against input specs.
func (h *Handler) parseInputs(specs []InputSpec, rawArgs []string) ([]ParsedInput, error) {
	// Count required inputs
	requiredCount := 0
	for _, spec := range specs {
		if spec.Required {
			requiredCount++
		}
	}

	if len(rawArgs) < requiredCount {
		return nil, NewUserError(fmt.Sprintf("Expected at least %d argument(s), got %d", requiredCount, len(rawArgs)))
	}

	// If no rest input, check we don't have too many args
	hasRest := len(specs) > 0 && specs[len(specs)-1].Rest
	if !hasRest && len(rawArgs) > len(specs) {
		return nil, NewUserError(fmt.Sprintf("Expected at most %d argument(s), got %d", len(specs), len(rawArgs)))
	}

	inputs := make([]ParsedInput, 0, len(specs))
	argIndex := 0

	for i := range specs {
		spec := &specs[i]

		if argIndex >= len(rawArgs) {
			// No more input - this input must be optional
			if spec.Required {
				return nil, NewUserError(fmt.Sprintf("Missing required parameter: %s", spec.Name))
			}
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

		value, err := h.parseValue(spec.Type, raw)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", spec.Name, err)
		}

		inputs = append(inputs, ParsedInput{
			Spec:  spec,
			Raw:   raw,
			Value: value,
		})
	}

	return inputs, nil
}

// parseValue parses a raw string into the appropriate type.
func (h *Handler) parseValue(inputType InputType, raw string) (any, error) {
	switch inputType {
	case InputTypeString:
		return raw, nil

	case InputTypeNumber:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, NewUserError(fmt.Sprintf("%q is not a valid number", raw))
		}
		return n, nil

	case InputTypeDirection:
		// Could validate against known directions here
		return raw, nil

	default:
		return nil, fmt.Errorf("unknown parameter type %q", inputType)
	}
}

// expandConfig processes $resolve directives in config.
// Strings are passed through unchanged - handlers expand templates themselves.
func (h *Handler) expandConfig(config map[string]any, inputs map[string]any, actorState *game.PlayerState, resolver *Resolver) (map[string]any, error) {
	if config == nil {
		return make(map[string]any), nil
	}

	expanded := make(map[string]any, len(config))

	for key, value := range config {
		expandedValue, err := h.expandValue(value, inputs, actorState, resolver)
		if err != nil {
			return nil, fmt.Errorf("expanding config key %q: %w", key, err)
		}
		expanded[key] = expandedValue
	}

	return expanded, nil
}

// expandValue processes a single config value, handling $resolve directives.
// Strings pass through unchanged - handlers expand templates with full context.
func (h *Handler) expandValue(value any, inputs map[string]any, actorState *game.PlayerState, resolver *Resolver) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		// Check if this is a $resolve directive
		if directive, ok := IsResolveDirective(v); ok {
			return h.processResolveDirective(directive, inputs, actorState, resolver)
		}
		// Otherwise recursively process the map
		expanded := make(map[string]any, len(v))
		for k, val := range v {
			expandedVal, err := h.expandValue(val, inputs, actorState, resolver)
			if err != nil {
				return nil, err
			}
			expanded[k] = expandedVal
		}
		return expanded, nil

	case []any:
		// Recursively process array elements
		expanded := make([]any, len(v))
		for i, val := range v {
			expandedVal, err := h.expandValue(val, inputs, actorState, resolver)
			if err != nil {
				return nil, err
			}
			expanded[i] = expandedVal
		}
		return expanded, nil

	default:
		// Pass through strings, numbers, bools, nil unchanged
		return value, nil
	}
}

// processResolveDirective handles a $resolve directive, resolving the target.
func (h *Handler) processResolveDirective(directive *ResolveDirective, inputs map[string]any, actorState *game.PlayerState, resolver *Resolver) (any, error) {
	// Get the input value to resolve
	var name string
	if directive.Input != "" {
		inputValue, exists := inputs[directive.Input]
		if !exists || inputValue == nil || inputValue == "" {
			if directive.Optional {
				return nil, nil
			}
			return nil, NewUserError(fmt.Sprintf("Missing required input: %s", directive.Input))
		}
		var ok bool
		name, ok = inputValue.(string)
		if !ok {
			return nil, fmt.Errorf("input %q is not a string", directive.Input)
		}
	}

	if name == "" {
		if directive.Optional {
			return nil, nil
		}
		return nil, NewUserError("No target specified")
	}

	// Resolve the target
	return resolver.Resolve(actorState, name, directive.Resolve, directive.Scope)
}

// expandTemplates recursively expands all template strings in config.
func (h *Handler) expandTemplates(config map[string]any, cmdCtx *CommandContext) (map[string]any, error) {
	if config == nil {
		return make(map[string]any), nil
	}

	expanded := make(map[string]any, len(config))
	for key, value := range config {
		expandedValue, err := h.expandTemplateValue(value, cmdCtx)
		if err != nil {
			return nil, fmt.Errorf("expanding template for key %q: %w", key, err)
		}
		expanded[key] = expandedValue
	}
	return expanded, nil
}

// expandTemplateValue expands templates in a single value.
func (h *Handler) expandTemplateValue(value any, cmdCtx *CommandContext) (any, error) {
	switch v := value.(type) {
	case string:
		return ExpandTemplate(v, cmdCtx)

	case map[string]any:
		expanded := make(map[string]any, len(v))
		for k, val := range v {
			expandedVal, err := h.expandTemplateValue(val, cmdCtx)
			if err != nil {
				return nil, err
			}
			expanded[k] = expandedVal
		}
		return expanded, nil

	case []any:
		expanded := make([]any, len(v))
		for i, val := range v {
			expandedVal, err := h.expandTemplateValue(val, cmdCtx)
			if err != nil {
				return nil, err
			}
			expanded[i] = expandedVal
		}
		return expanded, nil

	default:
		return value, nil
	}
}
