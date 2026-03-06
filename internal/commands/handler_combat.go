package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/game"
)

// CombatManager provides combat operations needed by command handlers.
type CombatManager interface {
	StartCombat(attacker, target combat.Combatant) error
}

// KillHandlerFactory creates handlers for the kill command.
type KillHandlerFactory struct {
	combat CombatManager
	zones  ZoneLocator
	pub    game.Publisher
}

func NewKillHandlerFactory(combat CombatManager, zones ZoneLocator, pub game.Publisher) *KillHandlerFactory {
	return &KillHandlerFactory{combat: combat, zones: zones, pub: pub}
}

func (f *KillHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeMobile, Required: true},
		},
	}
}

func (f *KillHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *KillHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, in *CommandInput) error {
		if in.Char.IsInCombat() {
			return NewUserError("You're already fighting!")
		}

		mi := in.Targets["target"].Mob.instance

		if err := f.combat.StartCombat(in.Char, mi); err != nil {
			return NewUserError(err.Error())
		}

		if err := f.pub.Publish(game.SinglePlayer(in.Char.Id()), nil,
			[]byte(fmt.Sprintf("You attack %s!", mi.Name()))); err != nil {
			slog.Warn("failed to notify attacker", "error", err)
		}

		zoneID, roomID := in.Char.Location()
		room := f.zones.GetZone(zoneID).GetRoom(roomID)
		roomMsg := fmt.Sprintf("%s attacks %s!", in.Char.Name(), mi.Name())
		if err := f.pub.Publish(room, []string{in.Char.Id()}, []byte(roomMsg)); err != nil {
			slog.Warn("failed to publish room attack message", "error", err)
		}

		return nil
	}, nil
}

// AssistHandlerFactory creates handlers for the assist command.
// When a target player is resolved, the actor joins that player's fight.
// When no target is given, the actor assists their follow leader.
type AssistHandlerFactory struct {
	combat  CombatManager
	zones   ZoneLocator
	players PlayerLookup
	pub     game.Publisher
}

func NewAssistHandlerFactory(combat CombatManager, zones ZoneLocator, players PlayerLookup, pub game.Publisher) *AssistHandlerFactory {
	return &AssistHandlerFactory{combat: combat, zones: zones, players: players, pub: pub}
}

func (f *AssistHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer, Required: false},
		},
	}
}

func (f *AssistHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *AssistHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, in *CommandInput) error {
		if in.Char.IsInCombat() {
			return NewUserError("You're already fighting!")
		}

		assistedId, assistedName, err := f.resolveAssisted(in)
		if err != nil {
			return err
		}

		assistedCI := f.players.GetPlayer(assistedId)
		if assistedCI == nil {
			return NewUserError(fmt.Sprintf("%s isn't here.", assistedName))
		}

		targetMobId := assistedCI.GetCombatTargetId()
		if targetMobId == "" {
			return NewUserError(fmt.Sprintf("%s isn't fighting anyone.", assistedName))
		}

		assistedZone, assistedRoom := assistedCI.Location()
		targetMob := f.zones.GetZone(assistedZone).GetRoom(assistedRoom).GetMob(targetMobId)
		if err := f.combat.StartCombat(in.Char, targetMob); err != nil {
			return NewUserError(fmt.Sprintf("%s isn't fighting anything you can assist with.", assistedName))
		}

		actorId := in.Char.Id()

		if err := f.pub.Publish(game.SinglePlayer(actorId), nil,
			[]byte(fmt.Sprintf("You jump to %s's aid!", assistedName))); err != nil {
			slog.Warn("failed to notify actor of assist", "error", err)
		}
		if err := f.pub.Publish(game.SinglePlayer(assistedId), nil,
			[]byte(fmt.Sprintf("%s jumps to your aid!", in.Char.Name()))); err != nil {
			slog.Warn("failed to notify assisted player", "error", err)
		}

		zoneID, roomID := in.Char.Location()
		room := f.zones.GetZone(zoneID).GetRoom(roomID)
		roomMsg := fmt.Sprintf("%s jumps to %s's aid!", in.Char.Name(), assistedName)
		if err := f.pub.Publish(room, []string{actorId, assistedId}, []byte(roomMsg)); err != nil {
			slog.Warn("failed to publish room assist message", "error", err)
		}

		return nil
	}, nil
}

// resolveAssisted determines who the actor wants to assist.
// Returns the assisted player's charId and display name.
func (f *AssistHandlerFactory) resolveAssisted(in *CommandInput) (string, string, error) {
	if target := in.Targets["target"]; target != nil {
		return target.Player.CharId, target.Player.Name, nil
	}

	// Fall back to follow leader.
	leaderId := in.Char.GetFollowingId()
	leader := f.players.GetPlayer(leaderId)
	if leader == nil {
		return "", "", NewUserError("Assist whom?")
	}

	return leaderId, leader.Name(), nil
}
