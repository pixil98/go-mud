package commands

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestResolver_ResolvePlayer(t *testing.T) {
	tests := map[string]struct {
		chars         map[string]*game.Character
		onlinePlayers map[string]struct{ zone, room storage.Identifier }
		actorZone     storage.Identifier
		actorRoom     storage.Identifier
		name          string
		scope         Scope
		expPlayerName string
		expErr        string
	}{
		"world scope finds any player": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"other-zone", "other-room"},
			},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "bob",
			scope:         ScopeWorld,
			expPlayerName: "Bob",
		},
		"zone scope finds player in same zone": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "other-room"},
			},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "bob",
			scope:         ScopeZone,
			expPlayerName: "Bob",
		},
		"zone scope rejects player in different zone": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"other-zone", "other-room"},
			},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "bob",
			scope:     ScopeZone,
			expErr:    `Player "bob" not found.`,
		},
		"room scope finds player in same room": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "my-room"},
			},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "bob",
			scope:         ScopeRoom,
			expPlayerName: "Bob",
		},
		"room scope rejects player in different room same zone": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "other-room"},
			},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "bob",
			scope:     ScopeRoom,
			expErr:    `Player "bob" not found.`,
		},
		"combined scope room or zone finds in zone": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "other-room"},
			},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "bob",
			scope:         ScopeRoom | ScopeZone,
			expPlayerName: "Bob",
		},
		"case insensitive match": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "my-room"},
			},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "BOB",
			scope:         ScopeWorld,
			expPlayerName: "Bob",
		},
		"player not found": {
			chars:         map[string]*game.Character{},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "nobody",
			scope:         ScopeWorld,
			expErr:        `Player "nobody" not found.`,
		},
		"player offline": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "bob",
			scope:         ScopeWorld,
			expErr:        `Player "bob" not found.`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			charStore := &mockCharStore{chars: tt.chars}
			world := game.NewWorldState(nil, charStore, &mockZoneStore{zones: map[string]*game.Zone{}}, &mockRoomStore{rooms: map[string]*game.Room{}}, &mockMobileStore{mobiles: map[string]*game.Mobile{}}, &mockObjectStore{objects: map[string]*game.Object{}}, &mockRaceStore{races: map[string]*game.Race{}})

			// Add actor
			actorChan := make(chan []byte, 1)
			_ = world.AddPlayer("actor", actorChan, tt.actorZone, tt.actorRoom)

			// Add other players
			for charId, loc := range tt.onlinePlayers {
				ch := make(chan []byte, 1)
				_ = world.AddPlayer(storage.Identifier(charId), ch, loc.zone, loc.room)
			}

			resolver := NewResolver(world)
			result, err := resolver.resolvePlayer("actor", tt.name, tt.scope)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if result.Name != tt.expPlayerName {
				t.Errorf("Name = %q, expected %q", result.Name, tt.expPlayerName)
			}
		})
	}
}

