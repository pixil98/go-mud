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
		room := f.world.Rooms().Get(string(roomId))
		if room == nil {
			return NewUserError("You are in an invalid location.")
		}

		// TODO: Add helper functions for channel naming schemes (player-X, zone-X, zone-X-room-Y)
		// to avoid recreating the naming patterns everywhere
		playerChannel := fmt.Sprintf("player-%s", strings.ToLower(data.Actor.Name))
		roomDesc := FormatFullRoomDescription(f.world, room, zoneId, roomId, data.Actor.Name)
		if f.pub != nil {
			_ = f.pub.Publish(playerChannel, []byte(roomDesc))
		}

		return nil
	}, nil
}

// FormatFullRoomDescription builds a complete room description including mobs and players.
// actorName is excluded from the player list (so you don't see "You are here").
func FormatFullRoomDescription(world *game.WorldState, room *game.Room, zoneId, roomId storage.Identifier, actorName string) string {
	var sb strings.Builder
	sb.WriteString(room.Name)
	sb.WriteString("\n")
	sb.WriteString(room.Description)
	sb.WriteString("\n")

	// Show mobs
	mobs := world.GetMobilesInRoom(zoneId, roomId)
	if len(mobs) > 0 {
		for _, mi := range mobs {
			if mob := world.Mobiles().Get(string(mi.MobileId)); mob != nil {
				if mob.LongDesc != "" {
					sb.WriteString(mob.LongDesc)
				} else {
					sb.WriteString(fmt.Sprintf("%s is here.", mob.ShortDesc))
				}
				sb.WriteString("\n")
			}
		}
	}

	// TODO: Show items

	// Show players
	var playersHere []storage.Identifier
	world.ForEachPlayer(func(charId storage.Identifier, state game.PlayerState) {
		pZone, pRoom := state.Location()
		if pZone == zoneId && pRoom == roomId {
			if char := world.Characters().Get(string(charId)); char != nil {
				if char.Name != actorName {
					playersHere = append(playersHere, charId)
				}
			}
		}
	})
	if len(playersHere) > 0 {
		for _, charId := range playersHere {
			name := string(charId) // fallback to ID
			if char := world.Characters().Get(string(charId)); char != nil {
				name = char.Name
			}
			sb.WriteString(fmt.Sprintf("%s is here.\n", name))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(formatExits(room.Exits))

	return sb.String()
}

func formatExits(exits map[string]game.Exit) string {
	if len(exits) == 0 {
		return "[Exits: none]"
	}
	dirs := make([]string, 0, len(exits))
	for dir := range exits {
		dirs = append(dirs, dir)
	}
	return fmt.Sprintf("[Exits: %s]", strings.Join(dirs, ", "))
}
