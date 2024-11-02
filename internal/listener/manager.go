package listener

import (
	"context"
	"io"

	"github.com/pixil98/go-log/log"
	"github.com/pixil98/go-mud/internal/player"
)

type ConnectionManager struct {
	pm *player.PlayerManager
}

func NewConnectionManager(pm *player.PlayerManager) *ConnectionManager {
	return &ConnectionManager{
		pm: pm,
	}
}

func (m *ConnectionManager) Start(ctx context.Context) error {
	<-ctx.Done()

	return nil
}

func (m *ConnectionManager) AcceptConnection(ctx context.Context, conn io.ReadWriter) {
	p := m.pm.NewPlayer(conn)

	err := p.Play(ctx)
	if err != nil {
		log.GetLogger(ctx).Error(err)
	}
}
