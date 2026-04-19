package game

import (
	"testing"
	"time"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestNewZoneInstance(t *testing.T) {
	tests := map[string]struct {
		zone    storage.SmartIdentifier[*assets.Zone]
		wantErr bool
	}{
		"valid zone no lifespan": {
			zone:    storage.NewResolvedSmartIdentifier("z1", &assets.Zone{ResetMode: assets.ZoneResetNever}),
			wantErr: false,
		},
		"valid zone with lifespan": {
			zone:    storage.NewResolvedSmartIdentifier("z1", &assets.Zone{ResetMode: assets.ZoneResetNever, Lifespan: "10m"}),
			wantErr: false,
		},
		"invalid lifespan returns error": {
			zone:    storage.NewResolvedSmartIdentifier("z1", &assets.Zone{ResetMode: assets.ZoneResetNever, Lifespan: "notaduration"}),
			wantErr: true,
		},
		"unresolved zone returns error": {
			zone:    storage.NewSmartIdentifier[*assets.Zone]("unresolved"),
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi, err := NewZoneInstance(tc.zone, nil)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewZoneInstance: %v", err)
			}
			if zi == nil {
				t.Fatal("NewZoneInstance returned nil")
			}
		})
	}
}

func TestZoneInstance_World(t *testing.T) {
	tests := map[string]struct {
		world *WorldState
	}{
		"nil world returns nil": {world: nil},
		"set world returns it":  {world: &WorldState{}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi, _ := NewZoneInstance(
				storage.NewResolvedSmartIdentifier("z", &assets.Zone{ResetMode: assets.ZoneResetNever}),
				tc.world,
			)
			if got := zi.World(); got != tc.world {
				t.Errorf("World() = %v, want %v", got, tc.world)
			}
		})
	}
}

func TestZoneInstance_AddRoom_GetRoom(t *testing.T) {
	tests := map[string]struct {
		roomId  string
		queryId string
		wantNil bool
	}{
		"added room is findable": {roomId: "r1", queryId: "r1", wantNil: false},
		"missing id returns nil": {roomId: "r1", queryId: "r2", wantNil: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi := newTestZone("z")
			zi.AddRoom(newTestRoom(tc.roomId))

			got := zi.GetRoom(tc.queryId)
			if tc.wantNil {
				if got != nil {
					t.Errorf("GetRoom(%q) = %v, want nil", tc.queryId, got)
				}
				return
			}
			if got == nil {
				t.Errorf("GetRoom(%q) = nil, want room", tc.queryId)
			}
		})
	}
}

func TestZoneInstance_ForEachRoom(t *testing.T) {
	tests := map[string]struct {
		roomCount int
	}{
		"empty zone visits no rooms": {roomCount: 0},
		"two rooms visited twice":    {roomCount: 2},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi := newTestZone("z")
			for i := 0; i < tc.roomCount; i++ {
				zi.AddRoom(newTestRoom(string(rune('a' + i))))
			}
			count := 0
			zi.ForEachRoom(func(string, *RoomInstance) { count++ })
			if count != tc.roomCount {
				t.Errorf("ForEachRoom called %d times, want %d", count, tc.roomCount)
			}
		})
	}
}

func TestZoneInstance_IsOccupied(t *testing.T) {
	tests := map[string]struct {
		addPlayer bool
		want      bool
	}{
		"empty zone is not occupied":   {addPlayer: false, want: false},
		"zone with player is occupied": {addPlayer: true, want: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi := newTestZone("z")
			ri := newTestRoom("r")
			zi.AddRoom(ri)
			if tc.addPlayer {
				ci := newTestCI("p", "player")
				ri.AddPlayer("p", ci)
			}
			if got := zi.IsOccupied(); got != tc.want {
				t.Errorf("IsOccupied() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestZoneInstance_ForEachPlayer(t *testing.T) {
	tests := map[string]struct {
		players int
		want    int
	}{
		"no players visited":       {players: 0, want: 0},
		"two players both visited": {players: 2, want: 2},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi := newTestZone("z")
			ri := newTestRoom("r")
			zi.AddRoom(ri)
			for i := 0; i < tc.players; i++ {
				id := string(rune('a' + i))
				ri.AddPlayer(id, newTestCI(id, "player"))
			}
			count := 0
			zi.ForEachPlayer(func(string, *CharacterInstance) { count++ })
			if count != tc.want {
				t.Errorf("ForEachPlayer called %d times, want %d", count, tc.want)
			}
		})
	}
}

func TestZoneInstance_FindPlayers(t *testing.T) {
	tests := map[string]struct {
		setup     func(*ZoneInstance)
		wantCount int
	}{
		"no players returns empty": {
			setup:     func(*ZoneInstance) {},
			wantCount: 0,
		},
		"matcher selects specific player": {
			setup: func(zi *ZoneInstance) {
				ri := newTestRoom("r")
				zi.AddRoom(ri)
				ri.AddPlayer("target", newTestCI("target", "Target"))
				ri.AddPlayer("other", newTestCI("other", "Other"))
			},
			wantCount: 1,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi := newTestZone("z")
			tc.setup(zi)
			got := zi.FindPlayers(func(ci *CharacterInstance) bool { return ci.Id() == "target" })
			if len(got) != tc.wantCount {
				t.Errorf("FindPlayers count = %d, want %d", len(got), tc.wantCount)
			}
		})
	}
}

func TestZoneInstance_FindMobs(t *testing.T) {
	tests := map[string]struct {
		setup     func(*ZoneInstance)
		wantCount int
	}{
		"no mobs returns empty": {
			setup:     func(*ZoneInstance) {},
			wantCount: 0,
		},
		"matcher selects specific mob": {
			setup: func(zi *ZoneInstance) {
				ri := newTestRoom("r")
				zi.AddRoom(ri)
				ri.AddMob(newTestMI("target", "Target"))
				ri.AddMob(newTestMI("other", "Other"))
			},
			wantCount: 1,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi := newTestZone("z")
			tc.setup(zi)
			got := zi.FindMobs(func(mi *MobileInstance) bool { return mi.Id() == "target" })
			if len(got) != tc.wantCount {
				t.Errorf("FindMobs count = %d, want %d", len(got), tc.wantCount)
			}
		})
	}
}

