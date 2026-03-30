package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
	"github.com/pixil98/go-mud/internal/storage"
)

// EffectFunc is a compiled effect closure with config baked in at registration time.
// Effects may optionally set fields on the AbilityResult to override the
// ability's template-based messages (e.g. attackEffect builds hit/miss lines).
type EffectFunc func(actor shared.Actor, targets map[string]*TargetRef, result *AbilityResult) error

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
func (ca *compiledAbility) exec(actor shared.Actor, targets map[string]*TargetRef, opts ExecAbilityOpts) (*AbilityResult, error) {
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
	id    string
	ca    *compiledAbility
	world WorldView
	pub   game.Publisher
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
	return Adapt[shared.Actor](func(ctx context.Context, actor shared.Actor, in *CommandInput) error {
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
func (w *abilityCommandWrapper) publishResult(result *AbilityResult, actor shared.Actor) error {
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
		zoneId, roomId := actor.Location()
		room := w.world.GetZone(zoneId).GetRoom(roomId)
		if err := w.pub.Publish(room, exclude, []byte(strings.Join(result.RoomLines, "\n"))); err != nil {
			return err
		}
	}
	return nil
}

// attackEffect reads the actor's attack grants and performs one attack roll per
// grant. Each hit delegates to dealDamage for damage application and threat.
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
	return func(actor shared.Actor, resolved map[string]*TargetRef, result *AbilityResult) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
		}
		for _, spec := range targets {
			ref := resolved[spec.Name]
			if ref == nil || ref.Actor == nil {
				continue
			}
			target := ref.Actor.Actor()
			if err := e.combat.StartCombat(actor, target); err != nil {
				return NewUserError(err.Error())
			}
			actorName := actor.Name()
			targetName := ref.Actor.Name
			attackArgs := actor.GrantArgs(assets.PerkGrantAttack)
			if len(attackArgs) == 0 {
				attackArgs = []string{"1d4"}
			}
			for _, arg := range attackArgs {
				dmgType, diceExpr := combat.ParseAttackArg(arg)
				attackBonus := assets.ApplyModifiers(0, 0, actor, assets.CombatAttackPrefix)
				roll := combat.RollAttack(attackBonus)
				ac := assets.ApplyModifiers(0, 0, target, assets.CombatACPrefix)
				var damage int
				if roll >= ac {
					dice, err := combat.ParseDice(diceExpr)
					if err != nil {
						dice = combat.DiceRoll{Count: 1, Sides: 4}
					}
					damage = dealDamage(e.combat, actor, target, dice.Roll(), dmgType)
				}
				result.ActorLines = append(result.ActorLines, combat.HitMsgActor(targetName, damage))
				result.TargetLines = append(result.TargetLines, combat.HitMsgTarget(actorName, damage))
				result.RoomLines = append(result.RoomLines, combat.HitMsgRoom(actorName, targetName, damage))
			}
			return nil
		}
		return nil
	}
}

// damageEffect applies damage to the primary target and initiates combat if the target is a mob.
//
// Config fields:
//   - "amount" (string, required): flat integer or dice expression (e.g. "25", "2d6+3").
//   - "damage_types" (comma-separated string, optional): damage type tags (e.g. "fire,ice").
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
	amount := config["amount"]
	if amount == "" {
		return fmt.Errorf("amount config required")
	}
	if _, err := combat.ParseDice(amount); err != nil {
		return fmt.Errorf("amount must be an integer or dice expression: %w", err)
	}
	return nil
}

func (e *damageEffect) Create(_ string, config map[string]string, targets []assets.TargetSpec) EffectFunc {
	dice, _ := combat.ParseDice(config["amount"])

	var damageTypes []string
	if dt := config["damage_types"]; dt != "" {
		damageTypes = strings.Split(dt, ",")
	}
	primaryType := assets.DamageTypeUntyped
	if len(damageTypes) > 0 {
		primaryType = damageTypes[0]
	}

	return func(actor shared.Actor, resolved map[string]*TargetRef, _ *AbilityResult) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
		}

		raw := dice.Roll()

		for _, spec := range targets {
			ref := resolved[spec.Name]
			if ref == nil {
				continue
			}
			dealDamage(e.combat, actor, ref.Actor.Actor(), raw, primaryType)
			return nil
		}

		return nil
	}
}

