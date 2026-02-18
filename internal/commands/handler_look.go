package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// LookHandlerFactory creates handlers that display the current room.
type LookHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

// NewLookHandlerFactory creates a new LookHandlerFactory with access to world state.
func NewLookHandlerFactory(world *game.WorldState, pub Publisher) *LookHandlerFactory {
	return &LookHandlerFactory{world: world, pub: pub}
}

func (f *LookHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypePlayer | TargetTypeMobile | TargetTypeObject, Required: false},
		},
	}
}

func (f *LookHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *LookHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		if cmdCtx.Session == nil {
			return fmt.Errorf("player state not found")
		}

		// Check if target was resolved (from targets section)
		if target := cmdCtx.Targets["target"]; target != nil {
			return f.showTarget(cmdCtx, target)
		}

		return f.showRoom(cmdCtx)
	}, nil
}

// showRoom displays the current room description.
func (f *LookHandlerFactory) showRoom(cmdCtx *CommandContext) error {
	zoneId, roomId := cmdCtx.Session.Location()

	ri := f.world.Instances()[zoneId].GetRoom(roomId)
	if ri == nil {
		return NewUserError("You are in an invalid location.")
	}

	roomDesc := ri.Describe(cmdCtx.Actor.Name)
	if f.pub != nil {
		return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte(roomDesc))
	}

	return nil
}

// showTarget displays information about a specific target.
func (f *LookHandlerFactory) showTarget(cmdCtx *CommandContext, target *TargetRef) error {
	var msg string
	switch target.Type {
	case TargetTypePlayer:
		msg = f.describePlayer(target.Player)
	case TargetTypeMobile:
		msg = f.describeMob(target.Mob)
	case TargetTypeObject:
		msg = f.describeObj(target.Obj)
	default:
		return NewUserError("You can't look at that.")
	}

	if f.pub != nil {
		return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte(msg))
	}
	return nil
}

// TODO: should these functions just be moved to the respective target's instance structs?
func (f *LookHandlerFactory) describePlayer(player *PlayerRef) string {
	return player.Description
}

func (f *LookHandlerFactory) describeMob(mob *MobileRef) string {
	return mob.Description
}

func (f *LookHandlerFactory) describeObj(obj *ObjectRef) string {
	if obj.instance == nil || obj.instance.Definition == nil || !obj.instance.Definition.HasFlag(game.ObjectFlagContainer) {
		return obj.Description
	}

	lines := []string{obj.Description, "It contains:"}
	lines = append(lines, FormatInventoryItems(obj.instance.Contents)...)
	return strings.Join(lines, "\n")
}