func TestResolver_ResolveMob(t *testing.T) {
	tests := map[string]struct {
		mobiles     map[string]*game.Mobile
		rooms       map[string]*game.Room
		spawnedMobs []struct{ mobileId, zoneId, roomId string }
		actorZone   storage.Identifier
		actorRoom   storage.Identifier
		name        string
		scope       Scope
		expMobName  string
		expErr      string
	}{
		"room scope finds mob in same room": {
			mobiles: map[string]*game.Mobile{
				"guard": {Aliases: []string{"guard", "soldier"}, ShortDesc: "a burly guard"},
			},
			rooms: map[string]*game.Room{
				"my-room": {ZoneId: "my-zone"},
			},
			spawnedMobs: []struct{ mobileId, zoneId, roomId string }{
				{"guard", "my-zone", "my-room"},
			},
			actorZone:  "my-zone",
			actorRoom:  "my-room",
			name:       "guard",
			scope:      ScopeRoom,
			expMobName: "a burly guard",
		},
		"room scope matches by alias": {
			mobiles: map[string]*game.Mobile{
				"guard": {Aliases: []string{"guard", "soldier"}, ShortDesc: "a burly guard"},
			},
			rooms: map[string]*game.Room{
				"my-room": {ZoneId: "my-zone"},
			},
			spawnedMobs: []struct{ mobileId, zoneId, roomId string }{
				{"guard", "my-zone", "my-room"},
			},
			actorZone:  "my-zone",
			actorRoom:  "my-room",
			name:       "soldier",
			scope:      ScopeRoom,
			expMobName: "a burly guard",
		},
		"room scope case insensitive": {
			mobiles: map[string]*game.Mobile{
				"guard": {Aliases: []string{"guard"}, ShortDesc: "a burly guard"},
			},
			rooms: map[string]*game.Room{
				"my-room": {ZoneId: "my-zone"},
			},
			spawnedMobs: []struct{ mobileId, zoneId, roomId string }{
				{"guard", "my-zone", "my-room"},
			},
			actorZone:  "my-zone",
			actorRoom:  "my-room",
			name:       "GUARD",
			scope:      ScopeRoom,
			expMobName: "a burly guard",
		},
		"room scope rejects mob in different room": {
			mobiles: map[string]*game.Mobile{
				"guard": {Aliases: []string{"guard"}, ShortDesc: "a burly guard"},
			},
			rooms: map[string]*game.Room{
				"my-room":    {ZoneId: "my-zone"},
				"other-room": {ZoneId: "my-zone"},
			},
			spawnedMobs: []struct{ mobileId, zoneId, roomId string }{
				{"guard", "my-zone", "other-room"},
			},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "guard",
			scope:     ScopeRoom,
			expErr:    `Mobile "guard" not found.`,
		},
		"zone scope finds mob in different room same zone": {
			mobiles: map[string]*game.Mobile{
				"guard": {Aliases: []string{"guard"}, ShortDesc: "a burly guard"},
			},
			rooms: map[string]*game.Room{
				"my-room":    {ZoneId: "my-zone"},
				"other-room": {ZoneId: "my-zone"},
			},
			spawnedMobs: []struct{ mobileId, zoneId, roomId string }{
				{"guard", "my-zone", "other-room"},
			},
			actorZone:  "my-zone",
			actorRoom:  "my-room",
			name:       "guard",
			scope:      ScopeZone,
			expMobName: "a burly guard",
		},
		"zone scope rejects mob in different zone": {
			mobiles: map[string]*game.Mobile{
				"guard": {Aliases: []string{"guard"}, ShortDesc: "a burly guard"},
			},
			rooms: map[string]*game.Room{
				"my-room":    {ZoneId: "my-zone"},
				"other-room": {ZoneId: "other-zone"},
			},
			spawnedMobs: []struct{ mobileId, zoneId, roomId string }{
				{"guard", "other-zone", "other-room"},
			},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "guard",
			scope:     ScopeZone,
			expErr:    `Mobile "guard" not found.`,
		},
		"world scope finds mob anywhere": {
			mobiles: map[string]*game.Mobile{
				"guard": {Aliases: []string{"guard"}, ShortDesc: "a burly guard"},
			},
			rooms: map[string]*game.Room{
				"my-room":    {ZoneId: "my-zone"},
				"other-room": {ZoneId: "other-zone"},
			},
			spawnedMobs: []struct{ mobileId, zoneId, roomId string }{
				{"guard", "other-zone", "other-room"},
			},
			actorZone:  "my-zone",
			actorRoom:  "my-room",
			name:       "guard",
			scope:      ScopeWorld,
			expMobName: "a burly guard",
		},
		"mob not found": {
			mobiles:   map[string]*game.Mobile{},
			rooms:     map[string]*game.Room{},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "nobody",
			scope:     ScopeRoom,
			expErr:    `Mobile "nobody" not found.`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			world := game.NewWorldState(
				nil,
				&mockCharStore{chars: map[string]*game.Character{}},
				&mockZoneStore{zones: map[string]*game.Zone{}},
				&mockRoomStore{rooms: tt.rooms},
				&mockMobileStore{mobiles: tt.mobiles},
				&mockObjectStore{objects: map[string]*game.Object{}},
				&mockRaceStore{races: map[string]*game.Race{}},
			)

			// Add actor
			actorChan := make(chan []byte, 1)
			_ = world.AddPlayer("actor", actorChan, tt.actorZone, tt.actorRoom)

			// Spawn mobs
			for _, spawn := range tt.spawnedMobs {
				world.SpawnMobileInstance(
					storage.Identifier(spawn.mobileId),
					storage.Identifier(spawn.zoneId),
					storage.Identifier(spawn.roomId),
				)
			}

			resolver := NewResolver(world)
			result, err := resolver.resolveMob("actor", tt.name, tt.scope)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if result.Name != tt.expMobName {
				t.Errorf("Name = %q, expected %q", result.Name, tt.expMobName)
			}
		})
	}
}

