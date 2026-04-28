package game

// PlayerGroup represents any group of players that can be iterated.
// Implemented by RoomInstance, ZoneInstance, WorldState, and groupPublishTarget.
type PlayerGroup interface {
	ForEachPlayer(func(string, *CharacterInstance))
}

// MessageTarget is the audience for a published message. Implemented by
// CharacterInstance, MobileInstance, RoomInstance, ZoneInstance, WorldState,
// and groupPublishTarget. Larger targets compose by calling Publish on their
// members; CharacterInstance is the leaf that actually delivers data.
type MessageTarget interface {
	Publish(data []byte, exclude []string)
}
