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
	// Check and deduct resource cost
	if ability.ResourceCost > 0 {
		cur, _ := in.Char.Resource(ability.Resource)
		if cur < ability.ResourceCost {
			return NewUserError(fmt.Sprintf("You don't have enough %s.", ability.Resource))
		}
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

	charId := in.Char.Character.Id()

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
		room := world.GetRoom(zoneId, roomId)
		if err := pub.Publish(room, exclude, []byte(msg)); err != nil {
			return err
		}
	}

	return nil
}

// damageEffect applies damage to the primary target.
//
// Config fields:
//   - "base_damage" (number, required): flat damage before modifiers.
//   - "damage_types" ([]string, optional): damage type tags (e.g. ["fire"]).
//     The caster's core.damage.<type>.pct modifiers are summed and applied as
//     a percentage bonus. core.damage.<type>.crit_pct modifiers give a percent
//     chance to double damage.
//
// The caster's core.combat.damage_mod is always added as flat bonus damage.
type damageEffect struct{}

func (e *damageEffect) Execute(ability *assets.Ability, in *CommandInput, targets map[string]*TargetRef) error {
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

// roomBuffEffect applies timed perks to the caster's current room.
//
// Config fields:
//   - "perks" ([]Perk, required): perks to apply to the room.
//   - "duration" (number, required): number of ticks the perks last.
//   - "name" (string, optional): entry name for the timed perk. Defaults to the ability name.
type roomBuffEffect struct {
	world RoomLocator
}

func (e *roomBuffEffect) Execute(ability *assets.Ability, in *CommandInput, _ map[string]*TargetRef) error {
	dur, ok := ability.Config["duration"].(float64)
	if !ok || dur <= 0 {
		return fmt.Errorf("room_buff effect: positive duration config required")
	}

	perksRaw, ok := ability.Config["perks"]
	if !ok {
		return fmt.Errorf("room_buff effect: perks config required")
	}
	raw, err := json.Marshal(perksRaw)
	if err != nil {
		return fmt.Errorf("room_buff effect: marshaling perks: %w", err)
	}
	var perks []assets.Perk
	if err := json.Unmarshal(raw, &perks); err != nil {
		return fmt.Errorf("room_buff effect: parsing perks: %w", err)
	}

	name, _ := ability.Config["name"].(string)
	if name == "" {
		name = ability.Name
	}

	zoneId, roomId := in.Char.Location()
	room := e.world.GetRoom(zoneId, roomId)
	if room == nil {
		return fmt.Errorf("room_buff effect: room not found")
	}
	room.Perks.AddPerks(name, perks, int(dur))
	return nil
}
