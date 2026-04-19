package game

import (
	"context"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestNewWorldState(t *testing.T) {
	tests := map[string]struct {
		zones   map[string]*assets.Zone
		rooms   map[string]*assets.Room
		wantErr bool
	}{
		"empty stores creates empty world": {
			zones: nil,
			rooms: nil,
		},
		"zone and room are wired together": {
			zones: map[string]*assets.Zone{
				"z1": {ResetMode: assets.ZoneResetNever},
			},
			rooms: map[string]*assets.Room{
				"r1": {
					Name: "Hall",
					Zone: storage.NewResolvedSmartIdentifier("z1", &assets.Zone{ResetMode: assets.ZoneResetNever}),
				},
			},
		},
		"invalid zone lifespan returns error": {
			zones: map[string]*assets.Zone{
				"bad": {ResetMode: assets.ZoneResetNever, Lifespan: "notaduration"},
			},
			rooms:   nil,
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w, err := NewWorldState(
				&fakeSubscriber{},
				newFakeStore[*assets.Zone](tc.zones),
				newFakeStore[*assets.Room](tc.rooms),
			)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewWorldState: %v", err)
			}
			if w == nil {
				t.Fatal("NewWorldState returned nil")
			}
		})
	}
}

func TestWorldState_Perks(t *testing.T) {
	tests := map[string]struct {
		name string
	}{
		"returns non-nil perk cache": {},
	}
	for name := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, _ := newTestWorld()
			if w.Perks() == nil {
				t.Error("Perks() returned nil")
			}
		})
	}
}

func TestWorldState_Instances(t *testing.T) {
	tests := map[string]struct {
		name string
	}{
		"returns zones map with expected zone": {},
	}
	for name := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, _ := newTestWorld()
			instances := w.Instances()
			if _, ok := instances["z1"]; !ok {
				t.Error("Instances() missing expected zone z1")
			}
		})
	}
}

func TestWorldState_GetZone(t *testing.T) {
	tests := map[string]struct {
		zoneId  string
		wantNil bool
	}{
		"existing zone returns instance": {zoneId: "z1", wantNil: false},
		"missing zone returns nil":       {zoneId: "nope", wantNil: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, _ := newTestWorld()
			got := w.GetZone(tc.zoneId)
			if tc.wantNil && got != nil {
				t.Errorf("GetZone(%q) = %v, want nil", tc.zoneId, got)
			}
			if !tc.wantNil && got == nil {
				t.Errorf("GetZone(%q) = nil, want zone", tc.zoneId)
			}
		})
	}
}

func TestWorldState_GetPlayer(t *testing.T) {
	tests := map[string]struct {
		addPlayer bool
		wantNil   bool
	}{
		"missing player returns nil":  {addPlayer: false, wantNil: true},
		"added player is retrievable": {addPlayer: true, wantNil: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, ri := newTestWorld()
			if tc.addPlayer {
				char := storage.NewResolvedSmartIdentifier("p1", &assets.Character{Name: "Player"})
				ci, _ := NewCharacterInstance(char, nil, ri)
				if err := w.AddPlayer(ci); err != nil {
					t.Fatalf("AddPlayer: %v", err)
				}
			}
			got := w.GetPlayer("p1")
			if tc.wantNil && got != nil {
				t.Error("GetPlayer returned non-nil, want nil")
			}
			if !tc.wantNil && got == nil {
				t.Error("GetPlayer returned nil, want player")
			}
		})
	}
}

func TestWorldState_AddPlayer(t *testing.T) {
	tests := map[string]struct {
		addTwice bool
		wantErr  error
	}{
		"first add succeeds":          {addTwice: false, wantErr: nil},
		"duplicate add returns error": {addTwice: true, wantErr: ErrPlayerExists},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, ri := newTestWorld()
			char := storage.NewResolvedSmartIdentifier("p1", &assets.Character{Name: "Player"})
			ci, _ := NewCharacterInstance(char, nil, ri)

			err := w.AddPlayer(ci)
			if err != nil {
				t.Fatalf("first AddPlayer: %v", err)
			}

			if tc.addTwice {
				err = w.AddPlayer(ci)
				if err != tc.wantErr {
					t.Errorf("second AddPlayer error = %v, want %v", err, tc.wantErr)
				}
			}
		})
	}
}

