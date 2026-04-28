package commands

import "github.com/pixil98/go-mud/internal/game"

// PlayerGroup represents any group of players that can be iterated.
// Satisfied by *game.RoomInstance, *game.ZoneInstance, *game.WorldState, and
// the value returned by game.GroupPublishTarget.
type PlayerGroup interface {
	ForEachPlayer(func(string, *game.CharacterInstance))
}

// MessageTarget is the audience for a published message. Satisfied by
// *game.CharacterInstance, *game.MobileInstance, *game.RoomInstance,
// *game.ZoneInstance, *game.WorldState, and the value returned by
// game.GroupPublishTarget. Larger targets compose by calling Publish on their
// members; CharacterInstance is the leaf that actually delivers data.
type MessageTarget interface {
	Publish(data []byte, exclude []string)
}
