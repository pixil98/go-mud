package listener

import (
	"context"
	"io"
	"log/slog"

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
	//TODO thread ctx though this for timeouts
	p, err := m.pm.NewPlayer(conn)
	if err != nil {
		slog.WarnContext(ctx, "creating player instance", "error", err)
		_, err := conn.Write([]byte("Failed to setup player session."))
		if err != nil {
			slog.WarnContext(ctx, "writing err to player", "error", err)
		}

		return
	}

	err = p.Play(ctx)
	if err != nil {
		slog.WarnContext(ctx, "playing", "error", err)
	}

	// Remove player from active players when they disconnect
	m.pm.RemovePlayer(p.Id())
}
