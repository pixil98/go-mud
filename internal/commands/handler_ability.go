package commands

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
)

// randIntn wraps rand.IntN for testability.
var randIntn = rand.IntN

// AbilityActor provides the character state needed by the ability and effect
// subsystem. This is intentionally wide because the ability system genuinely
// touches resources, perks, combat, location, and asset data.
type AbilityActor interface {
	CommandActor
	Location() (zoneId, roomId string)
	Asset() *assets.Character
	IsInCombat() bool
	IsAlive() bool
	Level() int
	Resource(name string) (current, max int)
	AdjustResource(name string, delta int)
	SpendAP(cost int) bool
	HasGrant(key, arg string) bool
	ModifierValue(key string) int
	GrantArgs(key string) []string
	AddTimedPerks(name string, perks []assets.Perk, ticks int)
	// combat.Combatant methods — needed by attack/damage effects that call StartCombat.
	SetInCombat(bool)
	CombatTargetId() string
	SetCombatTargetId(id string)
	OnDeath() []*game.ObjectInstance
}

var _ AbilityActor = (*game.CharacterInstance)(nil)

// EffectFunc is a compiled effect closure with config baked in at registration time.
type EffectFunc func(actor AbilityActor, targets map[string]*TargetRef) error

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

// AbilityHandlerFactory creates command handlers for abilities.
// The constructor resolves effect specs, validates config, and builds closures,
// capturing everything needed so Create() returns a lightweight runtime function.
type AbilityHandlerFactory struct {
	id          string
	effects     []EffectHandler
	effectFuncs []EffectFunc
	world       WorldView
	pub         game.Publisher
}

// NewAbilityHandlerFactory resolves the ability's effect specs against the
// provided handlers, validates config, and creates compiled closures.
func NewAbilityHandlerFactory(
	id string,
	ability *assets.Ability,
	handlers map[string]EffectHandler,
	world WorldView,
	pub game.Publisher,
) (*AbilityHandlerFactory, error) {
	var effects []EffectHandler
	var effectFuncs []EffectFunc
	for i, es := range ability.Effects {
		e, ok := handlers[es.Type]
		if !ok {
			return nil, fmt.Errorf("unknown effect %q", es.Type)
		}
		if err := e.ValidateConfig(es.Config); err != nil {
			return nil, fmt.Errorf("effect %q: %w", es.Type, err)
		}
		effects = append(effects, e)
		effectId := fmt.Sprintf("%s:%d", id, i)
		fn := e.Create(effectId, es.Config, ability.Command.Targets)
		effectFuncs = append(effectFuncs, fn)
	}
	return &AbilityHandlerFactory{
		id:          id,
		effects:     effects,
		effectFuncs: effectFuncs,
		world:       world,
		pub:         pub,
	}, nil
}