func TestResolver_ResolveObject(t *testing.T) {
	tests := map[string]struct {
		objects       map[string]*game.Object
		rooms         map[string]*game.Room
		zones         map[string]*game.Zone
		actorZone     storage.Identifier
		actorRoom     storage.Identifier
		scopeContents *game.Inventory
		name          string
		scope         Scope
		expObjName    string
		expErr        string
	}{
		"room scope finds object in room": {
			objects: map[string]*game.Object{
				"sword": {Aliases: []string{"sword"}, ShortDesc: "a rusty sword"},
			},
			rooms: map[string]*game.Room{
				"my-room": {ZoneId: "my-zone", ObjSpawns: []game.ObjectSpawn{{ObjectId: "sword"}}},
			},
			zones: map[string]*game.Zone{
				"my-zone": {ResetMode: "never"},
			},
			actorZone:  "my-zone",
			actorRoom:  "my-room",
			name:       "sword",
			scope:      ScopeRoom,
			expObjName: "a rusty sword",
		},
		"room scope rejects object in different room": {
			objects: map[string]*game.Object{
				"sword": {Aliases: []string{"sword"}, ShortDesc: "a rusty sword"},
			},
			rooms: map[string]*game.Room{
				"my-room":    {ZoneId: "my-zone"},
				"other-room": {ZoneId: "my-zone", ObjSpawns: []game.ObjectSpawn{{ObjectId: "sword"}}},
			},
			zones: map[string]*game.Zone{
				"my-zone": {ResetMode: "never"},
			},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "sword",
			scope:     ScopeRoom,
			expErr:    `Object "sword" not found.`,
		},
		"contents scope finds object in container": {
			objects: map[string]*game.Object{
				"torch": {Aliases: []string{"torch"}, ShortDesc: "a lit torch"},
			},
			rooms: map[string]*game.Room{},
			zones: map[string]*game.Zone{},
			scopeContents: &game.Inventory{
				Items: map[string]*game.ObjectInstance{
					"torch-1": {InstanceId: "torch-1", ObjectId: "torch"},
				},
			},
			actorZone:  "my-zone",
			actorRoom:  "my-room",
			name:       "torch",
			scope:      ScopeContents,
			expObjName: "a lit torch",
		},
		"contents scope with nil inventory finds nothing": {
			objects: map[string]*game.Object{
				"torch": {Aliases: []string{"torch"}, ShortDesc: "a lit torch"},
			},
			rooms:         map[string]*game.Room{},
			zones:         map[string]*game.Zone{},
			scopeContents: nil,
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "torch",
			scope:         ScopeContents,
			expErr:        `Object "torch" not found.`,
		},
		"contents scope does not fall through to room": {
			objects: map[string]*game.Object{
				"torch": {Aliases: []string{"torch"}, ShortDesc: "a lit torch"},
			},
			rooms: map[string]*game.Room{
				"my-room": {ZoneId: "my-zone", ObjSpawns: []game.ObjectSpawn{{ObjectId: "torch"}}},
			},
			zones: map[string]*game.Zone{
				"my-zone": {ResetMode: "never"},
			},
			scopeContents: &game.Inventory{
				Items: map[string]*game.ObjectInstance{},
			},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "torch",
			scope:     ScopeContents, // Only contents, not room
			expErr:    `Object "torch" not found.`,
		},
		"inventory scope finds object in actor inventory": {
			objects: map[string]*game.Object{
				"ring": {Aliases: []string{"ring"}, ShortDesc: "a gold ring"},
			},
			rooms:      map[string]*game.Room{},
			zones:      map[string]*game.Zone{},
			actorZone:  "my-zone",
			actorRoom:  "my-room",
			name:       "ring",
			scope:      ScopeInventory,
			expObjName: "a gold ring",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up actor with inventory for inventory scope test
			chars := map[string]*game.Character{
				"actor": {Name: "Actor"},
			}
			if tt.scope&ScopeInventory != 0 {
				inv := game.NewInventory()
				// Add objects matching the test to the actor's inventory
				for instId, oi := range func() map[string]*game.ObjectInstance {
					m := map[string]*game.ObjectInstance{}
					for id, obj := range tt.objects {
						_ = obj
						inst := &game.ObjectInstance{InstanceId: id + "-1", ObjectId: storage.Identifier(id)}
						m[inst.InstanceId] = inst
					}
					return m
				}() {
					_ = instId
					inv.Add(oi)
				}
				chars["actor"] = &game.Character{Name: "Actor", ActorInstance: game.ActorInstance{Inventory: inv}}
			}

			world := game.NewWorldState(
				nil,
				&mockCharStore{chars: chars},
				&mockZoneStore{zones: func() map[string]*game.Zone {
					if tt.zones != nil {
						return tt.zones
					}
					return map[string]*game.Zone{}
				}()},
				&mockRoomStore{rooms: tt.rooms},
				&mockMobileStore{mobiles: map[string]*game.Mobile{}},
				&mockObjectStore{objects: tt.objects},
				&mockRaceStore{races: map[string]*game.Race{}},
			)

			// Add actor
			actorChan := make(chan []byte, 1)
			_ = world.AddPlayer("actor", actorChan, tt.actorZone, tt.actorRoom)

			// Spawn room objects via zone reset
			for zoneId := range tt.zones {
				world.ResetZone(storage.Identifier(zoneId), true)
			}

			resolver := NewResolver(world)
			resolver.scopeContents = tt.scopeContents
			result, err := resolver.resolveObject("actor", tt.name, tt.scope)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if result.Name != tt.expObjName {
				t.Errorf("Name = %q, expected %q", result.Name, tt.expObjName)
			}
		})
	}
}

