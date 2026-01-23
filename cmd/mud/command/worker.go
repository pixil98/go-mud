package command

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/listener"
	"github.com/pixil98/go-mud/internal/player"
	"github.com/pixil98/go-mud/internal/plugins"
	"github.com/pixil98/go-mud/internal/plugins/base"
	"github.com/pixil98/go-service/service"
)

func BuildWorkers(config interface{}) (service.WorkerList, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("unable to cast config")
	}

	//TODO: Probably get a better context
	ctx := context.Background()

	// Setup Plugins
	pluginManager := plugins.NewPluginManager()
	err := pluginManager.Register(ctx, &base.BasePlugin{})
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

	// Create command handler and compile all commands
	cmdHandler := commands.NewHandler(storeCmds)
	err = cmdHandler.CompileAll()
	if err != nil {
		return nil, fmt.Errorf("compiling commands: %w", err)
	}

	// Create player manager
	playerManager := player.NewPlayerManager(cmdHandler, pluginManager, storeCharacters)

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
