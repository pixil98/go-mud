package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// LookHandlerFactory creates handlers that display the current room.
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

func (f *LookHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		if cmdCtx.Session == nil {
			return fmt.Errorf("player state not found")
		}

		// Check if target was resolved (from targets section)
		if target := cmdCtx.Targets["target"]; target != nil {
			return f.showTarget(cmdCtx, target)
		}

		return f.showRoom(cmdCtx)
	}, nil
}

// showRoom displays the current room description.
func (f *LookHandlerFactory) showRoom(cmdCtx *CommandContext) error {
	zoneId, roomId := cmdCtx.Session.Location()

	room := f.world.Rooms().Get(string(roomId))
	if room == nil {
		return NewUserError("You are in an invalid location.")
	}

	playerChannel := fmt.Sprintf("player-%s", strings.ToLower(cmdCtx.Actor.Name))
	roomDesc := FormatFullRoomDescription(f.world, room, zoneId, roomId, cmdCtx.Actor.Name)
	if f.pub != nil {
		_ = f.pub.Publish(playerChannel, []byte(roomDesc))
	}

	return nil
}

// showTarget displays information about a specific target.
func (f *LookHandlerFactory) showTarget(cmdCtx *CommandContext, target *TargetRef) error {
	playerChannel := fmt.Sprintf("player-%s", strings.ToLower(cmdCtx.Actor.Name))

	var msg string
	switch target.Type {
	case "player":
		msg = f.describePlayer(target.Player)
	case "mobile":
		msg = f.describeMob(target.Mob)
	case "object":
		msg = f.describeObj(target.Obj)
	default:
		return NewUserError("You can't look at that.")
	}

	if f.pub != nil {
		_ = f.pub.Publish(playerChannel, []byte(msg))
	}
	return nil
}

// TODO show inventory and condition
func (f *LookHandlerFactory) describePlayer(player *PlayerRef) string {
	return player.Description
}

func (f *LookHandlerFactory) describeMob(mob *MobileRef) string {
	return mob.Description
}

func (f *LookHandlerFactory) describeObj(obj *ObjectRef) string {
	return obj.Description
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

	// TODO: Show objects

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
	sort.Strings(dirs)
	return fmt.Sprintf("[Exits: %s]", strings.Join(dirs, ", "))
}
