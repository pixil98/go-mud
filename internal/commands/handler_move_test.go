package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/game"
)

func TestMoveFollowers(t *testing.T) {
	tests := map[string]struct {
		setup          func(fromRoom *game.RoomInstance) (leader game.FollowTarget, actors map[string]*mockActor)
		expMoved       []string          // ids expected to have moved
		expStayed      []string          // ids expected to have stayed
		expMsgContains map[string]string // id -> expected message substring
	}{
		"follower moves with leader": {
			setup: func(fromRoom *game.RoomInstance) (game.FollowTarget, map[string]*mockActor) {
				leader := &mockActor{id: "leader", name: "Leader", room: fromRoom}
				follower := &mockActor{id: "follower", name: "Follower", room: fromRoom}
				leader.followers = []game.FollowTarget{follower}
				return leader, map[string]*mockActor{"follower": follower}
			},
			expMoved:       []string{"follower"},
			expMsgContains: map[string]string{"follower": "You follow Leader."},
		},
		"follower in combat stays behind": {
			setup: func(fromRoom *game.RoomInstance) (game.FollowTarget, map[string]*mockActor) {
				leader := &mockActor{id: "leader", name: "Leader", room: fromRoom}
				follower := &mockActor{id: "follower", name: "Follower", room: fromRoom, inCombat: true}
				leader.followers = []game.FollowTarget{follower}
				return leader, map[string]*mockActor{"follower": follower}
			},
			expStayed:      []string{"follower"},
			expMsgContains: map[string]string{"follower": "Leader leaves north without you."},
		},
		"recursive following": {
			setup: func(fromRoom *game.RoomInstance) (game.FollowTarget, map[string]*mockActor) {
				leader := &mockActor{id: "leader", name: "Leader", room: fromRoom}
				mid := &mockActor{id: "mid", name: "Mid", room: fromRoom}
				tail := &mockActor{id: "tail", name: "Tail", room: fromRoom}
				leader.followers = []game.FollowTarget{mid}
				mid.followers = []game.FollowTarget{tail}
				return leader, map[string]*mockActor{"mid": mid, "tail": tail}
			},
			expMoved: []string{"mid", "tail"},
			expMsgContains: map[string]string{
				"mid":  "You follow Leader.",
				"tail": "You follow Mid.",
			},
		},
		"follower in different room is skipped": {
			setup: func(fromRoom *game.RoomInstance) (game.FollowTarget, map[string]*mockActor) {
				leader := &mockActor{id: "leader", name: "Leader", room: fromRoom}
				follower := &mockActor{id: "follower", name: "Follower"} // nil room != fromRoom
				leader.followers = []game.FollowTarget{follower}
				return leader, map[string]*mockActor{"follower": follower}
			},
			expStayed: []string{"follower"},
		},
		"subtree pruned when follower in different room": {
			setup: func(fromRoom *game.RoomInstance) (game.FollowTarget, map[string]*mockActor) {
				leader := &mockActor{id: "leader", name: "Leader", room: fromRoom}
				mid := &mockActor{id: "mid", name: "Mid"} // nil room != fromRoom
				tail := &mockActor{id: "tail", name: "Tail", room: fromRoom}
				leader.followers = []game.FollowTarget{mid}
				mid.followers = []game.FollowTarget{tail}
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
