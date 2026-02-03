package command

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/listener"
	"github.com/pixil98/go-mud/internal/player"
	"github.com/pixil98/go-mud/internal/plugins"
	"github.com/pixil98/go-mud/internal/plugins/base"
	"github.com/pixil98/go-service"
)

func BuildWorkers(config interface{}) (service.WorkerList, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("unable to cast config")
	}

	// Setup Plugins
	pluginManager := plugins.NewPluginManager()
	err := pluginManager.Register(&base.BasePlugin{})
	if err != nil {
		return nil, err
	}

	// Create Stores
	storeCharacters, err := cfg.Storage.Characters.NewFileStore()
	if err != nil {
		return nil, fmt.Errorf("creating character store: %w", err)
	}
	storeCmds, err := cfg.Storage.Commands.NewFileStore()
	if err != nil {
		return nil, fmt.Errorf("creating command store: %w", err)
	}

	// Setup the nats server
	natsServer, err := cfg.Nats.NewNatsServer()
	if err != nil {
		return nil, fmt.Errorf("creating nats server: %w", err)
	}

	// Create command handler and compile all commands
	cmdHandler := commands.NewHandler(storeCmds, natsServer)
	err = cmdHandler.CompileAll()
	if err != nil {
		return nil, fmt.Errorf("compiling commands: %w", err)
	}

	// Create world state
	worldState := game.NewWorldState(storeCharacters)

	// Create player manager
	playerManager := player.NewPlayerManager(cmdHandler, pluginManager, natsServer, worldState)

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
	driver := game.NewMudDriver([]game.TickHandler{
		playerManager,
		pluginManager,
	})

	// Create a worker list
	return service.WorkerList{
		"driver":      driver,
		"listeners":   &listeners,
		"nats server": natsServer,
	}, nil
}
