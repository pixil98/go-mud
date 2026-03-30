package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
)

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
