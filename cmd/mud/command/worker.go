package command

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/commands"
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
	storeCmds, err := cfg.Storage.Commands.NewFileStore(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating command store: %w", err)
	}

	// Create command handler
	cmdHandler := commands.NewHandler(storeCmds)

	// Create player manager
	playerManager := player.NewPlayerManager(cmdHandler, storeCharacters, storePronouns, storeRaces)

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

	// Setup the nats server
	nats, err := cfg.Nats.NewNatsServer()
	if err != nil {
		return nil, fmt.Errorf("creating nats server: %w", err)
	}

	// Create a worker list
	return service.WorkerList{
		"driver":      driver,
		"listeners":   &listeners,
		"nats server": nats,
	}, nil
}
