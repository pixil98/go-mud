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
}

// NewMoveHandlerFactory creates a new MoveHandlerFactory with access to world state.
func NewMoveHandlerFactory(world *game.WorldState) *MoveHandlerFactory {
	return &MoveHandlerFactory{world: world}
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

func (f *MoveHandlerFactory) Create(config map[string]any, pub Publisher) (CommandFunc, error) {
	direction := strings.ToLower(config["direction"].(string))

	return func(ctx context.Context, data *TemplateData) error {
		if data.State == nil {
			return fmt.Errorf("player state not found")
		}

		zoneId, roomId := data.State.Location()

		// Look up current room
		roomKey := string(zoneId) + "-" + string(roomId)
		currentRoom := f.world.Rooms().Get(roomKey)
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
		destRoom := storage.Identifier(exit.RoomId)

		// Verify destination room exists
		destRoomKey := string(destZone) + "-" + string(destRoom)
		newRoom := f.world.Rooms().Get(destRoomKey)
		if newRoom == nil {
			return NewUserError("That exit leads nowhere.")
		}

		// Get charId from Actor
		charId := storage.Identifier(strings.ToLower(data.Actor.Name()))

		// Move the player
		err := f.world.MovePlayer(charId, destZone, destRoom)
		if err != nil {
			return fmt.Errorf("failed to move player: %w", err)
		}

		// Send room description to player
		playerChannel := fmt.Sprintf("player-%s", strings.ToLower(data.Actor.Name()))
		roomDesc := formatRoomDescription(newRoom)
		if pub != nil {
			_ = pub.Publish(playerChannel, []byte(roomDesc))
		}

		return nil
	}, nil
}

func formatRoomDescription(room *game.Room) string {
	exitList := formatExits(room.Exits)
	return fmt.Sprintf("%s\n%s\n%s", room.Name, room.Description, exitList)
}

func formatExits(exits map[string]game.Exit) string {
	var dirs []string
	if len(exits) == 0 {
		return "[Exits: none]"
	} else {
	dirs = make([]string, 0, len(exits))
	}
	for dir := range exits {
		dirs = append(dirs, dir)
	}
	return fmt.Sprintf("[Exits: %s]", strings.Join(dirs, ", "))
}