func TestResolver_ResolveTarget(t *testing.T) {
	tests := map[string]struct {
		chars         map[string]*game.Character
		onlinePlayers map[string]struct{ zone, room storage.Identifier }
		mobiles       map[string]*game.Mobile
		rooms         map[string]*game.Room
		spawnedMobs   []struct{ mobileId, zoneId, roomId string }
		actorZone     storage.Identifier
		actorRoom     storage.Identifier
		name          string
		scope         Scope
		expType       string
		expErr        string
	}{
		"resolves player as target": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"my-zone", "my-room"},
			},
			rooms:     map[string]*game.Room{},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "bob",
			scope:     ScopeWorld,
			expType:   "player",
		},
		"resolves mob as target when no player found": {
			chars:         map[string]*game.Character{},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{},
			mobiles: map[string]*game.Mobile{
				"guard": {Aliases: []string{"guard"}, ShortDesc: "a burly guard"},
			},
			rooms: map[string]*game.Room{
				"my-room": {ZoneId: "my-zone"},
			},
			spawnedMobs: []struct{ mobileId, zoneId, roomId string }{
				{"guard", "my-zone", "my-room"},
			},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "guard",
			scope:     ScopeRoom,
			expType:   "mobile",
		},
		"prefers player over mob with same name": {
			chars: map[string]*game.Character{
				"guard": {Name: "Guard"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"guard": {"my-zone", "my-room"},
			},
			mobiles: map[string]*game.Mobile{
				"guard-mob": {Aliases: []string{"guard"}, ShortDesc: "a burly guard"},
			},
			rooms: map[string]*game.Room{
				"my-room": {ZoneId: "my-zone"},
			},
			spawnedMobs: []struct{ mobileId, zoneId, roomId string }{
				{"guard-mob", "my-zone", "my-room"},
			},
			actorZone: "my-zone",
			actorRoom: "my-room",
			name:      "guard",
			scope:     ScopeRoom,
			expType:   "player",
		},
		"target not found": {
			chars:         map[string]*game.Character{},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{},
			rooms:         map[string]*game.Room{},
			actorZone:     "my-zone",
			actorRoom:     "my-room",
			name:          "nobody",
			scope:         ScopeWorld,
			expErr:        `Target "nobody" not found.`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			charStore := &mockCharStore{chars: tt.chars}
			mobileStore := &mockMobileStore{mobiles: tt.mobiles}
			if mobileStore.mobiles == nil {
				mobileStore.mobiles = map[string]*game.Mobile{}
			}
			roomStore := &mockRoomStore{rooms: tt.rooms}
			if roomStore.rooms == nil {
				roomStore.rooms = map[string]*game.Room{}
			}

			world := game.NewWorldState(nil, charStore, &mockZoneStore{zones: map[string]*game.Zone{}}, roomStore, mobileStore, &mockObjectStore{objects: map[string]*game.Object{}}, &mockRaceStore{races: map[string]*game.Race{}})

			// Add actor
			actorChan := make(chan []byte, 1)
			_ = world.AddPlayer("actor", actorChan, tt.actorZone, tt.actorRoom)

			// Add other players
			for charId, loc := range tt.onlinePlayers {
				ch := make(chan []byte, 1)
				_ = world.AddPlayer(storage.Identifier(charId), ch, loc.zone, loc.room)
			}

			// Spawn mobs
			for _, spawn := range tt.spawnedMobs {
				//TODO: This is a sign we need an interface for world
				world.SpawnMobileInstance(
					storage.Identifier(spawn.mobileId),
					storage.Identifier(spawn.zoneId),
					storage.Identifier(spawn.roomId),
				)
			}

			resolver := NewResolver(world)
			result, err := resolver.resolveTarget("actor", tt.name, tt.scope)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if result.Type != tt.expType {
				t.Errorf("Type = %q, expected %q", result.Type, tt.expType)
			}
		})
	}
}
