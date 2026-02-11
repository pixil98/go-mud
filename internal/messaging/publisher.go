package messaging

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/storage"
)

// NatsPublisher implements the commands.Publisher interface by constructing
// channel names and delegating to the NatsServer.
type NatsPublisher struct {
	server *NatsServer
}

// NewNatsPublisher wraps a NatsServer with typed game channel methods.
func NewNatsPublisher(server *NatsServer) *NatsPublisher {
	return &NatsPublisher{server: server}
}

func (p *NatsPublisher) PublishToPlayer(charId storage.Identifier, data []byte) error {
	return p.server.Publish(fmt.Sprintf("player-%s", charId), data)
}

func (p *NatsPublisher) PublishToRoom(zoneId, roomId storage.Identifier, data []byte) error {
	return p.server.Publish(fmt.Sprintf("zone-%s-room-%s", zoneId, roomId), data)
}

func (p *NatsPublisher) PublishToZone(zoneId storage.Identifier, data []byte) error {
	return p.server.Publish(fmt.Sprintf("zone-%s", zoneId), data)
}

func (p *NatsPublisher) PublishToWorld(data []byte) error {
	return p.server.Publish("world", data)
}

func (p *NatsPublisher) Publish(subject string, data []byte) error {
	return p.server.Publish(subject, data)
}