// dealDamage applies raw damage of the given type to a target, handling CalcDamage,
// reflected damage, combat initiation, and threat. Returns the final damage dealt.
func dealDamage(cm CombatManager, actor shared.Actor, target shared.Actor, raw int, dmgType string) int {
	damage, reflected := combat.CalcDamage(raw, dmgType, actor, target)
	target.AdjustResource(assets.ResourceHp, -damage, false)
	if reflected > 0 {
		actor.AdjustResource(assets.ResourceHp, -reflected, false)
	}
	_ = cm.StartCombat(actor, target)
	cm.AddThreat(actor, target, damage)
	return damage
}

// aoeDamageEffect applies damage to enemies in the caster's room. Players hit
// mobs; mobs hit players. The "hit_allies" config extends targeting to same-side.
//
// Config fields:
//   - "amount" (string, required): flat integer or dice expression before modifiers.
//   - "damage_type" (string, optional): damage type tag. Defaults to untyped.
//   - "hit_allies" ("true"/"false", optional): also damage same-side targets. Default false.
type aoeDamageEffect struct {
	combat CombatManager
	world  ZoneLocator
}

func (e *aoeDamageEffect) Spec() *HandlerSpec { return nil }

func (e *aoeDamageEffect) ValidateConfig(config map[string]string) error {
	amount := config["amount"]
	if amount == "" {
		return fmt.Errorf("amount config required")
	}
	if _, err := combat.ParseDice(amount); err != nil {
		return fmt.Errorf("amount must be an integer or dice expression: %w", err)
	}
	return nil
}

func (e *aoeDamageEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	dice, _ := combat.ParseDice(config["amount"])
	dmgType := config["damage_type"]
	if dmgType == "" {
		dmgType = assets.DamageTypeUntyped
	}
	hitAllies := config["hit_allies"] == "true"

	return func(actor shared.Actor, _ map[string]*TargetRef, _ *AbilityResult) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
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

		actorId := actor.Id()
		isChar := actor.IsCharacter()

		// Hit enemies: players hit mobs, mobs hit players.
		if isChar {
			ri.ForEachMob(func(mi *game.MobileInstance) {
				dealDamage(e.combat, actor, mi, dice.Roll(), dmgType)
			})
		} else {
			ri.ForEachPlayer(func(charId string, ci *game.CharacterInstance) {
				dealDamage(e.combat, actor, ci, dice.Roll(), dmgType)
			})
		}

		// Optionally hit allies (skip self).
		if hitAllies {
			if isChar {
				ri.ForEachPlayer(func(charId string, ci *game.CharacterInstance) {
					if charId == actorId {
						return
					}
					dealDamage(e.combat, actor, ci, dice.Roll(), dmgType)
				})
			} else {
				ri.ForEachMob(func(mi *game.MobileInstance) {
					if mi.Id() == actorId {
						return
					}
					dealDamage(e.combat, actor, mi, dice.Roll(), dmgType)
				})
			}
		}

		return nil
	}
}

// aoeHealEffect heals allies in the caster's room. Players heal players; mobs
// heal mobs. The "hit_enemies" config extends targeting to the opposite side.
//
// Config fields:
//   - "amount" (string, required): flat integer or dice expression.
//   - "overheal" ("true"/"false", optional): allow healing above max HP. Default false.
//   - "hit_enemies" ("true"/"false", optional): also heal opposite-side targets. Default false.
type aoeHealEffect struct {
	combat CombatManager
	world  ZoneLocator
}

func (e *aoeHealEffect) Spec() *HandlerSpec { return nil }

func (e *aoeHealEffect) ValidateConfig(config map[string]string) error {
	amount := config["amount"]
	if amount == "" {
		return fmt.Errorf("amount config required")
	}
	if _, err := combat.ParseDice(amount); err != nil {
		return fmt.Errorf("amount must be an integer or dice expression: %w", err)
	}
	return nil
}

