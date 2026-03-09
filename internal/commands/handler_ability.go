package commands

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
)

// randIntn wraps rand.IntN for testability.
var randIntn = rand.IntN

// EffectHandler executes an ability's gameplay effect (damage, healing, etc.).
type EffectHandler interface {
	Execute(ability *assets.Ability, in *CommandInput, targets map[string]*TargetRef) error
}

// executeAbility runs an ability's effect handler and sends its messages.
// If effect is nil, only messages are sent.
func executeAbility(ability *assets.Ability, in *CommandInput, targets map[string]*TargetRef, world WorldView, pub game.Publisher, effect EffectHandler) error {
	// Check resource cost before spending any AP
	if ability.ResourceCost > 0 {
		cur, _ := in.Char.Resource(ability.Resource)
		if cur < ability.ResourceCost {
			return NewUserError(fmt.Sprintf("You don't have enough %s.", ability.Resource))
		}
	}

	// Check and spend action points
	apCost := ability.APCost
	if apCost == 0 {
		apCost = 1
	}
	if !in.Char.SpendAP(apCost) {
		return NewUserError("You're not ready to do that yet.")
	}

	// Deduct resource cost
	if ability.ResourceCost > 0 {
		in.Char.AdjustResource(ability.Resource, -ability.ResourceCost)
	}

	// Run effect first — if it fails, don't send messages
	if effect != nil {
		if err := effect.Execute(ability, in, targets); err != nil {
			return err
		}
	}

	if pub == nil {
		return nil
	}

	ctx := &templateContext{
		Actor:   in.Char.Character.Get(),
		Targets: targets,
		Color:   display.Color,
	}

	charId := in.Char.Id()

	// Actor message
	if ability.Messages.Actor != "" {
		msg, err := ExpandTemplate(ability.Messages.Actor, ctx)
		if err != nil {
			return fmt.Errorf("expanding actor message: %w", err)
		}
		if err := pub.Publish(game.SinglePlayer(charId), nil, []byte(msg)); err != nil {
			return err
		}
	}

	// Target message (send to first resolved player target)
	if ability.Messages.Target != "" {
		for _, spec := range ability.Command.Targets {
			ref := targets[spec.Name]
			if ref != nil && ref.Player != nil {
				msg, err := ExpandTemplate(ability.Messages.Target, ctx)
				if err != nil {
					return fmt.Errorf("expanding target message: %w", err)
				}
				if err := pub.Publish(game.SinglePlayer(ref.Player.CharId), nil, []byte(msg)); err != nil {
					return err
				}
				break
			}
		}
	}

	// Room message
	if ability.Messages.Room != "" {
		msg, err := ExpandTemplate(ability.Messages.Room, ctx)
		if err != nil {
			return fmt.Errorf("expanding room message: %w", err)
		}
		exclude := []string{charId}
		for _, ref := range targets {
			if ref != nil && ref.Player != nil {
				exclude = append(exclude, ref.Player.CharId)
			}
		}
		zoneId, roomId := in.Char.Location()
		room := world.GetZone(zoneId).GetRoom(roomId)
		if err := pub.Publish(room, exclude, []byte(msg)); err != nil {
			return err
		}
	}

	return nil
}

// attackEffect queues a manual attack for the next combat tick.
// It calls StartCombat (idempotent) then QueueAttack so the tick resolves
// the full attack sequence bundled with all other combat activity.
type attackEffect struct {
	combat CombatManager
}

func (e *attackEffect) Execute(ability *assets.Ability, in *CommandInput, targets map[string]*TargetRef) error {
	if in.Char.HasGrant(assets.PerkGrantPeaceful, "") {
		return errPeacefulArea
	}
	for _, spec := range ability.Command.Targets {
		ref := targets[spec.Name]
		if ref == nil || ref.Mob == nil {
			continue
		}
		if err := e.combat.StartCombat(in.Char, ref.Mob.instance); err != nil {
			return NewUserError(err.Error())
		}
		e.combat.QueueAttack(in.Char)
		return nil
	}
	return nil
}

// damageEffect applies damage to the primary target and initiates combat if the target is a mob.
//
// Config fields:
//   - "base_damage" (number, required): flat damage before modifiers.
//   - "damage_types" ([]string, optional): damage type tags (e.g. ["fire"]).
//     The caster's core.damage.<type>.pct modifiers are summed and applied as
//     a percentage bonus. core.damage.<type>.crit_pct modifiers give a percent
//     chance to double damage.
//
// The caster's core.combat.damage_mod is always added as flat bonus damage.
type damageEffect struct {
	combat CombatManager
}

