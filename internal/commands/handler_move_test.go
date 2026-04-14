package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// newDarkTestRoom creates a room whose asset spec has the `dark` grant perk.
func newDarkTestRoom(t *testing.T, id, name, zoneId string) *game.RoomInstance {
	t.Helper()
	zone := &assets.Zone{ResetMode: assets.ZoneResetNever}
	room := &assets.Room{
		Name:  name,
		Zone:  storage.NewResolvedSmartIdentifier(zoneId, zone),
		Perks: []assets.Perk{{Type: assets.PerkTypeGrant, Key: assets.PerkGrantDark}},
	}
	ri, err := game.NewRoomInstance(storage.NewResolvedSmartIdentifier(id, room))
	if err != nil {
		t.Fatalf("newDarkTestRoom: %v", err)
	}
	return ri
}

func TestAnnounceToRoomDarkness(t *testing.T) {
	tests := map[string]struct {
		observerDarkvision bool
		expNotified        bool
	}{
		"observer without darkvision doesn't see":   {observerDarkvision: false, expNotified: false},
		"observer with darkvision sees the message": {observerDarkvision: true, expNotified: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room := newDarkTestRoom(t, "dark-room", "Dark Room", "test-zone")

			actor := &mockActor{id: "actor", name: "Actor", room: room}

			msgs := make(chan []byte, 10)
			charRef := storage.NewResolvedSmartIdentifier("observer", &assets.Character{Name: "Observer"})
			observer, err := game.NewCharacterInstance(charRef, msgs, room)
			if err != nil {
				t.Fatalf("NewCharacterInstance: %v", err)
			}
			room.AddPlayer("observer", observer)
			observer.AddSource("room", room.Perks)
			if tc.observerDarkvision {
				dvCache := game.NewPerkCache(
					[]assets.Perk{{Type: assets.PerkTypeGrant, Key: assets.PerkGrantDarkvision}},
					nil,
				)
				observer.AddSource("darkvision", dvCache)
			}

			announceToRoom(room, actor, "Actor has arrived.")

			var got []string
			select {
			case msg := <-msgs:
				got = append(got, string(msg))
			default:
			}

			if tc.expNotified && len(got) == 0 {
				t.Errorf("expected observer to be notified, got nothing")
			}
			if !tc.expNotified && len(got) != 0 {
				t.Errorf("expected observer not to be notified, got %v", got)
			}
		})
	}
}

func TestMoveFollowers(t *testing.T) {
	tests := map[string]struct {
		setup          func(fromRoom *game.RoomInstance) (leader game.Actor, actors map[string]*mockActor)
		expMoved       []string          // ids expected to have moved
		expStayed      []string          // ids expected to have stayed
		expMsgContains map[string]string // id -> expected message substring
	}{
		"follower moves with leader": {
			setup: func(fromRoom *game.RoomInstance) (game.Actor, map[string]*mockActor) {
				leader := &mockActor{id: "leader", name: "Leader", room: fromRoom}
				follower := &mockActor{id: "follower", name: "Follower", room: fromRoom}
				leader.followers = []game.Actor{follower}
				return leader, map[string]*mockActor{"follower": follower}
			},
			expMoved:       []string{"follower"},
			expMsgContains: map[string]string{"follower": "You follow Leader."},
		},
		"follower in combat stays behind": {
			setup: func(fromRoom *game.RoomInstance) (game.Actor, map[string]*mockActor) {
				leader := &mockActor{id: "leader", name: "Leader", room: fromRoom}
				follower := &mockActor{id: "follower", name: "Follower", room: fromRoom, inCombat: true}
				leader.followers = []game.Actor{follower}
				return leader, map[string]*mockActor{"follower": follower}
			},
			expStayed:      []string{"follower"},
			expMsgContains: map[string]string{"follower": "Leader leaves north without you."},
		},
		"recursive following": {
			setup: func(fromRoom *game.RoomInstance) (game.Actor, map[string]*mockActor) {
				leader := &mockActor{id: "leader", name: "Leader", room: fromRoom}
				mid := &mockActor{id: "mid", name: "Mid", room: fromRoom}
				tail := &mockActor{id: "tail", name: "Tail", room: fromRoom}
				leader.followers = []game.Actor{mid}
				mid.followers = []game.Actor{tail}
				return leader, map[string]*mockActor{"mid": mid, "tail": tail}
			},
			expMoved: []string{"mid", "tail"},
			expMsgContains: map[string]string{
				"mid":  "You follow Leader.",
				"tail": "You follow Mid.",
			},
		},
		"follower in different room is skipped": {
			setup: func(fromRoom *game.RoomInstance) (game.Actor, map[string]*mockActor) {
				leader := &mockActor{id: "leader", name: "Leader", room: fromRoom}
				follower := &mockActor{id: "follower", name: "Follower"} // nil room != fromRoom
				leader.followers = []game.Actor{follower}
				return leader, map[string]*mockActor{"follower": follower}
			},
			expStayed: []string{"follower"},
		},
		"subtree pruned when follower in different room": {
			setup: func(fromRoom *game.RoomInstance) (game.Actor, map[string]*mockActor) {
				leader := &mockActor{id: "leader", name: "Leader", room: fromRoom}
				mid := &mockActor{id: "mid", name: "Mid"} // nil room != fromRoom
				tail := &mockActor{id: "tail", name: "Tail", room: fromRoom}
				leader.followers = []game.Actor{mid}
				mid.followers = []game.Actor{tail}
				return leader, map[string]*mockActor{"mid": mid, "tail": tail}
			},
			expStayed: []string{"mid", "tail"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fromRoom, err := newTestRoom("from-room", "From Room", "test-zone")
			if err != nil {
				t.Fatalf("failed to create from room: %v", err)
			}
			toRoom, err := newTestRoom("to-room", "To Room", "test-zone")
			if err != nil {
				t.Fatalf("failed to create to room: %v", err)
			}
			leader, actors := tt.setup(fromRoom)

			moveFollowers(leader, fromRoom, toRoom, "north")

			for _, id := range tt.expMoved {
				if !actors[id].moved {
					t.Errorf("expected %q to have moved", id)
				}
			}
			for _, id := range tt.expStayed {
				if actors[id].moved {
					t.Errorf("expected %q to have stayed", id)
				}
			}
			for id, substr := range tt.expMsgContains {
				found := false
				for _, msg := range actors[id].notified {
					if containsStr(msg, substr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected message to %q containing %q, got %v", id, substr, actors[id].notified)
				}
			}
		})
	}
}
