package command

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/driver"
	"github.com/pixil98/go-service/service"
)

func BuildWorkers(config interface{}) (service.WorkerList, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("unable to cast config")
	}

	// Create Listeners
	listeners := make(service.WorkerList, len(cfg.Listeners))
	for i, l := range cfg.Listeners {
		listener, err := l.NewListener()
		if err != nil {
			return nil, fmt.Errorf("creating listener %d: %w", i, err)
		}
		listeners[fmt.Sprintf("listener-%d", i)] = listener
	}

	// Create a new zone manager
	zoneManager, err := cfg.Zones.NewZoneManager()
	if err != nil {
		return nil, fmt.Errorf("creating zone manager: %w", err)
	}

	// Setup the mud driver
	driver := driver.NewMudDriver([]driver.Manager{
		zoneManager,
	})

	// Create a worker list
	return service.WorkerList{
		"driver":    driver,
		"listeners": &listeners,
	}, nil
}
