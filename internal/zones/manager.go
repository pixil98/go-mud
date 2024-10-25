package zones

import (
	"context"

	"github.com/pixil98/go-log/log"
)

type ZoneManager struct {
	zones []*Zone
}

func NewZoneManager(zones []*Zone) *ZoneManager {
	return &ZoneManager{
		zones: zones,
	}
}

func (zm *ZoneManager) Tick(ctx context.Context) error {
	logger := log.GetLogger(ctx)

	logger.Infof("ZoneManager: %d zones", len(zm.zones))
	return nil
}
