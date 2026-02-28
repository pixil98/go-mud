package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
)

// MoveHandlerFactory creates handlers that move players between rooms.
// Config:
//   - direction (required): the direction to move (north, south, east, west, up, down)
type MoveHandlerFactory struct {
	rooms RoomLocator
	pub   game.Publisher
}

// NewMoveHandlerFactory creates a new MoveHandlerFactory with access to world state.
func NewMoveHandlerFactory(rooms RoomLocator, pub game.Publisher) *MoveHandlerFactory {
	return &MoveHandlerFactory{rooms: rooms, pub: pub}
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
		if err := canMove(cmdCtx.Session); err != nil {
			return err
		}

		// Read direction from expanded config
		direction := strings.ToLower(cmdCtx.Config["direction"])
		if direction == "" {
			return fmt.Errorf("direction not set in config")
		}

		zoneId, roomId := cmdCtx.Session.Location()

		// Look up current room instance
		fromRoom := f.rooms.GetRoom(zoneId, roomId)
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
		toRoom := f.rooms.GetRoom(destZone, destRoomId)
		if toRoom == nil {
			return NewUserError("Alas, you cannot go that way...")
		}

		// Move the player (updates location, subscriptions, and room player lists)
		cmdCtx.Session.Move(fromRoom, toRoom)

		// Send room description to player
		roomDesc := toRoom.Describe(cmdCtx.Actor.Name)
		if f.pub != nil {
			if err := f.pub.Publish(game.SinglePlayer(cmdCtx.Session.Character.Id()), nil, []byte(roomDesc)); err != nil {
				slog.Warn("failed to send room description", "error", err)
			}
		}

		// Move any followers in the old room
		f.moveFollowers(cmdCtx.Session.Character.Id(), cmdCtx.Actor.Name, fromRoom, toRoom, direction)

		return nil
	}, nil
}

// canMove returns a UserError if the player cannot move, or nil if they can.
func canMove(ps *game.PlayerState) error {
	if ps.InCombat {
		return NewUserError("You can't move while fighting!")
	}
	return nil
}

// moveFollowers moves all players following leaderId from fromRoom to toRoom.
// Followers who can't move stay behind with a message. Recurses for each moved
// follower so that chains (A follows B follows C) cascade correctly.
func (f *MoveHandlerFactory) moveFollowers(leaderId, leaderName string, fromRoom, toRoom *game.RoomInstance, direction string) {
	type follower struct {
		charId string
		ps     *game.PlayerState
	}

	// Snapshot followers while holding the room lock.
	var followers []follower
	fromRoom.ForEachPlayer(func(charId string, ps *game.PlayerState) {
		if ps.FollowingId == leaderId {
			followers = append(followers, follower{charId: charId, ps: ps})
		}
	})

	for _, fl := range followers {
		if canMove(fl.ps) != nil {
			if f.pub != nil {
				if err := f.pub.Publish(game.SinglePlayer(fl.charId), nil,
					[]byte(fmt.Sprintf("%s leaves %s without you.", leaderName, direction))); err != nil {
					slog.Warn("failed to notify follower left behind", "error", err)
				}
			}
			continue
		}

		fl.ps.Move(fromRoom, toRoom)

		if f.pub != nil {
			roomDesc := toRoom.Describe(fl.ps.Character.Get().Name)
			msg := fmt.Sprintf("You follow %s.\n%s", leaderName, roomDesc)
			if err := f.pub.Publish(game.SinglePlayer(fl.charId), nil, []byte(msg)); err != nil {
				slog.Warn("failed to send room description to follower", "error", err)
			}
		}

		// Recurse: move this follower's followers too.
		f.moveFollowers(fl.charId, fl.ps.Character.Get().Name, fromRoom, toRoom, direction)
	}
}