func (f *AbilityHandlerFactory) Spec() *HandlerSpec {
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
	// Union target requirements from all effects.
	targets := make(map[string]TargetRequirement)
	for _, e := range f.effects {
		es := e.Spec()
		if es == nil {
			continue
		}
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
	for _, t := range targets {
		spec.Targets = append(spec.Targets, t)
	}
	return spec
}

func (f *AbilityHandlerFactory) ValidateConfig(config map[string]string) error {
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

func (f *AbilityHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[AbilityActor](func(ctx context.Context, actor AbilityActor, in *CommandInput) error {
		if !actor.HasGrant(assets.PerkGrantUnlockAbility, f.id) {
			return NewUserError("You don't know how to do that.")
		}

		return executeAbility(actor, in, in.Targets, f.world, f.pub, f.effectFuncs)
	}), nil
}

// executeAbility runs an ability's compiled effect closures and sends its messages.
// Messages are read from in.Config: message_actor, message_target, message_room.
func executeAbility(actor AbilityActor, in *CommandInput, targets map[string]*TargetRef, world WorldView, pub game.Publisher, effects []EffectFunc) error {
	resource := in.Config["resource"]
	resourceCost, _ := strconv.Atoi(in.Config["resource_cost"])
	apCost, _ := strconv.Atoi(in.Config["ap_cost"])

	// Check resource cost before spending any AP
	if resourceCost > 0 {
		cur, _ := actor.Resource(resource)
		if cur < resourceCost {
			return NewUserError(fmt.Sprintf("You don't have enough %s.", resource))
		}
	}

	// Check and spend action points
	if apCost == 0 {
		apCost = 1
	}
	if !actor.SpendAP(apCost) {
		return NewUserError("You're not ready to do that yet.")
	}

	// Deduct resource cost
	if resourceCost > 0 {
		actor.AdjustResource(resource, -resourceCost)
	}

	// Run effects in order — if any fails, don't send messages
	for _, effect := range effects {
		if err := effect(actor, targets); err != nil {
			return err
		}
	}

	if pub == nil {
		return nil
	}

	ctx := &templateContext{
		Actor:   actor.Asset(),
		Targets: targets,
		Color:   display.Color,
	}

	// Build exclude list as we send targeted messages.
	charId := actor.Id()
	exclude := []string{charId}

	// Actor message
	if tmpl := in.Config["message_actor"]; tmpl != "" {
		msg, err := ExpandTemplate(tmpl, ctx)
		if err != nil {
			return fmt.Errorf("expanding actor message: %w", err)
		}
		if err := pub.Publish(game.SinglePlayer(charId), nil, []byte(msg)); err != nil {
			return err
		}
	}

	// Target message (send to first resolved player target)
	if tmpl := in.Config["message_target"]; tmpl != "" {
		for _, ref := range targets {
			if ref != nil && ref.Player != nil {
				msg, err := ExpandTemplate(tmpl, ctx)
				if err != nil {
					return fmt.Errorf("expanding target message: %w", err)
				}
				if err := pub.Publish(game.SinglePlayer(ref.Player.CharId), nil, []byte(msg)); err != nil {
					return err
				}
				exclude = append(exclude, ref.Player.CharId)
				break
			}
		}
	}

	// Room message (exclude actor and any targeted players)
	if tmpl := in.Config["message_room"]; tmpl != "" {
		msg, err := ExpandTemplate(tmpl, ctx)
		if err != nil {
			return fmt.Errorf("expanding room message: %w", err)
		}
		zoneId, roomId := actor.Location()
		room := world.GetZone(zoneId).GetRoom(roomId)
		if err := pub.Publish(room, exclude, []byte(msg)); err != nil {
			return err
		}
	}

	return nil
}

// attackEffect queues a manual attack for the next combat tick.
type attackEffect struct {
	combat CombatManager
}

func (e *attackEffect) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeMobile, Required: true},
		},
	}
}

func (e *attackEffect) ValidateConfig(_ map[string]string) error { return nil }

func (e *attackEffect) Create(_ string, _ map[string]string, targets []assets.TargetSpec) EffectFunc {
	return func(actor AbilityActor, resolved map[string]*TargetRef) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
		}
		for _, spec := range targets {
			ref := resolved[spec.Name]
			if ref == nil || ref.Mob == nil {
				continue
			}
			if err := e.combat.StartCombat(actor, ref.Mob.instance); err != nil {
				return NewUserError(err.Error())
			}
			e.combat.QueueAttack(actor)
			return nil
		}
		return nil
	}
}

// damageEffect applies damage to the primary target and initiates combat if the target is a mob.
//
// Config fields:
//   - "damage" (int string, required): flat damage before modifiers.
//   - "damage_types" (comma-separated string, optional): damage type tags (e.g. "fire,ice").
//
type damageEffect struct {
	combat CombatManager
}

func (e *damageEffect) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeMobile | targetTypePlayer, Required: true},
		},
	}
}

func (e *damageEffect) ValidateConfig(config map[string]string) error {
	if _, err := strconv.Atoi(config["damage"]); err != nil {
		return fmt.Errorf("damage config required")
	}
	return nil
}

func (e *damageEffect) Create(_ string, config map[string]string, targets []assets.TargetSpec) EffectFunc {
	baseDamage, _ := strconv.Atoi(config["damage"])

	var damageTypes []string
	if dt := config["damage_types"]; dt != "" {
		damageTypes = strings.Split(dt, ",")
	}
	primaryType := assets.DamageTypeUntyped
	if len(damageTypes) > 0 {
		primaryType = damageTypes[0]
	}

	return func(actor AbilityActor, resolved map[string]*TargetRef) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
		}

		raw := baseDamage
		if raw < 1 {
			raw = 1
		}

		for _, spec := range targets {
			ref := resolved[spec.Name]
			if ref == nil {
				continue
			}
			damage, reflected := combat.CalcDamage(raw, primaryType, actor, targetPerkReader(ref))
			applyDamage(ref, damage)
			if reflected > 0 {
				actor.AdjustResource(assets.ResourceHp, -reflected)
			}
			if ref.Mob != nil {
				_ = e.combat.StartCombat(actor, ref.Mob.instance)
				e.combat.AddThreat(actor, ref.Mob.instance, damage)
			}
			return nil
		}

		return nil
	}
}

