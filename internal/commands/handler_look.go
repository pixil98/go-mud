package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// LookHandlerFactory creates handlers that display the current room.
// TODO: Add support for looking at targets (look <player>, look <item>, etc.)
type LookHandlerFactory struct {
	world *game.WorldState
}

// NewLookHandlerFactory creates a new LookHandlerFactory with access to world state.
func NewLookHandlerFactory(world *game.WorldState) *LookHandlerFactory {
	return &LookHandlerFactory{world: world}
}

func (f *LookHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *LookHandlerFactory) Create(config map[string]any, pub Publisher) (CommandFunc, error) {
	return func(ctx context.Context, data *TemplateData) error {
		if data.State == nil {
			return fmt.Errorf("player state not found")
		}

		zoneId, roomId := data.State.Location()

		// Look up current room
		roomKey := string(zoneId) + "-" + string(roomId)
		room := f.world.Rooms().Get(roomKey)
		if room == nil {
			return NewUserError("You are in an invalid location.")
		}

		// TODO: Add helper functions for channel naming schemes (player-X, zone-X, zone-X-room-Y)
		// to avoid recreating the naming patterns everywhere
		playerChannel := fmt.Sprintf("player-%s", strings.ToLower(data.Actor.Name()))
		roomDesc := formatRoomDescription(room)
		if pub != nil {
			_ = pub.Publish(playerChannel, []byte(roomDesc))
		}

		return nil
	}, nil
}
