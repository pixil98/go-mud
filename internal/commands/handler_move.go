package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// MoveHandlerFactory creates handlers that move players between rooms.
// Config:
//   - direction (required): the direction to move (north, south, east, west, up, down)
type MoveHandlerFactory struct {
	world *game.WorldState
	pub   game.Publisher
}

// NewMoveHandlerFactory creates a new MoveHandlerFactory with access to world state.
func NewMoveHandlerFactory(world *game.WorldState, pub game.Publisher) *MoveHandlerFactory {
	return &MoveHandlerFactory{world: world, pub: pub}
}

func (f *MoveHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "direction", Required: true},
		},
	}
}

func (f *MoveHandlerFactory) ValidateConfig(config map[string]any) error {
	direction, ok := config["direction"].(string)
	if !ok || direction == "" {
		return fmt.Errorf("direction is required")
	}

	return nil
}

func (f *MoveHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		if cmdCtx.Session.InCombat {
			return NewUserError("You can't move while fighting!")
		}

		// Read direction from expanded config
		direction := strings.ToLower(cmdCtx.Config["direction"])
		if direction == "" {
			return fmt.Errorf("direction not set in config")
		}

		zoneId, roomId := cmdCtx.Session.Location()

		// Look up current room instance
		fromRoom := f.world.Instances()[zoneId].GetRoom(roomId)
		if fromRoom == nil {
			return NewUserError("You are in an invalid location.")
		}

		// Check if exit exists
		exit, exists := fromRoom.Room.Get().Exits[direction]
		if !exists {
			return NewUserError(fmt.Sprintf("You cannot go %s from here.", direction))
		}

		// Check if exit is blocked by a closure
		if exit.Closure != nil {
			if fromRoom.IsExitLocked(direction) {
				return NewUserError(fmt.Sprintf("The %s is locked.", exit.Closure.Name))
			}
			if fromRoom.IsExitClosed(direction) {
				return NewUserError(fmt.Sprintf("The %s is closed.", exit.Closure.Name))
			}
		}

		// Determine destination zone (default to current if not specified)
		destZone := exit.Zone.Id()
		if destZone == "" {
			destZone = zoneId
		}
		destRoomId := exit.Room.Id()

		// Get destination room instance
		toRoom := f.world.Instances()[destZone].GetRoom(destRoomId)
		if toRoom == nil {
			return NewUserError("Alas, you cannot go that way...")
		}

		// Move the player (updates location, subscriptions, and room player lists)
		cmdCtx.Session.Move(fromRoom, toRoom)

		// Send room description to player
		roomDesc := toRoom.Describe(cmdCtx.Actor.Name)
		if f.pub != nil {
			return f.pub.Publish(game.SinglePlayer(cmdCtx.Session.Character.Id()), nil, []byte(roomDesc))
		}

		return nil
	}, nil
}