func (e *aoeHealEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	dice, _ := combat.ParseDice(config["amount"])
	overheal := config["overheal"] == "true"
	hitEnemies := config["hit_enemies"] == "true"

	heal := func(actor shared.Actor, target shared.Actor) {
		healAmount := dice.Roll()
		target.AdjustResource(assets.ResourceHp, healAmount, overheal)
		e.combat.NotifyHeal(actor, target, healAmount/2)
	}

	return func(actor shared.Actor, _ map[string]*TargetRef, _ *AbilityResult) error {
		zoneId, roomId := actor.Location()
		zi := e.world.GetZone(zoneId)
		if zi == nil {
			return nil
		}
		ri := zi.GetRoom(roomId)
		if ri == nil {
			return nil
		}

		isChar := actor.IsCharacter()

		// Heal allies: players heal players, mobs heal mobs.
		if isChar {
			ri.ForEachPlayer(func(_ string, ci *game.CharacterInstance) {
				heal(actor, ci)
			})
		} else {
			ri.ForEachMob(func(mi *game.MobileInstance) {
				heal(actor, mi)
			})
		}

		// Optionally heal enemies.
		if hitEnemies {
			if isChar {
				ri.ForEachMob(func(mi *game.MobileInstance) {
					heal(actor, mi)
				})
			} else {
				ri.ForEachPlayer(func(_ string, ci *game.CharacterInstance) {
					heal(actor, ci)
				})
			}
		}

		return nil
	}
}

// healEffect restores HP to the target and generates threat on all combatants
// fighting the target at half the amount healed.
//
// Config fields:
//   - "amount" (string, required): flat integer or dice expression (e.g. "25", "2d6+3").
//   - "overheal" ("true"/"false", optional): allow healing above max HP. Default false.
type healEffect struct {
	combat CombatManager
}

func (e *healEffect) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeActor, Required: true},
		},
	}
}

func (e *healEffect) ValidateConfig(config map[string]string) error {
	amount := config["amount"]
	if amount == "" {
		return fmt.Errorf("amount config required")
	}
	if _, err := combat.ParseDice(amount); err != nil {
		return fmt.Errorf("amount must be an integer or dice expression: %w", err)
	}
	return nil
}

func (e *healEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	dice, _ := combat.ParseDice(config["amount"])
	overheal := config["overheal"] == "true"

	return func(actor shared.Actor, resolved map[string]*TargetRef, _ *AbilityResult) error {
		ref := resolved["target"]
		if ref == nil || ref.Actor == nil {
			return nil
		}
		target := ref.Actor.Actor()
		healAmount := dice.Roll()
		target.AdjustResource(assets.ResourceHp, healAmount, overheal)
		e.combat.NotifyHeal(actor, target, healAmount/2)
		return nil
	}
}

// parsePerkArg parses a grant arg string into a Perk.
// Format: "modifier:key:value" or "grant:key:arg".
func parsePerkArg(arg string) (assets.Perk, error) {
	parts := strings.SplitN(arg, ":", 3)
	if len(parts) < 2 {
		return assets.Perk{}, fmt.Errorf("invalid perk arg %q: expected type:key[:value]", arg)
	}
	p := assets.Perk{Type: parts[0], Key: parts[1]}
	if len(parts) == 3 {
		switch p.Type {
		case assets.PerkTypeModifier:
			v, err := strconv.Atoi(parts[2])
			if err != nil {
				return assets.Perk{}, fmt.Errorf("invalid perk arg %q: bad value: %w", arg, err)
			}
			p.Value = v
		case assets.PerkTypeGrant:
			p.Arg = parts[2]
		}
	}
	return p, nil
}

// resolveGrantPerks reads the actor's grant args for the given key and parses
// each into a Perk. Returns nil if the actor has no grants for the key.
func resolveGrantPerks(actor shared.Actor, grantKey string) ([]assets.Perk, error) {
	args := actor.GrantArgs(grantKey)
	if len(args) == 0 {
		return nil, nil
	}
	perks := make([]assets.Perk, 0, len(args))
	for _, arg := range args {
		p, err := parsePerkArg(arg)
		if err != nil {
			return nil, err
		}
		perks = append(perks, p)
	}
	return perks, nil
}

// buffScope identifies what a buff effect targets.
type buffScope int

const (
	buffScopeActor buffScope = iota
	buffScopeRoom
	buffScopeZone
	buffScopeWorld
)

// buffWorld provides the zone/room lookup and world-level perks needed by buffEffect.
type buffWorld interface {
	ZoneLocator
	Perks() *game.PerkCache
}

// buffEffect applies timed perks to a target determined by scope: a specific
// actor (or self), the caster's room, zone, or the entire world.
type buffEffect struct {
	scope buffScope
	world buffWorld
}

func (e *buffEffect) Spec() *HandlerSpec {
	if e.scope == buffScopeActor {
		return &HandlerSpec{
			Targets: []TargetRequirement{
				{Name: "target", Type: targetTypeMobile | targetTypePlayer, Required: false},
			},
		}
	}
	return nil
}