func TestZoneInstance_FindObjs(t *testing.T) {
	tests := map[string]struct {
		setup     func(*ZoneInstance)
		wantCount int
	}{
		"no objects returns empty": {
			setup:     func(*ZoneInstance) {},
			wantCount: 0,
		},
		"object in room is found": {
			setup: func(zi *ZoneInstance) {
				ri := newTestRoom("r")
				zi.AddRoom(ri)
				ri.AddObj(newTestObj("sword"))
			},
			wantCount: 1,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi := newTestZone("z")
			tc.setup(zi)
			got := zi.FindObjs(func(*ObjectInstance) bool { return true })
			if len(got) != tc.wantCount {
				t.Errorf("FindObjs count = %d, want %d", len(got), tc.wantCount)
			}
		})
	}
}

func TestZoneInstance_FindExit(t *testing.T) {
	tests := map[string]struct {
		query string
	}{
		"any query returns empty":   {query: "north"},
		"empty query returns empty": {query: ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi := newTestZone("z")
			dir, re := zi.FindExit(tc.query)
			if dir != "" || re != nil {
				t.Errorf("FindExit(%q) = (%q, %v), want (\"\", nil)", tc.query, dir, re)
			}
		})
	}
}

func TestZoneInstance_Reset(t *testing.T) {
	makeZone := func(mode string) *ZoneInstance {
		zi, _ := NewZoneInstance(
			storage.NewResolvedSmartIdentifier("z", &assets.Zone{ResetMode: mode, Lifespan: "1h"}),
			nil,
		)
		return zi
	}

	tests := map[string]struct {
		mode      string
		force     bool
		pastReset bool // leave nextReset at zero (before any "now") to trigger reset
		addPlayer bool
		wantReset bool // if true, expect the pre-placed object to be drained from the room
	}{
		"never mode skips without force":            {mode: assets.ZoneResetNever, wantReset: false},
		"lifespan mode with future nextReset skips": {mode: assets.ZoneResetLifespan, pastReset: false, wantReset: false},
		"lifespan mode with past nextReset resets":  {mode: assets.ZoneResetLifespan, pastReset: true, wantReset: true},
		"empty mode occupied skips":                 {mode: assets.ZoneResetEmpty, pastReset: true, addPlayer: true, wantReset: false},
		"empty mode unoccupied resets":              {mode: assets.ZoneResetEmpty, pastReset: true, wantReset: true},
		"force bypasses never mode":                 {mode: assets.ZoneResetNever, force: true, wantReset: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi := makeZone(tc.mode)

			if tc.pastReset {
				// nextReset defaults to zero time which is before any real now,
				// so clock returning time.Now() will always be "past" it.
				zi.clock = time.Now
			} else {
				// Push nextReset far into the future so the time guard skips reset.
				zi.nextReset = time.Now().Add(24 * time.Hour)
				zi.clock = time.Now
			}

			ri := newTestRoom("r")
			zi.AddRoom(ri)
			ri.AddObj(newTestObj("sword")) // sentinel: drained on reset, kept if skipped

			if tc.addPlayer {
				ri.AddPlayer("p", newTestCI("p", "player"))
			}

			if err := zi.Reset(tc.force, nil); err != nil {
				t.Fatalf("Reset: %v", err)
			}

			count := len(ri.FindObjs(func(*ObjectInstance) bool { return true }))
			if tc.wantReset && count != 0 {
				t.Errorf("expected room objects drained after reset, got %d", count)
			}
			if !tc.wantReset && count != 1 {
				t.Errorf("expected 1 object (reset skipped), got %d", count)
			}
		})
	}
}

func TestZoneInstance_Tick(t *testing.T) {
	tests := map[string]struct {
		roomCount int
	}{
		"empty zone ticks without panic": {roomCount: 0},
		"zone with rooms ticks all":      {roomCount: 2},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			zi := newTestZone("z")
			for i := 0; i < tc.roomCount; i++ {
				zi.AddRoom(newTestRoom(string(rune('a' + i))))
			}
			zi.Tick() // must not panic
		})
	}
}
