package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// MoveHandlerFactory creates handlers that move players between rooms.
// Config:
//   - direction (required): the direction to move (north, south, east, west, up, down)
type MoveHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

// NewMoveHandlerFactory creates a new MoveHandlerFactory with access to world state.
func NewMoveHandlerFactory(world *game.WorldState, pub Publisher) *MoveHandlerFactory {
	return &MoveHandlerFactory{world: world, pub: pub}
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
		if cmdCtx.Session == nil {
			return fmt.Errorf("player state not found")
		}

		// Read direction from expanded config
		direction, ok := cmdCtx.Config["direction"].(string)
		if !ok || direction == "" {
			return fmt.Errorf("direction not set in config")
		}
		direction = strings.ToLower(direction)

		zoneId, roomId := cmdCtx.Session.Location()

		// Look up current room
		currentRoom := f.world.Rooms().Get(string(roomId))
		if currentRoom == nil {
			return NewUserError("You are in an invalid location.")
		}

		// Check if exit exists
		exit, exists := currentRoom.Exits[direction]
		if !exists {
			return NewUserError(fmt.Sprintf("You cannot go %s from here.", direction))
		}

		// Determine destination zone (default to current if not specified)
		destZone := storage.Identifier(exit.ZoneId)
		if exit.ZoneId == "" {
			destZone = zoneId
		}
		destRoomId := storage.Identifier(exit.RoomId)

		// Verify destination room exists
		newRoom := f.world.Rooms().Get(string(destRoomId))
		if newRoom == nil {
			return NewUserError("That exit leads nowhere.")
		}

		// Move the player (updates location and subscriptions)
		cmdCtx.Session.Move(destZone, destRoomId)

		// Send room description to player
		playerChannel := fmt.Sprintf("player-%s", strings.ToLower(cmdCtx.Actor.Name))
		roomDesc := FormatFullRoomDescription(f.world, newRoom, destZone, destRoomId, cmdCtx.Actor.Name)
		if f.pub != nil {
			_ = f.pub.Publish(playerChannel, []byte(roomDesc))
		}

		return nil
	}, nil
}
