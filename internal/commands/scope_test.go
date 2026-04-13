package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// mobInRoom places a mobile in the room and returns it.
func mobInRoom(t *testing.T, room *game.RoomInstance, id, name string) *game.MobileInstance {
	t.Helper()
	mobSpec := &assets.Mobile{Aliases: []string{name}, ShortDesc: name}
	mi := &game.MobileInstance{
		Mobile: storage.NewResolvedSmartIdentifier(id+"-spec", mobSpec),
		ActorInstance: game.ActorInstance{
			InstanceId: id,
			PerkCache:  *game.NewPerkCache(nil, nil),
		},
	}
	room.AddMob(mi)
	return mi
}

// objInRoom places an object in the room and returns it.
func objInRoom(t *testing.T, room *game.RoomInstance, id, name string) *game.ObjectInstance {
	t.Helper()
	objSpec := &assets.Object{Aliases: []string{name}, ShortDesc: name}
	oi := &game.ObjectInstance{
		InstanceId: id,
		Object:     storage.NewResolvedSmartIdentifier(id+"-spec", objSpec),
	}
	room.AddObj(oi)
	return oi
}

func TestSpacesForDarkRoom(t *testing.T) {
	tests := map[string]struct {
		grants    map[string]bool
		expBlocks bool // true if room lookups should be blocked
	}{
		"no dark grant sees normally": {
			grants:    nil,
			expBlocks: false,
		},
		"dark grant without darkvision is blocked": {
			grants:    map[string]bool{assets.PerkGrantDark: true},
			expBlocks: true,
		},
		"dark grant with darkvision sees normally": {
			grants: map[string]bool{
				assets.PerkGrantDark:       true,
				assets.PerkGrantDarkvision: true,
			},
			expBlocks: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, err := newTestRoom("dark-room", "Dark Room", "test-zone")
			if err != nil {
				t.Fatalf("newTestRoom: %v", err)
			}
			mobInRoom(t, room, "mob-1", "goblin")
			objInRoom(t, room, "obj-1", "sword")
			newTestPlayer("player-1", "Alice", room)

			actor := &mockActor{
				id:     "actor",
				name:   "Actor",
				room:   room,
				grants: tc.grants,
			}

			ws := NewWorldScopes()
			spaces, err := ws.SpacesFor(scopeRoom, actor)
			if err != nil {
				t.Fatalf("SpacesFor: %v", err)
			}
			if len(spaces) != 1 {
				t.Fatalf("expected 1 space, got %d", len(spaces))
			}
			finder := spaces[0].Finder

			gotMob := finder.FindMob("goblin")
			gotObj := finder.FindObj("sword")
			gotPlayer := finder.FindPlayer("Alice")

			if tc.expBlocks {
				if gotMob != nil {
					t.Errorf("expected FindMob blocked in dark, got %v", gotMob)
				}
				if gotObj != nil {
					t.Errorf("expected FindObj blocked in dark, got %v", gotObj)
				}
				if gotPlayer != nil {
					t.Errorf("expected FindPlayer blocked in dark, got %v", gotPlayer)
				}
			} else {
				if gotMob == nil {
					t.Errorf("expected FindMob to succeed, got nil")
				}
				if gotObj == nil {
					t.Errorf("expected FindObj to succeed, got nil")
				}
				if gotPlayer == nil {
					t.Errorf("expected FindPlayer to succeed, got nil")
				}
			}

			// Room remover should still be the room (so drops work in the dark).
			if spaces[0].Remover != room {
				t.Errorf("expected Remover to be room regardless of visibility")
			}
		})
	}
}
