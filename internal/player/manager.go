package player

import "context"

type PlayerManager struct {
}

func NewPlayerManager() *PlayerManager {
	return &PlayerManager{}
}

func (m *PlayerManager) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (m *PlayerManager) Tick(ctx context.Context) error {
	return nil
}
