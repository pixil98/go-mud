package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/game"
)

// KillHandlerFactory creates handlers for the kill command.
type KillHandlerFactory struct {
	combat *combat.Manager
	pub    game.Publisher
}

func NewKillHandlerFactory(combat *combat.Manager, pub game.Publisher) *KillHandlerFactory {
	return &KillHandlerFactory{combat: combat, pub: pub}
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

		_ = f.pub.Publish(game.SinglePlayer(cmdCtx.Session.Character.Id()), nil,
			[]byte(fmt.Sprintf("You attack %s!", mi.Mobile.Get().ShortDesc)))

		room := cmdCtx.World.Instances()[zoneID].GetRoom(roomID)
		roomMsg := fmt.Sprintf("%s attacks %s!", cmdCtx.Actor.Name, mi.Mobile.Get().ShortDesc)
		_ = f.pub.Publish(room, []string{cmdCtx.Session.Character.Id()}, []byte(roomMsg))

		return nil
	}, nil
}
