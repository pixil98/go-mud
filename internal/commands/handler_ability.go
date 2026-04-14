package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
)

// EffectFunc is a compiled effect closure with config baked in at registration time.
// Effects may optionally set fields on the AbilityResult to override the
// ability's template-based messages (e.g. attackEffect builds hit/miss lines).
type EffectFunc func(actor game.Actor, targets map[string]*TargetRef, result *AbilityResult) error

// EffectHandler defines an ability effect (damage, healing, buff, etc.).
// ValidateConfig checks config at registration time. Create returns a closure
// with the ability's config and target specs captured, so the closure only
// needs runtime state (actor, resolved targets).
type EffectHandler interface {
	// Spec returns target/config requirements for compile-time validation.
	// Return nil if the effect has no requirements.
	Spec() *HandlerSpec
	// ValidateConfig checks that the effect's config values are valid.
	ValidateConfig(config map[string]string) error
	// Create compiles the effect for a specific ability, returning a closure.
	// The id is a deterministic key for timed effects (e.g. "fireball:0").
	Create(id string, config map[string]string, targets []assets.TargetSpec) EffectFunc
}

// compiledAbility holds a pre-compiled ability with all config parsed at
// registration time. Used for direct execution via Handler.ExecAbility and
// wrapped by abilityCommandWrapper for command dispatch.
type compiledAbility struct {
	effectFuncs  []EffectFunc
	spec         *HandlerSpec
	resource     string
	resourceCost int
	apCost       int
	msgActor     string
	msgTarget    string
	msgRoom      string
}

// newCompiledAbility resolves the ability's effect specs against the provided
// handlers, validates config, compiles closures, and parses ability config.
func newCompiledAbility(id string, ability *assets.Ability, handlers map[string]EffectHandler) (*compiledAbility, error) {
	var effectFuncs []EffectFunc
	targets := make(map[string]TargetRequirement)
	for i, es := range ability.Effects {
		e, ok := handlers[es.Type]
		if !ok {
			return nil, fmt.Errorf("unknown effect %q", es.Type)
		}
		if err := e.ValidateConfig(es.Config); err != nil {
			return nil, fmt.Errorf("effect %q: %w", es.Type, err)
		}
		if es := e.Spec(); es != nil {
			for _, t := range es.Targets {
				if existing, ok := targets[t.Name]; ok {
					existing.Type |= t.Type
					existing.Required = existing.Required || t.Required
					targets[t.Name] = existing
				} else {
					targets[t.Name] = t
				}
			}
		}
		effectId := fmt.Sprintf("%s:%d", id, i)
		fn := e.Create(effectId, es.Config, ability.Command.Targets)
		effectFuncs = append(effectFuncs, fn)
	}

	spec := &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "ability_id", Required: true},
			{Name: "resource"},
			{Name: "resource_cost"},
			{Name: "ap_cost"},
			{Name: "message_actor"},
			{Name: "message_target"},
			{Name: "message_room"},
		},
	}
	for _, t := range targets {
		spec.Targets = append(spec.Targets, t)
	}

	config := ability.Command.Config
	resourceCost, _ := strconv.Atoi(config["resource_cost"])
	apCost, _ := strconv.Atoi(config["ap_cost"])

	return &compiledAbility{
		effectFuncs:  effectFuncs,
		spec:         spec,
		resource:     config["resource"],
		resourceCost: resourceCost,
		apCost:       apCost,
		msgActor:     config["message_actor"],
		msgTarget:    config["message_target"],
		msgRoom:      config["message_room"],
	}, nil
}

