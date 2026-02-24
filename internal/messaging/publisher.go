package messaging

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// NatsPublisher publishes messages to individual player NATS channels.
type NatsPublisher struct {
	server *NatsServer
}

// NewNatsPublisher wraps a NatsServer for per-player message delivery.
func NewNatsPublisher(server *NatsServer) *NatsPublisher {
	return &NatsPublisher{server: server}
}

func (p *NatsPublisher) Publish(targets game.PlayerGroup, exclude []storage.Identifier, data []byte) error {
	excludeSet := make(map[storage.Identifier]bool, len(exclude))
	for _, id := range exclude {
		excludeSet[id] = true
	}
	var firstErr error
	targets.ForEachPlayer(func(charId storage.Identifier, _ *game.PlayerState) {
		if excludeSet[charId] {
			return
		}
		if err := p.server.Publish(fmt.Sprintf("player-%s", charId), data); err != nil && firstErr == nil {
			firstErr = err
		}
	})
	return firstErr
}
