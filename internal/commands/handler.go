package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
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
	Targets map[string]*TargetRef // Resolved targets by name
	Config  map[string]string     // Expanded config values (all templates resolved)
}

// CommandFunc is the signature for compiled command functions.
// It receives CommandContext with fully expanded config.
type CommandFunc func(ctx context.Context, cmdCtx *CommandContext) error

// TargetRequirement describes an expected target for a handler.
type TargetRequirement struct {
	Name     string     // Target name (must match JSON "name" field)
	Type     TargetType // Expected type: TargetTypePlayer, TargetTypeObject, etc.
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
	// Create creates a CommandFunc. Handlers read expanded config from CommandContext.
	Create() (CommandFunc, error)
}

// compiledCommand holds a command that's been validated and compiled.
type compiledCommand struct {
	cmd     *Command
	cmdFunc CommandFunc
}

// Publisher provides typed methods for publishing messages to game channels.
type Publisher interface {
	// PublishToPlayer sends a message to a specific player's channel.
	PublishToPlayer(charId storage.Identifier, data []byte) error
	// PublishToRoom sends a message to all subscribers in a room.
	PublishToRoom(zoneId, roomId storage.Identifier, data []byte) error
	// PublishToZone sends a message to all subscribers in a zone.
	PublishToZone(zoneId storage.Identifier, data []byte) error
	// PublishToWorld sends a message to all subscribers on the world channel.
	PublishToWorld(data []byte) error
	// Publish sends a message to an arbitrary subject.
	// TODO: Evaluate whether this can be replaced with typed methods now that
	// plugins won't define custom channels.
	Publish(subject string, data []byte) error
}

type Handler struct {
	factories map[string]HandlerFactory
	compiled  map[storage.Identifier]*compiledCommand
}

func NewHandler(c storage.Storer[*Command], publisher Publisher, world *game.WorldState, races storage.Storer[*game.Race]) (*Handler, error) {
	h := &Handler{
		factories: make(map[string]HandlerFactory),
		compiled:  make(map[storage.Identifier]*compiledCommand),
	}

	// Register built-in handlers
	h.RegisterFactory("equipment", NewEquipmentHandlerFactory(world, publisher))
	h.RegisterFactory("help", NewHelpHandlerFactory(c, publisher))
	h.RegisterFactory("inventory", NewInventoryHandlerFactory(world, publisher))
	h.RegisterFactory("look", NewLookHandlerFactory(world, publisher))
	h.RegisterFactory("message", NewMessageHandlerFactory(publisher))
	h.RegisterFactory("move", NewMoveHandlerFactory(world, publisher))
	h.RegisterFactory("move_obj", NewMoveObjHandlerFactory(world, publisher))
	h.RegisterFactory("quit", NewQuitHandlerFactory(world))
	h.RegisterFactory("save", NewSaveHandlerFactory(world, publisher))
	h.RegisterFactory("title", NewTitleHandlerFactory(publisher))
	h.RegisterFactory("wear", NewWearHandlerFactory(world, publisher))
	h.RegisterFactory("who", NewWhoHandlerFactory(world, publisher, races))

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

	h.compiled[id] = &compiledCommand{
		cmd:     cmd,
		cmdFunc: cmdFunc,
	}
	return nil
}

