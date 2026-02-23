package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// ClosureHandlerFactory creates handlers for open/close/lock/unlock commands.
// Config:
//   - action (required): "open", "close", "lock", or "unlock"
//
// The handler accepts a single string input and resolves the target as either
// an exit direction or a container object in the room.
type ClosureHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

func NewClosureHandlerFactory(world *game.WorldState, pub Publisher) *ClosureHandlerFactory {
	return &ClosureHandlerFactory{world: world, pub: pub}
}

func (f *ClosureHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "action", Required: true},
			{Name: "target", Required: true},
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
		target := strings.ToLower(cmdCtx.Config["target"])

		zoneId, roomId := cmdCtx.Session.Location()
		room := f.world.Instances()[zoneId].GetRoom(roomId)
		if room == nil {
			return NewUserError("You are in an invalid location.")
		}

		// Try exit first, then container
		if exit, ok := room.Room.Get().Exits[target]; ok && exit.Closure != nil {
			return f.handleExit(action, target, exit.Closure, room, cmdCtx)
		}

		oi := room.FindObj(target)
		if oi != nil && oi.Object.Get().HasFlag(game.ObjectFlagContainer) && oi.Object.Get().Closure != nil {
			return f.handleContainer(action, oi, cmdCtx)
		}

		return NewUserError(fmt.Sprintf("You don't see anything to %s.", action))
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
		room.SetExitClosed(direction, false)
		return f.publish(cmdCtx, fmt.Sprintf("You open the %s.", name), fmt.Sprintf("%s opens the %s.", cmdCtx.Actor.Name, name))

	case "close":
		if room.IsExitClosed(direction) {
			return NewUserError(fmt.Sprintf("The %s is already closed.", name))
		}
		room.SetExitClosed(direction, true)
		return f.publish(cmdCtx, fmt.Sprintf("You close the %s.", name), fmt.Sprintf("%s closes the %s.", cmdCtx.Actor.Name, name))

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
		room.SetExitLocked(direction, true)
		return f.publish(cmdCtx, fmt.Sprintf("You lock the %s.", name), fmt.Sprintf("%s locks the %s.", cmdCtx.Actor.Name, name))

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
		room.SetExitLocked(direction, false)
		return f.publish(cmdCtx, fmt.Sprintf("You unlock the %s.", name), fmt.Sprintf("%s unlocks the %s.", cmdCtx.Actor.Name, name))
	}

	return nil
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
	if err := f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte(selfMsg)); err != nil {
		return err
	}
	return f.pub.PublishToRoom(cmdCtx.Session.ZoneId, cmdCtx.Session.RoomId, []byte(roomMsg))
}
