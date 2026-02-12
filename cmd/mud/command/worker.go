package command

import (
	"fmt"
	"time"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/listener"
	"github.com/pixil98/go-mud/internal/messaging"
	"github.com/pixil98/go-mud/internal/storage"
	"github.com/pixil98/go-service"
)

func BuildWorkers(config interface{}) (service.WorkerList, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("unable to cast config")
	}

	// Create Stores
	storeCharacters, err := cfg.Storage.Characters.BuildFileStore()
	if err != nil {
		return nil, fmt.Errorf("creating character store: %w", err)
	}
	storeCmds, err := cfg.Storage.Commands.BuildFileStore()
	if err != nil {
		return nil, fmt.Errorf("creating command store: %w", err)
	}
	storeZones, err := cfg.Storage.Zones.BuildFileStore()
	if err != nil {
		return nil, fmt.Errorf("creating zone store: %w", err)
	}
	storeRooms, err := cfg.Storage.Rooms.BuildFileStore()
	if err != nil {
		return nil, fmt.Errorf("creating room store: %w", err)
	}
	storeMobiles, err := cfg.Storage.Mobiles.BuildFileStore()
	if err != nil {
		return nil, fmt.Errorf("creating mobile store: %w", err)
	}
	storeObjects, err := cfg.Storage.Objects.BuildFileStore()
	if err != nil {
		return nil, fmt.Errorf("creating object store: %w", err)
	}
	storePronouns, err := cfg.Storage.Pronouns.BuildFileStore()
	if err != nil {
		return nil, fmt.Errorf("creating pronoun store: %w", err)
	}
	storeRaces, err := cfg.Storage.Races.BuildFileStore()
	if err != nil {
		return nil, fmt.Errorf("creating race store: %w", err)
	}
	pronouns := storage.NewSelectableStorer(storePronouns)
	races := storage.NewSelectableStorer(storeRaces)

	// Setup the nats server
	natsServer, err := cfg.Nats.buildNatsServer()
	if err != nil {
		return nil, fmt.Errorf("creating nats server: %w", err)
	}

	// Create world state (must be before command handler since handlers need it)
	world := game.NewWorldState(natsServer, storeCharacters, storeZones, storeRooms, storeMobiles, storeObjects)

	// Spawn initial mobiles in all zones
	for zoneId := range storeZones.GetAll() {
		world.ResetZone(zoneId, true)
	}

	// Create publisher for command handlers
	publisher := messaging.NewNatsPublisher(natsServer)

	// Create command handler and compile all commands
	cmdHandler, err := commands.NewHandler(storeCmds, publisher, world, races)
	if err != nil {
		return nil, fmt.Errorf("compiling commands: %w", err)
	}

	// Create player manager
	playerManager := cfg.PlayerManager.BuildPlayerManager(cmdHandler, world, pronouns, races)

	// Create connection manager
	connectionManager := listener.NewConnectionManager(playerManager)

	// Create Listeners
	listeners := make(service.WorkerList, len(cfg.Listeners))
	for i, l := range cfg.Listeners {
		listener, err := l.BuildListener(connectionManager)
		if err != nil {
			return nil, fmt.Errorf("creating listener %d: %w", i, err)
		}
		listeners[fmt.Sprintf("listener-%d", i)] = listener
	}

	// Setup the mud driver
	var opts []game.MudDriverOpt
	if cfg.TickInterval != "" {
		l, err := time.ParseDuration(cfg.TickInterval)
		if err != nil {
			return nil, fmt.Errorf("parsing tick interval %q", cfg.TickInterval)
		}
		opts = append(opts, game.WithTickLength(l))
	}
	driver := game.NewMudDriver([]game.Ticker{world}, opts...)

	// Create a worker list
	return service.WorkerList{
		"driver":      driver,
		"listeners":   &listeners,
		"nats server": natsServer,
	}, nil
}
