package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// ClosureHandlerFactory creates handlers for open/close/lock/unlock commands.
// Config:
//   - action (required): "open", "close", "lock", or "unlock"
//
// Targets:
//   - target (required): an exit or container object resolved by the command system
type ClosureHandlerFactory struct {
	world *game.WorldState
	pub   game.Publisher
}

func NewClosureHandlerFactory(world *game.WorldState, pub game.Publisher) *ClosureHandlerFactory {
	return &ClosureHandlerFactory{world: world, pub: pub}
}

func (f *ClosureHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypeObject | TargetTypeExit, Required: true},
		},
		Config: []ConfigRequirement{
			{Name: "action", Required: true},
		},
	}
}

func (f *ClosureHandlerFactory) ValidateConfig(config map[string]any) error {
	action, _ := config["action"].(string)
	switch action {
	case "open", "close", "lock", "unlock":
		return nil
	default:
		return fmt.Errorf("action must be open, close, lock, or unlock")
	}
}

func (f *ClosureHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		action := cmdCtx.Config["action"]
		target := cmdCtx.Targets["target"]

		switch target.Type {
		case TargetTypeExit:
			closure := target.Exit.exit.Closure
			if closure == nil {
				return NewUserError(fmt.Sprintf("You can't %s that.", action))
			}
			zoneId, roomId := cmdCtx.Session.Location()
			room := f.world.Instances()[zoneId].GetRoom(roomId)
			return f.handleExit(action, target.Exit.Direction, closure, room, cmdCtx)

		case TargetTypeObject:
			oi := target.Obj.instance
			if !oi.Object.Get().HasFlag(game.ObjectFlagContainer) || oi.Object.Get().Closure == nil {
				return NewUserError(fmt.Sprintf("You can't %s that.", action))
			}
			return f.handleContainer(action, oi, cmdCtx)

		default:
			return NewUserError(fmt.Sprintf("You can't %s that.", action))
		}
	}, nil
}

func (f *ClosureHandlerFactory) handleExit(action, direction string, closure *game.Closure, room *game.RoomInstance, cmdCtx *CommandContext) error {
	name := closure.Name

	switch action {
	case "open":
		if room.IsExitLocked(direction) {
			return NewUserError(fmt.Sprintf("The %s is locked.", name))
		}
		if !room.IsExitClosed(direction) {
			return NewUserError(fmt.Sprintf("The %s is already open.", name))
		}

	case "close":
		if room.IsExitClosed(direction) {
			return NewUserError(fmt.Sprintf("The %s is already closed.", name))
		}

	case "lock":
		if !room.IsExitClosed(direction) {
			return NewUserError(fmt.Sprintf("You need to close the %s first.", name))
		}
		if room.IsExitLocked(direction) {
			return NewUserError(fmt.Sprintf("The %s is already locked.", name))
		}
		if closure.Lock == nil {
			return NewUserError(fmt.Sprintf("The %s has no lock.", name))
		}
		if err := f.checkKey(cmdCtx, closure.Lock); err != nil {
			return err
		}

	case "unlock":
		if !room.IsExitLocked(direction) {
			return NewUserError(fmt.Sprintf("The %s is not locked.", name))
		}
		if closure.Lock == nil {
			return NewUserError(fmt.Sprintf("The %s has no lock.", name))
		}
		if err := f.checkKey(cmdCtx, closure.Lock); err != nil {
			return err
		}
	}

	// Apply state change to this exit and the other side of the door
	exit := room.Room.Get().Exits[direction]
	applyExitAction(action, room, direction)
	zoneId, roomId := cmdCtx.Session.Location()
	if otherRoom, otherDir := game.FindOtherSide(exit, zoneId, roomId, f.world.Instances()); otherRoom != nil {
		applyExitAction(action, otherRoom, otherDir)
	}

	return f.publish(cmdCtx, fmt.Sprintf("You %s the %s.", action, name), fmt.Sprintf("%s %ss the %s.", cmdCtx.Actor.Name, action, name))
}

// applyExitAction applies a closure state change to a single exit.
func applyExitAction(action string, room *game.RoomInstance, direction string) {
	switch action {
	case "open":
		room.SetExitClosed(direction, false)
	case "close":
		room.SetExitClosed(direction, true)
	case "lock":
		room.SetExitLocked(direction, true)
	case "unlock":
		room.SetExitLocked(direction, false)
	}
}

func (f *ClosureHandlerFactory) handleContainer(action string, oi *game.ObjectInstance, cmdCtx *CommandContext) error {
	closure := oi.Object.Get().Closure
	name := closure.Name
	if name == "" {
		name = oi.Object.Get().ShortDesc
	}
	capName := strings.ToUpper(name[:1]) + name[1:]

	switch action {
	case "open":
		if oi.Locked {
			return NewUserError(fmt.Sprintf("%s is locked.", capName))
		}
		if !oi.Closed {
			return NewUserError(fmt.Sprintf("%s is already open.", capName))
		}
		oi.Closed = false
		return f.publish(cmdCtx, fmt.Sprintf("You open %s.", name), fmt.Sprintf("%s opens %s.", cmdCtx.Actor.Name, name))

	case "close":
		if oi.Closed {
			return NewUserError(fmt.Sprintf("%s is already closed.", capName))
		}
		oi.Closed = true
		return f.publish(cmdCtx, fmt.Sprintf("You close %s.", name), fmt.Sprintf("%s closes %s.", cmdCtx.Actor.Name, name))

	case "lock":
		if !oi.Closed {
			return NewUserError(fmt.Sprintf("You need to close %s first.", name))
		}
		if oi.Locked {
			return NewUserError(fmt.Sprintf("%s is already locked.", capName))
		}
		if closure.Lock == nil {
			return NewUserError(fmt.Sprintf("%s has no lock.", capName))
		}
		if err := f.checkKey(cmdCtx, closure.Lock); err != nil {
			return err
		}
		oi.Locked = true
		return f.publish(cmdCtx, fmt.Sprintf("You lock %s.", name), fmt.Sprintf("%s locks %s.", cmdCtx.Actor.Name, name))

	case "unlock":
		if !oi.Locked {
			return NewUserError(fmt.Sprintf("%s is not locked.", capName))
		}
		if closure.Lock == nil {
			return NewUserError(fmt.Sprintf("%s has no lock.", capName))
		}
		if err := f.checkKey(cmdCtx, closure.Lock); err != nil {
			return err
		}
		oi.Locked = false
		return f.publish(cmdCtx, fmt.Sprintf("You unlock %s.", name), fmt.Sprintf("%s unlocks %s.", cmdCtx.Actor.Name, name))
	}

	return nil
}

func (f *ClosureHandlerFactory) checkKey(cmdCtx *CommandContext, lock *game.Lock) error {
	if cmdCtx.Actor.Inventory.FindObjByDef(lock.KeyId.Id()) == nil {
		return NewUserError("You don't have the key.")
	}
	return nil
}

func (f *ClosureHandlerFactory) publish(cmdCtx *CommandContext, selfMsg, roomMsg string) error {
	if f.pub == nil {
		return nil
	}
	if err := f.pub.Publish(game.SinglePlayer(cmdCtx.Session.CharId), nil, []byte(selfMsg)); err != nil {
		return err
	}
	zoneId, roomId := cmdCtx.Session.Location()
	room := cmdCtx.World.Instances()[zoneId].GetRoom(roomId)
	return f.pub.Publish(room, []storage.Identifier{cmdCtx.Session.CharId}, []byte(roomMsg))
}
