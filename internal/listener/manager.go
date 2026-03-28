package listener

import (
	"context"
	"io"
	"log/slog"

	"github.com/pixil98/go-mud/internal/player"
)

// ConnectionManager routes incoming network connections to the PlayerManager.
type ConnectionManager struct {
	pm *player.PlayerManager
}

// NewConnectionManager creates a ConnectionManager backed by the given PlayerManager.
func NewConnectionManager(pm *player.PlayerManager) *ConnectionManager {
	return &ConnectionManager{
		pm: pm,
	}
}

// AcceptConnection starts a player session for the given connection.
func (m *ConnectionManager) AcceptConnection(ctx context.Context, conn io.ReadWriter) {
	if err := m.pm.RunSession(ctx, conn); err != nil {
		slog.WarnContext(ctx, "player session", "error", err)
	}
}
