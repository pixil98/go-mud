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
	StartCombat(attacker, target combat.Combatant, zoneID, roomID string) error
	GetPlayerFighter(charId string) *combat.Fighter
}

// KillHandlerFactory creates handlers for the kill command.
type KillHandlerFactory struct {
	combat CombatManager
	rooms  RoomLocator
	pub    game.Publisher
}

func NewKillHandlerFactory(combat CombatManager, rooms RoomLocator, pub game.Publisher) *KillHandlerFactory {
	return &KillHandlerFactory{combat: combat, rooms: rooms, pub: pub}
}

func (f *KillHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypeMobile, Required: true},
		},
	}
}

func (f *KillHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *KillHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		if cmdCtx.Session.InCombat {
			return NewUserError("You're already fighting!")
		}

		target := cmdCtx.Targets["target"]

		mi := target.Mob.instance
		attacker := &combat.PlayerCombatant{
			Character: cmdCtx.Session.Character,
			Player:    cmdCtx.Session,
		}
		defender := &combat.MobCombatant{Instance: mi}

		zoneID, roomID := cmdCtx.Session.Location()
		err := f.combat.StartCombat(attacker, defender, zoneID, roomID)
		if err != nil {
			return NewUserError(err.Error())
		}

		if err := f.pub.Publish(game.SinglePlayer(cmdCtx.Session.Character.Id()), nil,
			[]byte(fmt.Sprintf("You attack %s!", mi.Mobile.Get().ShortDesc))); err != nil {
			slog.Warn("failed to notify attacker", "error", err)
		}

		room := f.rooms.GetRoom(zoneID, roomID)
		roomMsg := fmt.Sprintf("%s attacks %s!", cmdCtx.Actor.Name, mi.Mobile.Get().ShortDesc)
		if err := f.pub.Publish(room, []string{cmdCtx.Session.Character.Id()}, []byte(roomMsg)); err != nil {
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
	rooms   RoomLocator
	players PlayerLookup
	pub     game.Publisher
}

func NewAssistHandlerFactory(combat CombatManager, rooms RoomLocator, players PlayerLookup, pub game.Publisher) *AssistHandlerFactory {
	return &AssistHandlerFactory{combat: combat, rooms: rooms, players: players, pub: pub}
}

func (f *AssistHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypePlayer, Required: false},
		},
	}
}

func (f *AssistHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *AssistHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		if cmdCtx.Session.InCombat {
			return NewUserError("You're already fighting!")
		}

		// Resolve who we're assisting: explicit target or follow leader.
		assistedId, assistedName, err := f.resolveAssisted(cmdCtx)
		if err != nil {
			return err
		}

		// Look up the assisted player's current fight.
		fighter := f.combat.GetPlayerFighter(assistedId)
		if fighter == nil {
			return NewUserError(fmt.Sprintf("%s isn't fighting anyone.", assistedName))
		}

		attacker := &combat.PlayerCombatant{
			Character: cmdCtx.Session.Character,
			Player:    cmdCtx.Session,
		}

		zoneID, roomID := cmdCtx.Session.Location()
		if err := f.combat.StartCombat(attacker, fighter.Target, zoneID, roomID); err != nil {
			return NewUserError(err.Error())
		}

		actorId := cmdCtx.Session.Character.Id()

		if err := f.pub.Publish(game.SinglePlayer(actorId), nil,
			[]byte(fmt.Sprintf("You jump to %s's aid!", assistedName))); err != nil {
			slog.Warn("failed to notify actor of assist", "error", err)
		}
		if err := f.pub.Publish(game.SinglePlayer(assistedId), nil,
			[]byte(fmt.Sprintf("%s jumps to your aid!", cmdCtx.Actor.Name))); err != nil {
			slog.Warn("failed to notify assisted player", "error", err)
		}

		room := f.rooms.GetRoom(zoneID, roomID)
		roomMsg := fmt.Sprintf("%s jumps to %s's aid!", cmdCtx.Actor.Name, assistedName)
		if err := f.pub.Publish(room, []string{actorId, assistedId}, []byte(roomMsg)); err != nil {
			slog.Warn("failed to publish room assist message", "error", err)
		}

		return nil
	}, nil
}

// resolveAssisted determines who the actor wants to assist.
// Returns the assisted player's charId and display name.
func (f *AssistHandlerFactory) resolveAssisted(cmdCtx *CommandContext) (string, string, error) {
	if target := cmdCtx.Targets["target"]; target != nil {
		return target.Player.CharId, target.Player.Name, nil
	}

	// Fall back to follow leader.
	leaderId := cmdCtx.Session.FollowingId
	leader := f.players.GetPlayer(leaderId)
	if leader == nil {
		return "", "", NewUserError("Assist whom?")
	}

	return leaderId, leader.Character.Get().Name, nil
}