func TestWorldState_RemovePlayer(t *testing.T) {
	tests := map[string]struct {
		addFirst bool
		wantErr  error
	}{
		"remove existing player succeeds": {addFirst: true, wantErr: nil},
		"remove missing player errors":    {addFirst: false, wantErr: ErrPlayerNotFound},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, ri := newTestWorld()
			if tc.addFirst {
				char := storage.NewResolvedSmartIdentifier("p1", &assets.Character{Name: "Player"})
				ci, _ := NewCharacterInstance(char, nil, ri)
				if err := w.AddPlayer(ci); err != nil {
					t.Fatalf("AddPlayer: %v", err)
				}
			}

			err := w.RemovePlayer("p1")
			if err != tc.wantErr {
				t.Errorf("RemovePlayer error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestWorldState_SetPlayerQuit(t *testing.T) {
	tests := map[string]struct {
		addFirst bool
		wantErr  error
		wantQuit bool
	}{
		"sets quit on existing player": {addFirst: true, wantErr: nil, wantQuit: true},
		"missing player returns error": {addFirst: false, wantErr: ErrPlayerNotFound},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, ri := newTestWorld()
			var ci *CharacterInstance
			if tc.addFirst {
				char := storage.NewResolvedSmartIdentifier("p1", &assets.Character{Name: "Player"})
				ci, _ = NewCharacterInstance(char, nil, ri)
				if err := w.AddPlayer(ci); err != nil {
					t.Fatalf("AddPlayer: %v", err)
				}
			}

			err := w.SetPlayerQuit("p1", true)
			if err != tc.wantErr {
				t.Errorf("SetPlayerQuit error = %v, want %v", err, tc.wantErr)
			}
			if tc.addFirst && ci.IsQuit() != tc.wantQuit {
				t.Errorf("IsQuit() = %v, want %v", ci.IsQuit(), tc.wantQuit)
			}
		})
	}
}

func TestWorldState_MarkPlayerActive(t *testing.T) {
	tests := map[string]struct {
		addFirst bool
	}{
		"missing player is a no-op":   {addFirst: false},
		"existing player is refreshed": {addFirst: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, ri := newTestWorld()
			if tc.addFirst {
				char := storage.NewResolvedSmartIdentifier("p1", &assets.Character{Name: "Player"})
				ci, _ := NewCharacterInstance(char, nil, ri)
				if err := w.AddPlayer(ci); err != nil {
					t.Fatalf("AddPlayer: %v", err)
				}
			}
			w.MarkPlayerActive("p1") // must not panic
		})
	}
}

func TestWorldState_ForEachPlayer(t *testing.T) {
	tests := map[string]struct {
		playerCount int
	}{
		"no players visits none":       {playerCount: 0},
		"two players both visited":     {playerCount: 2},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, ri := newTestWorld()
			for i := 0; i < tc.playerCount; i++ {
				id := string(rune('a' + i))
				char := storage.NewResolvedSmartIdentifier(id, &assets.Character{Name: id})
				ci, _ := NewCharacterInstance(char, nil, ri)
				if err := w.AddPlayer(ci); err != nil {
					t.Fatalf("AddPlayer: %v", err)
				}
			}
			count := 0
			w.ForEachPlayer(func(string, *CharacterInstance) { count++ })
			if count != tc.playerCount {
				t.Errorf("ForEachPlayer called %d times, want %d", count, tc.playerCount)
			}
		})
	}
}

func TestWorldState_SpawnMob(t *testing.T) {
	tests := map[string]struct {
		withFollow bool
	}{
		"mob spawned into room":            {withFollow: false},
		"mob spawned with follower target": {withFollow: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, ri := newTestWorld()
			mob := storage.NewResolvedSmartIdentifier("m1", &assets.Mobile{ShortDesc: "goblin"})

			var follow Actor
			if tc.withFollow {
				follower := newTestMI("leader", "leader")
				follow = follower
			}

			mi, err := w.SpawnMob(mob, ri, follow)
			if err != nil {
				t.Fatalf("SpawnMob: %v", err)
			}
			if mi == nil {
				t.Fatal("SpawnMob returned nil")
			}
			found := ri.FindMobs(func(m *MobileInstance) bool { return m.Id() == mi.Id() })
			if len(found) == 0 {
				t.Error("spawned mob not found in room")
			}
			if tc.withFollow && mi.Following() == nil {
				t.Error("mob should be following leader but Following() is nil")
			}
		})
	}
}

func TestWorldState_ResetAll(t *testing.T) {
	tests := map[string]struct {
		name string
	}{
		"resets all zones without error": {},
	}
	for name := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, _ := newTestWorld()
			if err := w.ResetAll(); err != nil {
				t.Errorf("ResetAll: %v", err)
			}
		})
	}
}

func TestWorldState_Tick(t *testing.T) {
	ctx := context.Background()
	tests := map[string]struct {
		addPlayer bool
	}{
		"empty world ticks without error":    {addPlayer: false},
		"world with player ticks without error": {addPlayer: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w, _, ri := newTestWorld()
			if tc.addPlayer {
				char := storage.NewResolvedSmartIdentifier("p1", &assets.Character{Name: "Player"})
				ci, _ := NewCharacterInstance(char, make(chan []byte, 10), ri)
				if err := w.AddPlayer(ci); err != nil {
					t.Fatalf("AddPlayer: %v", err)
				}
			}
			if err := w.Tick(ctx); err != nil {
				t.Errorf("Tick: %v", err)
			}
		})
	}
}
