package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
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
type HandlerFactory interface {
	// ValidateConfig validates that the config contains required fields.
	ValidateConfig(config map[string]any) error
	// Create creates a CommandFunc from the validated config.
	Create(config map[string]any, pub Publisher) (CommandFunc, error)
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
	store     storage.Storer[*Command]
	factories map[string]HandlerFactory
	compiled  map[storage.Identifier]*compiledCommand
	publisher Publisher
}

func NewHandler(c storage.Storer[*Command], publisher Publisher) *Handler {
	h := &Handler{
		store:     c,
		factories: make(map[string]HandlerFactory),
		compiled:  make(map[storage.Identifier]*compiledCommand),
		publisher: publisher,
	}
	// Register built-in handlers
	h.RegisterFactory("message", &MessageHandlerFactory{})
	h.RegisterFactory("quit", &QuitHandlerFactory{})
	return h
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

// CompileAll compiles all commands from the store.
// Call this after all handler factories have been registered.
func (h *Handler) CompileAll() error {
	for id, cmd := range h.store.GetAll() {
		err := h.compile(id, cmd)
		if err != nil {
			return fmt.Errorf("compiling command %q: %w", id, err)
		}
	}
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

	cmdFunc, err := factory.Create(cmd.Config, h.publisher)
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

// MessageHandlerFactory creates handlers that publish messages to channels.
// Config:
//   - recipient_channel (optional): template for the recipient's channel
//   - recipient_message (required if recipient_channel set): template for message to recipient
//   - sender_channel (optional): template for sender's channel (for confirmation)
//   - sender_message (required if sender_channel set): template for sender confirmation
type MessageHandlerFactory struct{}

func (f *MessageHandlerFactory) ValidateConfig(config map[string]any) error {
	recipientChannel, _ := config["recipient_channel"].(string)
	recipientMessage, _ := config["recipient_message"].(string)
	if recipientChannel != "" && recipientMessage == "" {
		return fmt.Errorf("recipient_message is required when recipient_channel is set")
	}

	senderChannel, _ := config["sender_channel"].(string)
	senderMessage, _ := config["sender_message"].(string)
	if senderChannel != "" && senderMessage == "" {
		return fmt.Errorf("sender_message is required when sender_channel is set")
	}

	if recipientChannel == "" && senderChannel == "" {
		return fmt.Errorf("at least one of recipient_channel or sender_channel is required")
	}

	return nil
}

func (f *MessageHandlerFactory) Create(config map[string]any, pub Publisher) (CommandFunc, error) {
	recipientChannel, _ := config["recipient_channel"].(string)
	recipientMessage, _ := config["recipient_message"].(string)
	senderChannel, _ := config["sender_channel"].(string)
	senderMessage, _ := config["sender_message"].(string)

	return func(ctx context.Context, data *TemplateData) error {
		// Send confirmation to sender if configured
		if senderChannel != "" {
			channel, err := ExpandTemplate(senderChannel, data)
			if err != nil {
				return fmt.Errorf("expanding sender channel template: %w", err)
			}

			message, err := ExpandTemplate(senderMessage, data)
			if err != nil {
				return fmt.Errorf("expanding sender message template: %w", err)
			}

			if err := pub.Publish(channel, []byte(message)); err != nil {
				return err
			}
		}

		// Send message to recipient if configured
		if recipientChannel != "" {
			channel, err := ExpandTemplate(recipientChannel, data)
			if err != nil {
				return fmt.Errorf("expanding recipient channel template: %w", err)
			}

			message, err := ExpandTemplate(recipientMessage, data)
			if err != nil {
				return fmt.Errorf("expanding recipient message template: %w", err)
			}

			if err := pub.Publish(channel, []byte(message)); err != nil {
				return err
			}
		}

		return nil
	}, nil
}

// QuitHandlerFactory creates handlers that signal the player wants to quit.
type QuitHandlerFactory struct{}

func (f *QuitHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *QuitHandlerFactory) Create(config map[string]any, pub Publisher) (CommandFunc, error) {
	return func(ctx context.Context, data *TemplateData) error {
		if data.State != nil {
			data.State.Quit = true
		}
		return nil
	}, nil
}
