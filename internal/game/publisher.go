package game

import "github.com/pixil98/go-mud/internal/storage"

// Publisher provides methods for publishing messages to game channels.
type Publisher interface {
	PublishToPlayer(charId storage.Identifier, data []byte) error
	PublishToRoom(zoneId, roomId storage.Identifier, data []byte) error
}