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

	if !game.ValidDirections[strings.ToLower(direction)] {
		return fmt.Errorf("invalid direction: %s", direction)
	}

	return nil
}

func (f *MoveHandlerFactory) Create(config map[string]any) (CommandFunc, error) {
	direction := strings.ToLower(config["direction"].(string))

	return func(ctx context.Context, data *TemplateData) error {
		if data.State == nil {
			return fmt.Errorf("player state not found")
		}

		zoneId, roomId := data.State.Location()

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
			return NewUserError("Alas, you cannot go that way...")
		}

		// Move the player (updates location and subscriptions)
		data.State.Move(destZone, destRoomId)

		// TODO: we should reuse the look command
		// Send room description to player
		playerChannel := fmt.Sprintf("player-%s", strings.ToLower(data.Actor.Name))
		roomDesc := FormatFullRoomDescription(f.world, newRoom, destZone, destRoomId, data.Actor.Name)
		if f.pub != nil {
			_ = f.pub.Publish(playerChannel, []byte(roomDesc))
		}

		return nil
	}, nil
}
