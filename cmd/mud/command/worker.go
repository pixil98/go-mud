package command

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/driver"
	"github.com/pixil98/go-mud/internal/listener"
	"github.com/pixil98/go-mud/internal/player"
	"github.com/pixil98/go-service/service"
)

func BuildWorkers(config interface{}) (service.WorkerList, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("unable to cast config")
	}

	//TODO: Probably get a better context
	ctx := context.Background()

	// Create Stores
	storeCharacters, err := cfg.Storage.Characters.NewFileStore(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating character store: %w", err)
	}
	storePronouns, err := cfg.Storage.Pronouns.NewFileStore(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating pronoun store: %w", err)
	}
	storeRaces, err := cfg.Storage.Races.NewFileStore(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating race store: %w", err)
	}

	// Create player manager
	playerManager := player.NewPlayerManager(storeCharacters, storePronouns, storeRaces)

	// Create connection manager
	connectionManager := listener.NewConnectionManager(playerManager)

	// Create Listeners
	listeners := make(service.WorkerList, len(cfg.Listeners))
	for i, l := range cfg.Listeners {
		listener, err := l.NewListener(connectionManager)
		if err != nil {
			return nil, fmt.Errorf("creating listener %d: %w", i, err)
		}
		listeners[fmt.Sprintf("listener-%d", i)] = listener
	}

	// Setup the mud driver
	driver := driver.NewMudDriver([]driver.TickHandler{
		playerManager,
	})

	// Create a worker list
	return service.WorkerList{
		"driver":    driver,
		"listeners": &listeners,
	}, nil
}