// exec runs the ability's effects and expands message templates, returning an
// AbilityResult without publishing. This is the shared core used by both the
// command handler (via abilityCommandWrapper.Create) and direct invocation
// (via Handler.ExecAbility).
func (ca *compiledAbility) exec(actor game.Actor, targets map[string]*TargetRef, opts ExecAbilityOpts) (*AbilityResult, error) {
	// Check resource cost before spending any AP.
	if ca.resourceCost > 0 {
		cur, _ := actor.Resource(ca.resource)
		if cur < ca.resourceCost {
			return nil, NewUserError(fmt.Sprintf("You don't have enough %s.", ca.resource))
		}
	}

	// Check and spend action points.
	if !opts.SkipAP {
		apCost := ca.apCost
		if apCost == 0 {
			apCost = 1
		}
		if !actor.SpendAP(apCost) {
			return nil, NewUserError("You're not ready to do that yet.")
		}
	}

	// Deduct resource cost.
	if ca.resourceCost > 0 {
		actor.AdjustResource(ca.resource, -ca.resourceCost, false)
	}

	// Expand message templates first so effects can append detail lines.
	result := &AbilityResult{}
	tmplCtx := &templateContext{
		Actor:   actor,
		Targets: targets,
		Color:   display.Color,
	}

	if ca.msgActor != "" {
		msg, err := ExpandTemplate(ca.msgActor, tmplCtx)
		if err != nil {
			return nil, fmt.Errorf("expanding actor message: %w", err)
		}
		if msg != "" {
			result.ActorLines = append(result.ActorLines, msg)
		}
	}
	if ca.msgTarget != "" {
		msg, err := ExpandTemplate(ca.msgTarget, tmplCtx)
		if err != nil {
			return nil, fmt.Errorf("expanding target message: %w", err)
		}
		if msg != "" {
			result.TargetLines = append(result.TargetLines, msg)
		}
		for _, ref := range targets {
			if ref != nil && ref.Actor != nil && ref.Actor.CharId != "" {
				result.TargetId = ref.Actor.CharId
				break
			}
		}
	}
	if ca.msgRoom != "" {
		msg, err := ExpandTemplate(ca.msgRoom, tmplCtx)
		if err != nil {
			return nil, fmt.Errorf("expanding room message: %w", err)
		}
		if msg != "" {
			result.RoomLines = append(result.RoomLines, msg)
		}
	}

	// Run effects after template expansion — effects may append detail lines.
	for _, effect := range ca.effectFuncs {
		if err := effect(actor, targets, result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// abilityCommandWrapper wraps a compiledAbility so it can be registered as a
// HandlerFactory for command dispatch. It adds the unlock check, message
// publishing, and spec/validation that the command system requires.
type abilityCommandWrapper struct {
	id  string
	ca  *compiledAbility
	pub game.Publisher
}

// Spec returns the compiled spec from the underlying compiledAbility.
func (w *abilityCommandWrapper) Spec() *HandlerSpec {
	return w.ca.spec
}

// ValidateConfig checks that resource_cost and ap_cost are non-negative integers.
func (w *abilityCommandWrapper) ValidateConfig(config map[string]string) error {
	if cost := config["resource_cost"]; cost != "" {
		n, err := strconv.Atoi(cost)
		if err != nil {
			return fmt.Errorf("resource_cost: %w", err)
		}
		if n < 0 {
			return fmt.Errorf("resource_cost must not be negative")
		}
		if n > 0 && config["resource"] == "" {
			return fmt.Errorf("resource_cost requires resource")
		}
	}
	if cost := config["ap_cost"]; cost != "" {
		n, err := strconv.Atoi(cost)
		if err != nil {
			return fmt.Errorf("ap_cost: %w", err)
		}
		if n < 0 {
			return fmt.Errorf("ap_cost must not be negative")
		}
	}
	return nil
}

// Create returns a compiled command function that checks unlock, executes the
// ability via exec(), and publishes the result messages.
func (w *abilityCommandWrapper) Create() (CommandFunc, error) {
	return Adapt[game.Actor](func(ctx context.Context, actor game.Actor, in *CommandInput) error {
		if !actor.HasGrant(assets.PerkGrantUnlockAbility, w.id) {
			return NewUserError("You don't know how to do that.")
		}

		result, err := w.ca.exec(actor, in.Targets, ExecAbilityOpts{})
		if err != nil {
			return err
		}
		return w.publishResult(result, actor)
	}), nil
}

// publishResult delivers an AbilityResult's messages to the appropriate audiences.
func (w *abilityCommandWrapper) publishResult(result *AbilityResult, actor game.Actor) error {
	if w.pub == nil {
		return nil
	}

	charId := actor.Id()
	exclude := []string{charId}

	if len(result.ActorLines) > 0 {
		actor.Notify(strings.Join(result.ActorLines, "\n"))
	}
	if len(result.TargetLines) > 0 && result.TargetId != "" {
		if err := w.pub.Publish(game.SinglePlayer(result.TargetId), nil, []byte(strings.Join(result.TargetLines, "\n"))); err != nil {
			return err
		}
		exclude = append(exclude, result.TargetId)
	}
	if len(result.RoomLines) > 0 {
		if err := w.pub.Publish(actor.Room(), exclude, []byte(strings.Join(result.RoomLines, "\n"))); err != nil {
			return err
		}
	}
	return nil
}