// aoeDamageEffect applies damage to all mobs and (optionally) players in the caster's room.
//
// Config fields:
//   - "damage" (int string, required): flat damage before modifiers.
//   - "damage_type" (string, optional): damage type tag. Defaults to untyped.
//   - "hit_players" ("true"/"false", optional): whether to hit other players. Default false.
type aoeDamageEffect struct {
	combat CombatManager
	world  ZoneLocator
}

func (e *aoeDamageEffect) Spec() *HandlerSpec { return nil }

func (e *aoeDamageEffect) ValidateConfig(config map[string]string) error {
	if _, err := strconv.Atoi(config["damage"]); err != nil {
		return fmt.Errorf("damage config required")
	}
	return nil
}

func (e *aoeDamageEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	baseDamage, _ := strconv.Atoi(config["damage"])
	dmgType := config["damage_type"]
	if dmgType == "" {
		dmgType = assets.DamageTypeUntyped
	}
	hitPlayers := config["hit_players"] == "true"

	return func(actor AbilityActor, _ map[string]*TargetRef) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
		}

		raw := baseDamage
		if raw < 1 {
			raw = 1
		}

		zoneId, roomId := actor.Location()
		zi := e.world.GetZone(zoneId)
		if zi == nil {
			return nil
		}
		ri := zi.GetRoom(roomId)
		if ri == nil {
			return nil
		}

		ri.ForEachMob(func(mi *game.MobileInstance) {
			damage, reflected := combat.CalcDamage(raw, dmgType, actor, mi)
			mi.AdjustResource(assets.ResourceHp, -damage)
			if reflected > 0 {
				actor.AdjustResource(assets.ResourceHp, -reflected)
			}
			_ = e.combat.StartCombat(actor, mi)
			e.combat.AddThreat(actor, mi, damage)
		})

		if hitPlayers {
			actorId := actor.Id()
			ri.ForEachPlayer(func(charId string, ci *game.CharacterInstance) {
				if charId == actorId {
					return
				}
				damage, reflected := combat.CalcDamage(raw, dmgType, actor, ci)
				ci.AdjustResource(assets.ResourceHp, -damage)
				if reflected > 0 {
					actor.AdjustResource(assets.ResourceHp, -reflected)
				}
			})
		}

		return nil
	}
}

// targetPerkReader returns a combat.PerkReader for the target so CalcDamage can
// apply defense perks. Falls back to a no-op reader if the ref has no instance.
func targetPerkReader(ref *TargetRef) combat.PerkReader {
	if ref.Player != nil {
		return ref.Player.session
	}
	if ref.Mob != nil {
		return ref.Mob.instance
	}
	return nopPerkReader{}
}

type nopPerkReader struct{}

func (nopPerkReader) ModifierValue(_ string) int { return 0 }

// applyDamage reduces a target's HP by amount.
func applyDamage(ref *TargetRef, amount int) {
	if ref.Player != nil {
		ref.Player.session.AdjustResource(assets.ResourceHp, -amount)
	} else if ref.Mob != nil {
		ref.Mob.instance.AdjustResource(assets.ResourceHp, -amount)
	}
}

// parsePerkBuffConfig extracts the common config fields for perk buff effects.
// Each effect entry holds a single perk; repeat the effect for multiple perks.
//
// Config fields:
//   - "perk_type" (string, required): perk type (e.g. "modifier", "grant").
//   - "perk_key" (string, required): the perk key.
//   - "perk_value" (int string, optional): perk value (for modifiers).
//   - "perk_arg" (string, optional): perk argument (for grants).
//   - "duration" (int string, required): number of ticks the perk lasts.
//   - "name" (string, optional): entry name for the timed perk. Defaults to id.
func parsePerkBuffConfig(id string, config map[string]string) (string, assets.Perk, int) {
	dur, _ := strconv.Atoi(config["duration"])
	perkValue, _ := strconv.Atoi(config["perk_value"])

	perk := assets.Perk{
		Type:  config["perk_type"],
		Key:   config["perk_key"],
		Value: perkValue,
		Arg:   config["perk_arg"],
	}

	name := config["name"]
	if name == "" {
		name = id
	}

	return name, perk, dur
}

