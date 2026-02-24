package game

import "github.com/pixil98/go-mud/internal/storage"

// PlayerGroup represents any group of players that can be iterated.
// Implemented by RoomInstance, ZoneInstance, WorldState, and singlePlayer.
type PlayerGroup interface {
	ForEachPlayer(func(storage.Identifier, *PlayerState))
}

// singlePlayer wraps a single charId as a PlayerGroup.
type singlePlayer storage.Identifier

func (sp singlePlayer) ForEachPlayer(fn func(storage.Identifier, *PlayerState)) {
	fn(storage.Identifier(sp), nil)
}

// SinglePlayer returns a PlayerGroup targeting a single player.
func SinglePlayer(charId storage.Identifier) PlayerGroup {
	return singlePlayer(charId)
}

// Publisher sends messages to groups of players.
type Publisher interface {
	Publish(targets PlayerGroup, exclude []storage.Identifier, data []byte) error
}
