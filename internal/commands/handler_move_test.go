package commands

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

func newTestRoom(id, name, zoneId string) *game.RoomInstance {
	zone := &game.Zone{ResetMode: game.ZoneResetNever}
	room := &game.Room{
		Name: name,
		Zone: storage.NewResolvedSmartIdentifier(zoneId, zone),
	}
	ri, _ := game.NewRoomInstance(storage.NewResolvedSmartIdentifier(id, room))
	return ri
}

func newTestPlayer(charId, name string, room *game.RoomInstance) *game.PlayerState {
	ps := &game.PlayerState{
		Character: storage.NewResolvedSmartIdentifier(charId, &game.Character{Name: name}),
		ZoneId:    room.Room.Get().Zone.Id(),
		RoomId:    room.Room.Id(),
	}
	room.AddPlayer(charId, ps)
	return ps
}

func TestMoveFollowers(t *testing.T) {
	tests := map[string]struct {
		setup          func(from, to *game.RoomInstance) (leaderId, leaderName string, players map[string]*game.PlayerState)
		expMovedTo     []string          // charIds expected in toRoom after move
		expStayedIn    []string          // charIds expected in fromRoom after move
		expMsgContains map[string]string // charId -> expected message substring
	}{
		"follower moves with leader": {
			setup: func(from, to *game.RoomInstance) (string, string, map[string]*game.PlayerState) {
				leader := newTestPlayer("leader", "Leader", from)
				follower := newTestPlayer("follower", "Follower", from)
				follower.FollowingId = "leader"
				return "leader", "Leader", map[string]*game.PlayerState{"leader": leader, "follower": follower}
			},
			expMovedTo:     []string{"follower"},
			expMsgContains: map[string]string{"follower": "You follow Leader."},
		},
		"follower in combat stays behind": {
			setup: func(from, to *game.RoomInstance) (string, string, map[string]*game.PlayerState) {
				leader := newTestPlayer("leader", "Leader", from)
				follower := newTestPlayer("follower", "Follower", from)
				follower.FollowingId = "leader"
				follower.InCombat = true
				return "leader", "Leader", map[string]*game.PlayerState{"leader": leader, "follower": follower}
			},
			expStayedIn:    []string{"follower"},
			expMsgContains: map[string]string{"follower": "Leader leaves north without you."},
		},
		"recursive following": {
			setup: func(from, to *game.RoomInstance) (string, string, map[string]*game.PlayerState) {
				leader := newTestPlayer("leader", "Leader", from)
				mid := newTestPlayer("mid", "Mid", from)
				mid.FollowingId = "leader"
				tail := newTestPlayer("tail", "Tail", from)
				tail.FollowingId = "mid"
				return "leader", "Leader", map[string]*game.PlayerState{"leader": leader, "mid": mid, "tail": tail}
			},
			expMovedTo: []string{"mid", "tail"},
			expMsgContains: map[string]string{
				"mid":  "You follow Leader.",
				"tail": "You follow Mid.",
			},
		},
		"non-follower stays": {
			setup: func(from, to *game.RoomInstance) (string, string, map[string]*game.PlayerState) {
				leader := newTestPlayer("leader", "Leader", from)
				bystander := newTestPlayer("bystander", "Bystander", from)
				return "leader", "Leader", map[string]*game.PlayerState{"leader": leader, "bystander": bystander}
			},
			expStayedIn: []string{"bystander"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fromRoom := newTestRoom("test-room-1", "Room One", "test-zone")
			toRoom := newTestRoom("test-room-2", "Room Two", "test-zone")
			pub := &recordingPublisher{}
			factory := &MoveHandlerFactory{pub: pub}

			leaderId, leaderName, players := tt.setup(fromRoom, toRoom)

			factory.moveFollowers(leaderId, leaderName, fromRoom, toRoom, "north")

			for _, charId := range tt.expMovedTo {
				ps := players[charId]
				_, roomId := ps.Location()
				if roomId != "test-room-2" {
					t.Errorf("player %q in room %q, expected test-room-2", charId, roomId)
				}
			}

			for _, charId := range tt.expStayedIn {
				ps := players[charId]
				_, roomId := ps.Location()
				if roomId != "test-room-1" {
					t.Errorf("player %q in room %q, expected test-room-1", charId, roomId)
				}
			}

			for charId, expMsg := range tt.expMsgContains {
				msgs := pub.messagesTo(charId)
				found := false
				for _, m := range msgs {
					if strings.Contains(m, expMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected message to %q containing %q, got %v", charId, expMsg, msgs)
				}
			}
		})
	}
}

func TestCanMove(t *testing.T) {
	tests := map[string]struct {
		ps     *game.PlayerState
		expNil bool
	}{
		"can move": {
			ps:     &game.PlayerState{},
			expNil: true,
		},
		"in combat": {
			ps:     &game.PlayerState{InCombat: true},
			expNil: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := canMove(tt.ps)
			if tt.expNil && err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
			if !tt.expNil && err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
