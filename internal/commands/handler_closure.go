package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
)

// ClosureActor provides the character state needed by the closure handler.
type ClosureActor interface {
	Id() string
	Name() string
	Publish(data []byte, exclude []string)
	Room() *game.RoomInstance
	Inventory() *game.Inventory
}

var _ ClosureActor = (*game.CharacterInstance)(nil)

// ClosureHandlerFactory creates handlers for open/close/lock/unlock commands.
// Config:
//   - action (required): "open", "close", "lock", or "unlock"
//
// Targets:
//   - target (required): an exit or container object resolved by the command system
type ClosureHandlerFactory struct{}

// NewClosureHandlerFactory creates a handler factory for open/close/lock/unlock commands.
func NewClosureHandlerFactory() *ClosureHandlerFactory {
	return &ClosureHandlerFactory{}
}

// Spec returns the required target (exit or container object) and config (action) for closure commands.
func (f *ClosureHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypeObject | targetTypeExit, Required: true},
		},
		Config: []ConfigRequirement{
			{Name: "action", Required: true},
		},
	}
}

// ValidateConfig checks that action is one of open, close, lock, or unlock.
func (f *ClosureHandlerFactory) ValidateConfig(config map[string]string) error {
	switch config["action"] {
	case assets.ClosureActionOpen, assets.ClosureActionClose, assets.ClosureActionLock, assets.ClosureActionUnlock:
		return nil
	default:
		return errors.New("action must be open, close, lock, or unlock")
	}
}

// Create returns a compiled command function for the configured closure action.
func (f *ClosureHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[ClosureActor](f.handle), nil
}

func (f *ClosureHandlerFactory) handle(ctx context.Context, char ClosureActor, in *CommandInput) error {
	action := in.Config["action"]
	target := in.FirstTarget("target")

	switch target.Type {
	case targetTypeExit:
		re := target.Exit.exit
		if re.Exit.Closure == nil {
			return NewUserError(fmt.Sprintf("You can't %s that.", action))
		}
		return f.handleExit(action, target.Exit.Direction, re, char.Room(), char)

	case targetTypeObject:
		oi := target.Obj.instance
		if !oi.Object.Get().HasFlag(assets.ObjectFlagContainer) || oi.Object.Get().Closure == nil {
			return NewUserError(fmt.Sprintf("You can't %s that.", action))
		}
		return f.handleContainer(action, oi, char)

	default:
		return NewUserError(fmt.Sprintf("You can't %s that.", action))
	}
}

func (f *ClosureHandlerFactory) handleExit(action, direction string, re *game.ResolvedExit, room *game.RoomInstance, char ClosureActor) error {
	closure := re.Exit.Closure
	name := closure.Name

	switch action {
	case assets.ClosureActionOpen:
		if re.IsLocked() {
			return NewUserError(fmt.Sprintf("The %s is locked.", name))
		}
		if !re.IsClosed() {
			return NewUserError(fmt.Sprintf("The %s is already open.", name))
		}

	case assets.ClosureActionClose:
		if re.IsClosed() {
			return NewUserError(fmt.Sprintf("The %s is already closed.", name))
		}

	case assets.ClosureActionLock:
		if !re.IsClosed() {
			return NewUserError(fmt.Sprintf("You need to close the %s first.", name))
		}
		if re.IsLocked() {
			return NewUserError(fmt.Sprintf("The %s is already locked.", name))
		}
		if closure.Lock == nil {
			return NewUserError(fmt.Sprintf("The %s has no lock.", name))
		}
		if err := f.checkKey(char, closure.Lock); err != nil {
			return err
		}

	case assets.ClosureActionUnlock:
		if !re.IsLocked() {
			return NewUserError(fmt.Sprintf("The %s is not locked.", name))
		}
		if closure.Lock == nil {
			return NewUserError(fmt.Sprintf("The %s has no lock.", name))
		}
		if err := f.checkKey(char, closure.Lock); err != nil {
			return err
		}
	}

	// Apply state change to this exit and the other side of the door.
	applyExitAction(action, re)
	if _, other := re.OtherSide(room); other != nil {
		applyExitAction(action, other)
	}

	return f.publish(char, fmt.Sprintf("You %s the %s.", action, name), fmt.Sprintf("%s %ss the %s.", char.Name(), action, name))
}

// applyExitAction applies a closure state change to a resolved exit.
func applyExitAction(action string, re *game.ResolvedExit) {
	switch action {
	case assets.ClosureActionOpen:
		re.SetClosed(false)
	case assets.ClosureActionClose:
		re.SetClosed(true)
	case assets.ClosureActionLock:
		re.SetLocked(true)
	case assets.ClosureActionUnlock:
		re.SetLocked(false)
	}
}

func (f *ClosureHandlerFactory) handleContainer(action string, oi *game.ObjectInstance, char ClosureActor) error {
	closure := oi.Object.Get().Closure
	name := closure.Name
	if name == "" {
		name = oi.Object.Get().ShortDesc
	}
	capName := display.Capitalize(name)

	switch action {
	case assets.ClosureActionOpen:
		if oi.Locked {
			return NewUserError(fmt.Sprintf("%s is locked.", capName))
		}
		if !oi.Closed {
			return NewUserError(fmt.Sprintf("%s is already open.", capName))
		}
		oi.Closed = false
		return f.publish(char, fmt.Sprintf("You open %s.", name), fmt.Sprintf("%s opens %s.", char.Name(), name))

	case assets.ClosureActionClose:
		if oi.Closed {
			return NewUserError(fmt.Sprintf("%s is already closed.", capName))
		}
		oi.Closed = true
		return f.publish(char, fmt.Sprintf("You close %s.", name), fmt.Sprintf("%s closes %s.", char.Name(), name))

	case assets.ClosureActionLock:
		if !oi.Closed {
			return NewUserError(fmt.Sprintf("You need to close %s first.", name))
		}
		if oi.Locked {
			return NewUserError(fmt.Sprintf("%s is already locked.", capName))
		}
		if closure.Lock == nil {
			return NewUserError(fmt.Sprintf("%s has no lock.", capName))
		}
		if err := f.checkKey(char, closure.Lock); err != nil {
			return err
		}
		oi.Locked = true
		return f.publish(char, fmt.Sprintf("You lock %s.", name), fmt.Sprintf("%s locks %s.", char.Name(), name))

	case assets.ClosureActionUnlock:
		if !oi.Locked {
			return NewUserError(fmt.Sprintf("%s is not locked.", capName))
		}
		if closure.Lock == nil {
			return NewUserError(fmt.Sprintf("%s has no lock.", capName))
		}
		if err := f.checkKey(char, closure.Lock); err != nil {
			return err
		}
		oi.Locked = false
		return f.publish(char, fmt.Sprintf("You unlock %s.", name), fmt.Sprintf("%s unlocks %s.", char.Name(), name))
	}

	return nil
}

func (f *ClosureHandlerFactory) checkKey(char ClosureActor, lock *assets.Lock) error {
	if char.Inventory().FindObjByDef(lock.KeyId.Id()) == nil {
		return NewUserError("You don't have the key.")
	}
	return nil
}

func (f *ClosureHandlerFactory) publish(char ClosureActor, selfMsg, roomMsg string) error {
	char.Publish([]byte(selfMsg), nil)
	char.Room().Publish([]byte(roomMsg), []string{char.Id()})
	return nil
}
