package commands

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
)

func TestMoveFollowers(t *testing.T) {
	tests := map[string]struct {
		setup          func(from, to *game.RoomInstance) (leaderId, leaderName string, players map[string]*game.CharacterInstance, msgChans map[string]chan []byte)
		expMovedTo     []string          // charIds expected in toRoom after move
		expStayedIn    []string          // charIds expected in fromRoom after move
		expMsgContains map[string]string // charId -> expected message substring
	}{
		"follower moves with leader": {
			setup: func(from, to *game.RoomInstance) (string, string, map[string]*game.CharacterInstance, map[string]chan []byte) {
				leader := newTestPlayer("leader", "Leader", from)
				follower, fMsgs := newTestPlayerWithMsgs("follower", "Follower", from)
				follower.SetFollowingId("leader")
				return "leader", "Leader",
					map[string]*game.CharacterInstance{"leader": leader, "follower": follower},
					map[string]chan []byte{"follower": fMsgs}
			},
			expMovedTo:     []string{"follower"},
			expMsgContains: map[string]string{"follower": "You follow Leader."},
		},
		"follower in combat stays behind": {
			setup: func(from, to *game.RoomInstance) (string, string, map[string]*game.CharacterInstance, map[string]chan []byte) {
				leader := newTestPlayer("leader", "Leader", from)
				follower, fMsgs := newTestPlayerWithMsgs("follower", "Follower", from)
				follower.SetFollowingId("leader")
				follower.SetInCombat(true)
				return "leader", "Leader",
					map[string]*game.CharacterInstance{"leader": leader, "follower": follower},
					map[string]chan []byte{"follower": fMsgs}
			},
			expStayedIn:    []string{"follower"},
			expMsgContains: map[string]string{"follower": "Leader leaves north without you."},
		},
		"recursive following": {
			setup: func(from, to *game.RoomInstance) (string, string, map[string]*game.CharacterInstance, map[string]chan []byte) {
				leader := newTestPlayer("leader", "Leader", from)
				mid, midMsgs := newTestPlayerWithMsgs("mid", "Mid", from)
				mid.SetFollowingId("leader")
				tail, tailMsgs := newTestPlayerWithMsgs("tail", "Tail", from)
				tail.SetFollowingId("mid")
				return "leader", "Leader",
					map[string]*game.CharacterInstance{"leader": leader, "mid": mid, "tail": tail},
					map[string]chan []byte{"mid": midMsgs, "tail": tailMsgs}
			},
			expMovedTo: []string{"mid", "tail"},
			expMsgContains: map[string]string{
				"mid":  "You follow Leader.",
				"tail": "You follow Mid.",
			},
		},
		"non-follower stays": {
			setup: func(from, to *game.RoomInstance) (string, string, map[string]*game.CharacterInstance, map[string]chan []byte) {
				leader := newTestPlayer("leader", "Leader", from)
				bystander := newTestPlayer("bystander", "Bystander", from)
				return "leader", "Leader",
					map[string]*game.CharacterInstance{"leader": leader, "bystander": bystander},
					nil
			},
			expStayedIn: []string{"bystander"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fromRoom, err := newTestRoom("test-room-1", "Room One", "test-zone")
			if err != nil {
				t.Fatalf("failed to create from room: %v", err)
			}
			toRoom, err := newTestRoom("test-room-2", "Room Two", "test-zone")
			if err != nil {
				t.Fatalf("failed to create to room: %v", err)
			}
			factory := &MoveHandlerFactory{}

			leaderId, leaderName, players, msgChans := tt.setup(fromRoom, toRoom)

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
				ch := msgChans[charId]
				if ch == nil {
					t.Fatalf("no msgs channel for %q", charId)
				}
				msgs := drainChan(ch)
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

// drainChan reads all pending messages from a channel.
func drainChan(ch chan []byte) []string {
	var msgs []string
	for {
		select {
		case msg := <-ch:
			msgs = append(msgs, string(msg))
		default:
			return msgs
		}
	}
}

func TestCanMove(t *testing.T) {
	tests := map[string]struct {
		ps     *game.CharacterInstance
		expNil bool
	}{
		"can move": {
			ps: func() *game.CharacterInstance {
				return newCharacterInstance("test", "Test")
			}(),
			expNil: true,
		},
		"in combat": {
			ps: func() *game.CharacterInstance {
				ci := newCharacterInstance("test", "Test")
				ci.SetInCombat(true)
				return ci
			}(),
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
