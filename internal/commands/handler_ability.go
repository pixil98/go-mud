package commands

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
)

// EffectHandler executes an ability's gameplay effect (damage, healing, etc.).
type EffectHandler interface {
	Execute(ability *assets.Ability, in *CommandInput, targets map[string]*TargetRef) error
}

// executeAbility runs an ability's effect handler and sends its messages.
// If effect is nil, only messages are sent.
func executeAbility(ability *assets.Ability, in *CommandInput, targets map[string]*TargetRef, world WorldView, pub game.Publisher, effect EffectHandler) error {
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

// damageEffect applies flat damage to the primary target.
// Reads "base_damage" from the ability's Config (numeric value).
type damageEffect struct{}

func (e *damageEffect) Execute(ability *assets.Ability, in *CommandInput, targets map[string]*TargetRef) error {
	baseDamage, ok := ability.Config["base_damage"].(float64)
	if !ok {
		return fmt.Errorf("damage effect: base_damage config required")
	}
	damage := int(baseDamage)

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

// applyDamage reduces a target's HP, clamping at zero.
func applyDamage(ref *TargetRef, amount int) {
	if ref.Player != nil {
		ref.Player.session.CurrentHP -= amount
		if ref.Player.session.CurrentHP < 0 {
			ref.Player.session.CurrentHP = 0
		}
	} else if ref.Mob != nil {
		ref.Mob.instance.CurrentHP -= amount
		if ref.Mob.instance.CurrentHP < 0 {
			ref.Mob.instance.CurrentHP = 0
		}
	}
}