// validatePerkBuffConfig checks the required config for perk buff effects.
func validatePerkBuffConfig(config map[string]string) error {
	dur, err := strconv.Atoi(config["duration"])
	if err != nil || dur <= 0 {
		return fmt.Errorf("positive duration config required")
	}
	if config["perk_type"] == "" {
		return fmt.Errorf("perk_type config required")
	}
	if config["perk_key"] == "" {
		return fmt.Errorf("perk_key config required")
	}
	return nil
}

// actorBuffEffect applies a timed perk to a target player/mob, or self if no target.
type actorBuffEffect struct{}

func (e *actorBuffEffect) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeMobile | targetTypePlayer, Required: false},
		},
	}
}

func (e *actorBuffEffect) ValidateConfig(config map[string]string) error {
	return validatePerkBuffConfig(config)
}

func (e *actorBuffEffect) Create(id string, config map[string]string, targets []assets.TargetSpec) EffectFunc {
	name, perk, dur := parsePerkBuffConfig(id, config)
	perks := []assets.Perk{perk}

	return func(actor AbilityActor, resolved map[string]*TargetRef) error {
		for _, spec := range targets {
			ref := resolved[spec.Name]
			if ref == nil {
				continue
			}
			if ref.Player != nil {
				ref.Player.session.AddTimedPerks(name, perks, dur)
				return nil
			}
			if ref.Mob != nil {
				ref.Mob.instance.AddTimedPerks(name, perks, dur)
				return nil
			}
		}
		actor.AddTimedPerks(name, perks, dur)
		return nil
	}
}

// roomBuffEffect applies a timed perk to the caster's current room.
type roomBuffEffect struct {
	world ZoneLocator
}

func (e *roomBuffEffect) Spec() *HandlerSpec { return nil }

func (e *roomBuffEffect) ValidateConfig(config map[string]string) error {
	return validatePerkBuffConfig(config)
}

func (e *roomBuffEffect) Create(id string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	name, perk, dur := parsePerkBuffConfig(id, config)
	perks := []assets.Perk{perk}

	return func(actor AbilityActor, _ map[string]*TargetRef) error {
		zoneId, roomId := actor.Location()
		room := e.world.GetZone(zoneId).GetRoom(roomId)
		if room == nil {
			return fmt.Errorf("room_buff effect: room not found")
		}
		room.Perks.AddTimedPerks(name, perks, dur)
		return nil
	}
}

// zoneBuffEffect applies a timed perk to the caster's current zone.
type zoneBuffEffect struct {
	world WorldView
}

func (e *zoneBuffEffect) Spec() *HandlerSpec { return nil }

func (e *zoneBuffEffect) ValidateConfig(config map[string]string) error {
	return validatePerkBuffConfig(config)
}

func (e *zoneBuffEffect) Create(id string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	name, perk, dur := parsePerkBuffConfig(id, config)
	perks := []assets.Perk{perk}

	return func(actor AbilityActor, _ map[string]*TargetRef) error {
		zoneId, _ := actor.Location()
		zone := e.world.GetZone(zoneId)
		if zone == nil {
			return fmt.Errorf("zone_buff effect: zone not found")
		}
		zone.Perks.AddTimedPerks(name, perks, dur)
		return nil
	}
}

// worldBuffEffect applies a timed perk to the entire world.
type worldBuffEffect struct {
	world *game.WorldState
}

func (e *worldBuffEffect) Spec() *HandlerSpec { return nil }

func (e *worldBuffEffect) ValidateConfig(config map[string]string) error {
	return validatePerkBuffConfig(config)
}

func (e *worldBuffEffect) Create(id string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	name, perk, dur := parsePerkBuffConfig(id, config)
	perks := []assets.Perk{perk}

	return func(_ AbilityActor, _ map[string]*TargetRef) error {
		e.world.Perks.AddTimedPerks(name, perks, dur)
		return nil
	}
}
