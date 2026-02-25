package game

// PlayerGroup represents any group of players that can be iterated.
// Implemented by RoomInstance, ZoneInstance, WorldState, and singlePlayer.
type PlayerGroup interface {
	ForEachPlayer(func(string, *PlayerState))
}

// singlePlayer wraps a single charId as a PlayerGroup.
type singlePlayer string

func (sp singlePlayer) ForEachPlayer(fn func(string, *PlayerState)) {
	fn(string(sp), nil)
}

// SinglePlayer returns a PlayerGroup targeting a single player.
func SinglePlayer(charId string) PlayerGroup {
	return singlePlayer(charId)
}

// Publisher sends messages to groups of players.
type Publisher interface {
	Publish(targets PlayerGroup, exclude []string, data []byte) error
}
