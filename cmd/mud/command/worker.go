package command

import (
	"fmt"
	"time"

	"github.com/pixil98/go-mud/internal/combat"
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

	// Build dictionary of all game stores
	dict, err := cfg.Storage.BuildDictionary()
	if err != nil {
		return nil, fmt.Errorf("building dictionary: %w", err)
	}

	// Build command store separately (not a game type)
	storeCmds, err := cfg.Storage.Commands.BuildFileStore()
	if err != nil {
		return nil, fmt.Errorf("creating command store: %w", err)
	}

	// Setup the nats server
	natsServer, err := cfg.Nats.buildNatsServer()
	if err != nil {
		return nil, fmt.Errorf("creating nats server: %w", err)
	}

	// Create world state (must be before command handler since handlers need it)
	world, err := game.NewWorldState(natsServer, dict.Zones, dict.Rooms)
	if err != nil {
		return nil, fmt.Errorf("creating world state: %w", err)
	}

	// Spawn initial mobiles and objects in all zones
	for _, zi := range world.Instances() {
		err := zi.Reset(true, world.Instances())
		if err != nil {
			return nil, err
		}
	}

	// Create publisher for command handlers
	publisher := messaging.NewNatsPublisher(natsServer)

	// Create combat manager
	defaultZone := cfg.PlayerManager.DefaultZone
	defaultRoom := cfg.PlayerManager.DefaultRoom
	combatPub := game.NewCombatMessagePublisher(publisher)
	combatEvents := game.NewCombatEventHandler(world, publisher,
		storage.Identifier(defaultZone), storage.Identifier(defaultRoom))
	combatManager := combat.NewManager(combatPub, combatEvents)

	// Create command handler and compile all commands
	cmdHandler, err := commands.NewHandler(storeCmds, dict, publisher, world, combatManager)
	if err != nil {
		return nil, fmt.Errorf("compiling commands: %w", err)
	}

	// Create player manager
	playerManager, err := cfg.PlayerManager.BuildPlayerManager(cmdHandler, world, dict)
	if err != nil {
		return nil, fmt.Errorf("creating player manager: %w", err)
	}

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
	driver := game.NewMudDriver([]game.Ticker{world, playerManager, combatManager}, opts...)

	// Create a worker list
	return service.WorkerList{
		"driver":      driver,
		"listeners":   &listeners,
		"nats server": natsServer,
	}, nil
}
