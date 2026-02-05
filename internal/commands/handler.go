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

// ParsedArg represents a validated and parsed command argument.
type ParsedArg struct {
	Spec  *ParamSpec
	Raw   string // Original player input
	Value any    // Parsed value: int for number, string for string/direction, etc.
}

// CommandFunc is the signature for compiled command functions.
// It receives template data containing player state and parsed arguments.
type CommandFunc func(ctx context.Context, data *TemplateData) error

// HandlerFactory creates CommandFuncs from command configurations.
// Implementations should expose their expected config structure.
// Factories that need a Publisher should accept it in their constructor.
type HandlerFactory interface {
	// ValidateConfig validates that the config contains required fields.
	ValidateConfig(config map[string]any) error
	// Create creates a CommandFunc from the validated config.
	Create(config map[string]any) (CommandFunc, error)
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

	cmdFunc, err := factory.Create(cmd.Config)
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

	// Validate and parse arguments
	args, err := h.parseArgs(compiled.cmd.Params, rawArgs)
	if err != nil {
		return err
	}

	// Build template data from world state and args
	// This resolves target-type parameters
	data, err := NewTemplateData(world, charId, args)
	if err != nil {
		return err
	}

	return compiled.cmdFunc(ctx, data)
}

// parseArgs validates raw string arguments against parameter specs.
func (h *Handler) parseArgs(specs []ParamSpec, rawArgs []string) ([]ParsedArg, error) {
	// Count required params (rest params only need 1 word minimum)
	requiredCount := 0
	for _, spec := range specs {
		if spec.Required {
			requiredCount++
		}
	}

	if len(rawArgs) < requiredCount {
		return nil, NewUserError(fmt.Sprintf("Expected at least %d argument(s), got %d", requiredCount, len(rawArgs)))
	}

	// If no rest param, check we don't have too many args
	hasRest := len(specs) > 0 && specs[len(specs)-1].Rest
	if !hasRest && len(rawArgs) > len(specs) {
		return nil, NewUserError(fmt.Sprintf("Expected at most %d argument(s), got %d", len(specs), len(rawArgs)))
	}

	args := make([]ParsedArg, 0, len(specs))
	argIndex := 0

	for i := range specs {
		spec := &specs[i]

		if argIndex >= len(rawArgs) {
			// No more input - this param must be optional
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

		args = append(args, ParsedArg{
			Spec:  spec,
			Raw:   raw,
			Value: value,
		})
	}

	return args, nil
}

// parseValue parses a raw string into the appropriate type.
// For types that require game state (target, player, mob, item),
// this returns the raw string. Resolution happens at a higher level.
func (h *Handler) parseValue(paramType ParamType, raw string) (any, error) {
	switch paramType {
	case ParamTypeString:
		return raw, nil

	case ParamTypeNumber:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, NewUserError(fmt.Sprintf("%q is not a valid number", raw))
		}
		return n, nil

	case ParamTypeDirection:
		// Could validate against known directions here
		return raw, nil

	case ParamTypeTarget, ParamTypePlayer, ParamTypeMob, ParamTypeItem:
		// These require game state to resolve. Return raw string for now.
		// Resolution will happen at a higher level that has access to game state.
		return raw, nil

	default:
		return nil, fmt.Errorf("unknown parameter type %q", paramType)
	}
}