// validateSpec validates a command against a handler's spec.
func (h *Handler) validateSpec(cmd *Command, spec *HandlerSpec) error {
	// Build maps for quick lookup
	cmdTargets := make(map[string]TargetSpec)
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

		// Validate type matches
		if target.Type != req.Type {
			return fmt.Errorf("target %q: expected type %s, got %s", req.Name, req.Type.String(), target.Type.String())
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

// Exec executes a command with the given arguments.
func (h *Handler) Exec(ctx context.Context, world *game.WorldState, charId storage.Identifier, cmdName string, rawArgs ...string) error {
	compiled, ok := h.compiled[storage.Identifier(strings.ToLower(cmdName))]
	if !ok {
		return NewUserError(fmt.Sprintf("Command %q is unknown.", cmdName))
	}

	// Parse inputs
	inputs, err := h.parseInputs(compiled.cmd.Inputs, rawArgs)
	if err != nil {
		return err
	}

	// Build input map for template expansion and target resolution.
	// Pre-populate optional inputs with zero values so templates don't render "<no value>".
	inputMap := make(map[string]any, len(compiled.cmd.Inputs))
	for _, spec := range compiled.cmd.Inputs {
		if !spec.Required {
			inputMap[spec.Name] = ""
		}
	}
	for _, input := range inputs {
		inputMap[input.Spec.Name] = input.Value
	}

	actor := world.Characters().Get(string(charId))
	session := world.GetPlayer(charId)

	// Resolve targets from targets section
	targets, err := h.resolveTargets(compiled.cmd.Targets, inputMap, charId, world)
	if err != nil {
		return err
	}

	// Expand config templates
	expandedConfig, err := h.expandConfig(compiled.cmd.Config, actor, session, targets, inputMap)
	if err != nil {
		return err
	}

	cmdCtx := &CommandContext{
		Actor:   actor,
		Session: session,
		World:   world,
		Targets: targets,
		Config:  expandedConfig,
	}

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
		return nil, NewUserError(fmt.Sprintf("Expected at least %d argument(s), got %d.", requiredCount, len(rawArgs)))
	}

	// If no rest input, check we don't have too many args
	hasRest := len(specs) > 0 && specs[len(specs)-1].Rest
	if !hasRest && len(rawArgs) > len(specs) {
		return nil, NewUserError(fmt.Sprintf("Expected at most %d argument(s), got %d.", len(specs), len(rawArgs)))
	}

	inputs := make([]ParsedInput, 0, len(specs))
	argIndex := 0

	for i := range specs {
		spec := &specs[i]

		if argIndex >= len(rawArgs) {
			// No more input - this input must be optional
			if spec.Required {
				return nil, NewUserError(fmt.Sprintf("Parameter %q is required.", spec.Name))
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
			return nil, NewUserError(fmt.Sprintf("%q is not a valid number.", raw))
		}
		return n, nil

	default:
		return nil, fmt.Errorf("unknown parameter type %q", inputType)
	}
}

// resolveTargets resolves all targets from the targets section.
func (h *Handler) resolveTargets(specs []TargetSpec, inputs map[string]any, charId storage.Identifier, world *game.WorldState) (map[string]*TargetRef, error) {
	if len(specs) == 0 {
		return make(map[string]*TargetRef), nil
	}

	resolver := NewResolver(world)
	targets := make(map[string]*TargetRef, len(specs))

	for _, spec := range specs {
		// Get the input value
		inputValue, exists := inputs[spec.Input]
		if !exists || inputValue == nil || inputValue == "" {
			if spec.Optional {
				targets[spec.Name] = nil
				continue
			}
			return nil, NewUserError(fmt.Sprintf("Input %q is required.", spec.Input))
		}

		name, ok := inputValue.(string)
		if !ok {
			return nil, fmt.Errorf("input %q is not a string", spec.Input)
		}

		// Set scope contents if applicable (search inside another resolved target).
		// When the scope target was resolved, restrict to ScopeContents only so we
		// don't fall through to room/inventory and pick up the wrong item.
		// When the scope target was not provided (optional), use all declared scopes.
		resolver.scopeContents = nil
		scope := spec.Scope()
		if spec.ScopeTarget != "" {
			scopeRef := targets[spec.ScopeTarget]
			if scopeRef != nil && scopeRef.Obj != nil && scopeRef.Obj.Instance != nil {
				objDef := world.Objects().Get(string(scopeRef.Obj.ObjectId))
				if objDef == nil || !objDef.HasFlag(game.ObjectFlagContainer) {
					name := strings.ToUpper(scopeRef.Obj.Name[:1]) + scopeRef.Obj.Name[1:]
					return nil, NewUserError(fmt.Sprintf("%s is not a container.", name))
				}
				resolver.scopeContents = scopeRef.Obj.Instance.Contents
				scope = ScopeContents
			}
		}

		// Resolve the target
		resolved, err := resolver.Resolve(charId, name, spec.Type, scope)
		if err != nil {
			return nil, err
		}

		// Convert to TargetRef
		switch t := resolved.(type) {
		case *PlayerRef:
			targets[spec.Name] = &TargetRef{Type: "player", Player: t}
		case *MobileRef:
			targets[spec.Name] = &TargetRef{Type: "mobile", Mob: t}
		case *ObjectRef:
			targets[spec.Name] = &TargetRef{Type: "object", Obj: t}
		case *TargetRef:
			targets[spec.Name] = t
		default:
			return nil, fmt.Errorf("unexpected resolved type: %T", resolved)
		}
	}

	return targets, nil
}

// templateContext holds data for template expansion.
type templateContext struct {
	Actor   *game.Character
	Session *game.PlayerState
	Targets map[string]*TargetRef
	Inputs  map[string]any
}

// expandConfig expands all template strings in config and returns map[string]string.
func (h *Handler) expandConfig(config map[string]any, actor *game.Character, session *game.PlayerState, targets map[string]*TargetRef, inputs map[string]any) (map[string]string, error) {
	if config == nil {
		return make(map[string]string), nil
	}

	tmplCtx := &templateContext{
		Actor:   actor,
		Session: session,
		Targets: targets,
		Inputs:  inputs,
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
