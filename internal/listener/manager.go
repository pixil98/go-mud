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

func (m *ConnectionManager) AcceptConnection(ctx context.Context, conn io.ReadWriter) {
	l := log.GetLogger(ctx)
	//TODO thread ctx though this for timeouts
	p, err := m.pm.NewPlayer(conn)
	if err != nil {
		l.Warnf("creating player instance: %v", err)
		_, err := conn.Write([]byte("Failed to setup player session."))
		if err != nil {
			l.Warnf("writing err to player: %v", err)
		}

		return
	}

	err = p.Play(ctx)
	if err != nil {
		log.GetLogger(ctx).Warnf("playing: %v", err)
	}
}
