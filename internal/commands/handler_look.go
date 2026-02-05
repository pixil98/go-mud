package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// LookHandlerFactory creates handlers that display the current room.
// TODO: Add support for looking at targets (look <player>, look <item>, etc.)
type LookHandlerFactory struct {
	world *game.WorldState
	pub   Publisher
}

// NewLookHandlerFactory creates a new LookHandlerFactory with access to world state.
func NewLookHandlerFactory(world *game.WorldState, pub Publisher) *LookHandlerFactory {
	return &LookHandlerFactory{world: world, pub: pub}
}

func (f *LookHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *LookHandlerFactory) Create(config map[string]any) (CommandFunc, error) {
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

		// Find other players in the room
		var playersHere []storage.Identifier
		actorName := data.Actor.Name()
		f.world.ForEachPlayer(func(charId storage.Identifier, state game.PlayerState) {
			pZone, pRoom := state.Location()
			if pZone == zoneId && pRoom == roomId {
				if char := f.world.Characters().Get(string(charId)); char != nil {
					if char.Name() != actorName {
						playersHere = append(playersHere, charId)
					}
				}
			}
		})

		// TODO: Find mobs in the room
		// TODO: Find items in the room

		// TODO: Add helper functions for channel naming schemes (player-X, zone-X, zone-X-room-Y)
		// to avoid recreating the naming patterns everywhere
		playerChannel := fmt.Sprintf("player-%s", strings.ToLower(actorName))
		roomDesc := f.formatFullRoomDescription(room, playersHere)
		if f.pub != nil {
			_ = f.pub.Publish(playerChannel, []byte(roomDesc))
		}

		return nil
	}, nil
}

func (f *LookHandlerFactory) formatFullRoomDescription(room *game.Room, players []storage.Identifier) string {
	var sb strings.Builder
	sb.WriteString(room.Name)
	sb.WriteString("\n")
	sb.WriteString(room.Description)

	// Show players
	if len(players) > 0 {
		sb.WriteString("\n")
		for _, charId := range players {
			name := string(charId) // fallback to ID
			if char := f.world.Characters().Get(string(charId)); char != nil {
				name = char.Name()
			}
			sb.WriteString(fmt.Sprintf("%s is here.\n", name))
		}
	}

	// TODO: Show mobs
	// TODO: Show items

	sb.WriteString("\n")
	sb.WriteString(formatExits(room.Exits))

	return sb.String()
}