func (e *buffEffect) ValidateConfig(config map[string]string) error {
	dur, err := strconv.Atoi(config["duration"])
	if err != nil || dur <= 0 {
		return fmt.Errorf("positive duration config required")
	}
	if config["grant_key"] != "" {
		return nil
	}
	if config["perk_type"] == "" {
		return fmt.Errorf("perk_type or grant_key config required")
	}
	if config["perk_key"] == "" {
		return fmt.Errorf("perk_key config required")
	}
	return nil
}

func (e *buffEffect) Create(id string, config map[string]string, targets []assets.TargetSpec) EffectFunc {
	dur, _ := strconv.Atoi(config["duration"])
	name := config["name"]
	if name == "" {
		name = id
	}

	var perks []assets.Perk
	if config["grant_key"] == "" {
		perkValue, _ := strconv.Atoi(config["perk_value"])
		perks = []assets.Perk{{
			Type:  config["perk_type"],
			Key:   config["perk_key"],
			Value: perkValue,
			Arg:   config["perk_arg"],
		}}
	}

	grantKey := config["grant_key"]

	return func(actor shared.Actor, resolved map[string]*TargetRef, _ *AbilityResult) error {
		p := perks
		if grantKey != "" {
			var err error
			if p, err = resolveGrantPerks(actor, grantKey); err != nil {
				return err
			}
		}
		if len(p) == 0 {
			return nil
		}

		switch e.scope {
		case buffScopeActor:
			for _, spec := range targets {
				ref := resolved[spec.Name]
				if ref == nil || ref.Actor == nil {
					continue
				}
				ref.Actor.Actor().AddTimedPerks(name, p, dur)
				return nil
			}
			actor.AddTimedPerks(name, p, dur)
		case buffScopeRoom:
			zoneId, roomId := actor.Location()
			room := e.world.GetZone(zoneId).GetRoom(roomId)
			if room == nil {
				return fmt.Errorf("buff effect: room not found")
			}
			room.Perks.AddTimedPerks(name, p, dur)
		case buffScopeZone:
			zoneId, _ := actor.Location()
			zone := e.world.GetZone(zoneId)
			if zone == nil {
				return fmt.Errorf("buff effect: zone not found")
			}
			zone.Perks.AddTimedPerks(name, p, dur)
		case buffScopeWorld:
			e.world.Perks().AddTimedPerks(name, p, dur)
		}
		return nil
	}
}

// aoeThreatEffect modifies threat tables on all mobs in the caster's room.
//
// Config fields:
//   - "mode" (string, required): one of "add", "set_to_top", "set_to_value".
//   - "amount" (int string): threat delta for "add", absolute value for "set_to_value".
//     Required for "add" and "set_to_value" modes; ignored by "set_to_top".
//   - "in_combat_only" ("true"/"false", optional): only affect mobs already in combat. Default false.
type aoeThreatEffect struct {
	combat CombatManager
	world  ZoneLocator
}

func (e *aoeThreatEffect) Spec() *HandlerSpec { return nil }

func (e *aoeThreatEffect) ValidateConfig(config map[string]string) error {
	if err := validateThreatConfig(config); err != nil {
		return err
	}
	if v := config["in_combat_only"]; v != "" && v != "true" && v != "false" {
		return fmt.Errorf("in_combat_only must be \"true\" or \"false\", got %q", v)
	}
	return nil
}

func (e *aoeThreatEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	mode := config["mode"]
	amount, _ := strconv.Atoi(config["amount"])
	inCombatOnly := config["in_combat_only"] == "true"

	applyThreat := func(actor shared.Actor, target shared.Actor) {
		if inCombatOnly && !target.IsInCombat() {
			return
		}
		if err := e.combat.StartCombat(actor, target); err != nil {
			slog.Warn("aoe_threat: StartCombat failed", "target", target.Id(), "error", err)
			return
		}
		switch mode {
		case ThreatModeAdd:
			e.combat.AddThreat(actor, target, amount)
		case ThreatModeSetToTop:
			e.combat.TopThreat(actor, target)
		case ThreatModeSetToValue:
			e.combat.SetThreat(actor, target, amount)
		}
	}

	return func(actor shared.Actor, _ map[string]*TargetRef, _ *AbilityResult) error {
		if actor.HasGrant(assets.PerkGrantPeaceful, "") {
			return errPeacefulArea
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
			applyThreat(actor, mi)
		})

		return nil
	}
}