func (e *damageEffect) Execute(ability *assets.Ability, in *CommandInput, targets map[string]*TargetRef) error {
	if in.Char.HasGrant(assets.PerkGrantPeaceful, "") {
		return errPeacefulArea
	}
	baseDamage, ok := ability.Config["base_damage"].(float64)
	if !ok {
		return fmt.Errorf("damage effect: base_damage config required")
	}
	damage := int(baseDamage)

	// Add flat damage_mod from caster perks
	damage += in.Char.ModifierValue(assets.PerkKeyCombatDmgMod)

	// Apply damage type percentage bonuses and crit
	if arr, ok := ability.Config["damage_types"].([]any); ok && len(arr) > 0 {
		pctBonus := 0
		critPct := 0
		for _, v := range arr {
			dt, ok := v.(string)
			if !ok {
				continue
			}
			pctBonus += in.Char.ModifierValue(assets.DamageKey(dt, assets.DamageAspectPct))
			critPct += in.Char.ModifierValue(assets.DamageKey(dt, assets.DamageAspectCritPct))
		}
		if pctBonus != 0 {
			damage = damage * (100 + pctBonus) / 100
		}
		if critPct > 0 && randIntn(100) < critPct {
			damage *= 2
		}
	}

	if damage < 1 {
		damage = 1
	}

	// Apply to the first resolved target
	for _, spec := range ability.Command.Targets {
		ref := targets[spec.Name]
		if ref == nil {
			continue
		}
		applyDamage(ref, damage)
		if ref.Mob != nil {
			_ = e.combat.StartCombat(in.Char, ref.Mob.instance)
			e.combat.AddThreat(in.Char, ref.Mob.instance, damage)
		}
		return nil
	}

	return nil
}

// applyDamage reduces a target's HP by amount.
func applyDamage(ref *TargetRef, amount int) {
	if ref.Player != nil {
		ref.Player.session.AdjustResource(assets.ResourceHp, -amount)
	} else if ref.Mob != nil {
		ref.Mob.instance.AdjustResource(assets.ResourceHp, -amount)
	}
}

// parsePerkBuffConfig extracts the common config fields for perk buff effects.
//
// Config fields:
//   - "perks" ([]Perk, required): perks to apply.
//   - "duration" (number, required): number of ticks the perks last.
//   - "name" (string, optional): entry name for the timed perk. Defaults to the ability name.
func parsePerkBuffConfig(handler string, ability *assets.Ability) (string, []assets.Perk, int, error) {
	dur, ok := ability.Config["duration"].(float64)
	if !ok || dur <= 0 {
		return "", nil, 0, fmt.Errorf("%s effect: positive duration config required", handler)
	}

	perksRaw, ok := ability.Config["perks"]
	if !ok {
		return "", nil, 0, fmt.Errorf("%s effect: perks config required", handler)
	}
	raw, err := json.Marshal(perksRaw)
	if err != nil {
		return "", nil, 0, fmt.Errorf("%s effect: marshaling perks: %w", handler, err)
	}
	var perks []assets.Perk
	if err := json.Unmarshal(raw, &perks); err != nil {
		return "", nil, 0, fmt.Errorf("%s effect: parsing perks: %w", handler, err)
	}

	name, _ := ability.Config["name"].(string)
	if name == "" {
		name = ability.Name
	}

	return name, perks, int(dur), nil
}

// actorBuffEffect applies timed perks to a target player/mob, or self if no target.
type actorBuffEffect struct{}

func (e *actorBuffEffect) Execute(ability *assets.Ability, in *CommandInput, targets map[string]*TargetRef) error {
	name, perks, dur, err := parsePerkBuffConfig("actor_buff", ability)
	if err != nil {
		return err
	}

	// Apply to the first resolved player or mob target.
	for _, spec := range ability.Command.Targets {
		ref := targets[spec.Name]
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

	// No target resolved — apply to self.
	in.Char.AddTimedPerks(name, perks, dur)
	return nil
}

// roomBuffEffect applies timed perks to the caster's current room.
type roomBuffEffect struct {
	world ZoneLocator
}

func (e *roomBuffEffect) Execute(ability *assets.Ability, in *CommandInput, _ map[string]*TargetRef) error {
	name, perks, dur, err := parsePerkBuffConfig("room_buff", ability)
	if err != nil {
		return err
	}

	zoneId, roomId := in.Char.Location()
	room := e.world.GetZone(zoneId).GetRoom(roomId)
	if room == nil {
		return fmt.Errorf("room_buff effect: room not found")
	}
	room.Perks.AddTimedPerks(name, perks, dur)
	return nil
}

// zoneBuffEffect applies timed perks to the caster's current zone.
type zoneBuffEffect struct {
	world WorldView
}

func (e *zoneBuffEffect) Execute(ability *assets.Ability, in *CommandInput, _ map[string]*TargetRef) error {
	name, perks, dur, err := parsePerkBuffConfig("zone_buff", ability)
	if err != nil {
		return err
	}

	zoneId, _ := in.Char.Location()
	zone := e.world.GetZone(zoneId)
	if zone == nil {
		return fmt.Errorf("zone_buff effect: zone not found")
	}
	zone.Perks.AddTimedPerks(name, perks, dur)
	return nil
}

// worldBuffEffect applies timed perks to the entire world.
type worldBuffEffect struct {
	world *game.WorldState
}

func (e *worldBuffEffect) Execute(ability *assets.Ability, _ *CommandInput, _ map[string]*TargetRef) error {
	name, perks, dur, err := parsePerkBuffConfig("world_buff", ability)
	if err != nil {
		return err
	}

	e.world.Perks.AddTimedPerks(name, perks, dur)
	return nil
}
