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
	if err := m.pm.RunSession(ctx, conn); err != nil {
		slog.WarnContext(ctx, "player session", "error", err)
	}
}
