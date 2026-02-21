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
	pub    Publisher
}

func NewKillHandlerFactory(combat *combat.Manager, pub Publisher) *KillHandlerFactory {
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
		attacker := &game.PlayerCombatant{
			CharId:    cmdCtx.Session.CharId,
			Player:    cmdCtx.Session,
			Character: cmdCtx.Actor,
		}
		defender := &game.MobCombatant{Instance: mi}

		zoneID, roomID := cmdCtx.Session.Location()
		err := f.combat.StartCombat(attacker, defender, string(zoneID), string(roomID))
		if err != nil {
			return NewUserError(err.Error())
		}

		msg := fmt.Sprintf("%s attacks %s!", cmdCtx.Actor.Name, mi.Mobile.Get().ShortDesc)
		_ = f.pub.PublishToRoom(zoneID, roomID, []byte(msg))

		return nil
	}, nil
}