// Threat effect mode constants for the "mode" config field.
const (
	ThreatModeAdd        = "add"
	ThreatModeSetToTop   = "set_to_top"
	ThreatModeSetToValue = "set_to_value"
)

// threatEffect modifies threat tables on combat targets.
//
// Config fields:
//   - "mode" (string, required): one of "add", "set_to_top", "set_to_value".
//   - "amount" (int string): threat delta for "add", absolute value for "set_to_value".
//     Required for "add" and "set_to_value" modes; ignored by "set_to_top".
type threatEffect struct {
	combat CombatManager
}

func (e *threatEffect) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeMobile | targetTypePlayer, Required: true},
		},
	}
}

func (e *threatEffect) ValidateConfig(config map[string]string) error {
	return validateThreatConfig(config)
}

// validateThreatConfig checks the mode/amount fields shared by threat effects.
func validateThreatConfig(config map[string]string) error {
	mode := config["mode"]
	switch mode {
	case ThreatModeAdd, ThreatModeSetToValue:
		if config["amount"] == "" {
			return fmt.Errorf("amount config required for mode %q", mode)
		}
		if _, err := strconv.Atoi(config["amount"]); err != nil {
			return fmt.Errorf("amount must be an integer: %w", err)
		}
	case ThreatModeSetToTop:
		// No amount needed.
	default:
		return fmt.Errorf("unknown threat mode %q", mode)
	}
	return nil
}

func (e *threatEffect) Create(_ string, config map[string]string, targets []assets.TargetSpec) EffectFunc {
	mode := config["mode"]
	amount, _ := strconv.Atoi(config["amount"])

	return func(actor shared.Actor, resolved map[string]*TargetRef, _ *AbilityResult) error {
		ref := resolved["target"]
		if ref == nil || ref.Actor == nil {
			return nil
		}
		target := ref.Actor.Actor()

		// Ensure the caster is in combat with the target.
		if err := e.combat.StartCombat(actor, target); err != nil {
			return NewUserError(err.Error())
		}

		switch mode {
		case ThreatModeAdd:
			e.combat.AddThreat(actor, target, amount)
		case ThreatModeSetToTop:
			e.combat.TopThreat(actor, target)
		case ThreatModeSetToValue:
			e.combat.SetThreat(actor, target, amount)
		}

		return nil
	}
}

// Spawn destination constants for spawnObjEffect.
const (
	SpawnDestRoom      = "room"
	SpawnDestInventory = "inventory"
)

// spawnObjEffect spawns an object into the caster's room or inventory.
//
// Config fields:
//   - "object_id" (string, required): the asset ID of the object to spawn.
//   - "destination" (string, optional): "room" (default) or "inventory".
type spawnObjEffect struct {
	objects storage.Storer[*assets.Object]
	world   ZoneLocator
}

func (e *spawnObjEffect) Spec() *HandlerSpec { return nil }

func (e *spawnObjEffect) ValidateConfig(config map[string]string) error {
	objId := config["object_id"]
	if objId == "" {
		return fmt.Errorf("object_id config required")
	}
	if e.objects.Get(objId) == nil {
		return fmt.Errorf("object %q not found", objId)
	}
	dest := config["destination"]
	if dest != "" && dest != SpawnDestRoom && dest != SpawnDestInventory {
		return fmt.Errorf("destination must be %q or %q", SpawnDestRoom, SpawnDestInventory)
	}
	return nil
}

func (e *spawnObjEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	objId := config["object_id"]
	dest := config["destination"]
	if dest == "" {
		dest = SpawnDestRoom
	}

	return func(actor shared.Actor, _ map[string]*TargetRef, _ *AbilityResult) error {
		si := storage.NewSmartIdentifier[*assets.Object](objId)
		if err := si.Resolve(e.objects); err != nil {
			return fmt.Errorf("spawn_obj: %w", err)
		}
		oi, err := game.NewObjectInstance(si)
		if err != nil {
			return fmt.Errorf("spawn_obj: %w", err)
		}
		oi.ActivateDecay()

		switch dest {
		case SpawnDestInventory:
			actor.GetInventory().AddObj(oi)
		case SpawnDestRoom:
			zoneId, roomId := actor.Location()
			zi := e.world.GetZone(zoneId)
			if zi == nil {
				return nil
			}
			ri := zi.GetRoom(roomId)
			if ri == nil {
				return nil
			}
			ri.AddObj(oi)
		}
		return nil
	}
}
