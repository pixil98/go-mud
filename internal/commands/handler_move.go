package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// MoveHandlerFactory creates handlers that move players between rooms.
// Config:
//   - direction (required): the direction to move (north, south, east, west, up, down)
type MoveHandlerFactory struct {
	zones ZoneLocator
}

// NewMoveHandlerFactory creates a new MoveHandlerFactory with access to world state.
func NewMoveHandlerFactory(zones ZoneLocator) *MoveHandlerFactory {
	return &MoveHandlerFactory{zones: zones}
}

// Spec returns the handler's target and config requirements.
func (f *MoveHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Config: []ConfigRequirement{
			{Name: "direction", Required: true},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *MoveHandlerFactory) ValidateConfig(config map[string]string) error {
	direction := config["direction"]
	if direction == "" {
		return fmt.Errorf("direction is required")
	}

	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *MoveHandlerFactory) Create() (CommandFunc, error) {
	return f.handle, nil
}

func (f *MoveHandlerFactory) handle(ctx context.Context, in *CommandInput) error {
	char := in.Actor
	if char.IsInCombat() {
		return NewUserError("You can't move while fighting!")
	}

	// Read direction from expanded config
	direction := strings.ToLower(in.Config["direction"])
	if direction == "" {
		return fmt.Errorf("direction not set in config")
	}

	zoneId, roomId := char.Location()

	// Look up current room instance
	fromRoom := f.zones.GetZone(zoneId).GetRoom(roomId)
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
	toRoom := f.zones.GetZone(destZone).GetRoom(destRoomId)
	if toRoom == nil {
		return NewUserError("Alas, you cannot go that way...")
	}

	if toRoom.Room.Get().HasFlag(assets.RoomFlagSingleOccupant) && toRoom.PlayerCount() >= 1 {
		return NewUserError("There isn't enough room for you to enter.")
	}

	// Announce departure
	announceDepart(char, fromRoom, direction)

	// Move the player (updates location, subscriptions, and room player lists)
	char.Move(fromRoom, toRoom)

	// Announce arrival
	announceArrive(char, toRoom)

	// Send room description to player
	char.Notify(DescribeRoom(char, toRoom))

	// Move any followers in the old room
	moveFollowers(char, fromRoom, toRoom, direction)

	return nil
}

// announceDepart notifies players in the room that an actor is leaving.
func announceDepart(actor interface{ Name() string; HasGrant(string, string) bool }, room *game.RoomInstance, direction string) {
	if actor.HasGrant(assets.PerkGrantSneak, "") {
		return
	}
	msg := fmt.Sprintf("%s leaves %s.", actor.Name(), direction)
	announceToRoom(room, actor, msg)
}

// announceArrive notifies players in the room that an actor has arrived.
func announceArrive(actor interface{ Name() string; HasGrant(string, string) bool }, room *game.RoomInstance) {
	if actor.HasGrant(assets.PerkGrantSneak, "") {
		return
	}
	msg := fmt.Sprintf("%s has arrived.", actor.Name())
	announceToRoom(room, actor, msg)
}

// announceToRoom sends a message to all players in the room except the actor.
func announceToRoom(room *game.RoomInstance, actor interface{ Name() string }, msg string) {
	actorName := actor.Name()
	room.ForEachPlayer(func(_ string, ps *game.CharacterInstance) {
		if ps.Name() != actorName {
			ps.Notify(msg)
		}
	})
}

// moveFollowers walks the leader's follower tree and moves each follower from
// fromRoom to toRoom. Followers not in the same room or in combat are skipped
// along with their entire subtree.
func moveFollowers(leader game.FollowTarget, fromRoom, toRoom *game.RoomInstance, direction string) {
	fromRoomId := fromRoom.Room.Id()
	for _, fl := range leader.Followers() {
		_, followerRoom := fl.Location()
		if followerRoom != fromRoomId {
			continue
		}
		if fl.IsInCombat() {
			fl.Notify(fmt.Sprintf("%s leaves %s without you.", leader.Name(), direction))
			continue
		}

		announceDepart(fl, fromRoom, direction)
		fl.Move(fromRoom, toRoom)
		announceArrive(fl, toRoom)
		fl.Notify(fmt.Sprintf("You follow %s.\n%s", leader.Name(), DescribeRoom(fl, toRoom)))
		moveFollowers(fl, fromRoom, toRoom, direction)
	}
}
