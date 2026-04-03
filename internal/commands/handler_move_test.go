package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/game"
)

// mockFollowTarget is a test double for game.FollowTarget.
type mockFollowTarget struct {
	id        string
	name      string
	inCombat  bool
	zoneId    string
	roomId    string
	moved     bool
	notified  []string
	following game.FollowTarget
	followers []game.FollowTarget
}

func (m *mockFollowTarget) Id() string                            { return m.id }
func (m *mockFollowTarget) Name() string                          { return m.name }
func (m *mockFollowTarget) IsInCombat() bool                      { return m.inCombat }
func (m *mockFollowTarget) Location() (string, string)            { return m.zoneId, m.roomId }
func (m *mockFollowTarget) Resource(string) (int, int)            { return 0, 0 }
func (m *mockFollowTarget) IsCharacter() bool                     { return true }
func (m *mockFollowTarget) Notify(msg string)                     { m.notified = append(m.notified, msg) }
func (m *mockFollowTarget) Following() game.FollowTarget          { return m.following }
func (m *mockFollowTarget) SetFollowing(ft game.FollowTarget)     { m.following = ft }
func (m *mockFollowTarget) Followers() []game.FollowTarget        { return m.followers }
func (m *mockFollowTarget) AddFollower(ft game.FollowTarget)      { m.followers = append(m.followers, ft) }
func (m *mockFollowTarget) RemoveFollower(string)                 {}
func (m *mockFollowTarget) SetFollowerGrouped(string, bool)       {}
func (m *mockFollowTarget) IsFollowerGrouped(string) bool         { return false }
func (m *mockFollowTarget) GroupedFollowers() []game.FollowTarget { return nil }
func (m *mockFollowTarget) Move(_, to *game.RoomInstance) {
	m.moved = true
	m.roomId = to.Room.Id()
}

func TestMoveFollowers(t *testing.T) {
	tests := map[string]struct {
		setup          func() (leader game.FollowTarget, actors map[string]*mockFollowTarget)
		expMoved       []string          // ids expected to have moved
		expStayed      []string          // ids expected to have stayed
		expMsgContains map[string]string // id -> expected message substring
	}{
		"follower moves with leader": {
			setup: func() (game.FollowTarget, map[string]*mockFollowTarget) {
				leader := &mockFollowTarget{id: "leader", name: "Leader", roomId: "from-room"}
				follower := &mockFollowTarget{id: "follower", name: "Follower", roomId: "from-room"}
				leader.followers = []game.FollowTarget{follower}
				return leader, map[string]*mockFollowTarget{"follower": follower}
			},
			expMoved:       []string{"follower"},
			expMsgContains: map[string]string{"follower": "You follow Leader."},
		},
		"follower in combat stays behind": {
			setup: func() (game.FollowTarget, map[string]*mockFollowTarget) {
				leader := &mockFollowTarget{id: "leader", name: "Leader", roomId: "from-room"}
				follower := &mockFollowTarget{id: "follower", name: "Follower", roomId: "from-room", inCombat: true}
				leader.followers = []game.FollowTarget{follower}
				return leader, map[string]*mockFollowTarget{"follower": follower}
			},
			expStayed:      []string{"follower"},
			expMsgContains: map[string]string{"follower": "Leader leaves north without you."},
		},
		"recursive following": {
			setup: func() (game.FollowTarget, map[string]*mockFollowTarget) {
				leader := &mockFollowTarget{id: "leader", name: "Leader", roomId: "from-room"}
				mid := &mockFollowTarget{id: "mid", name: "Mid", roomId: "from-room"}
				tail := &mockFollowTarget{id: "tail", name: "Tail", roomId: "from-room"}
				leader.followers = []game.FollowTarget{mid}
				mid.followers = []game.FollowTarget{tail}
				return leader, map[string]*mockFollowTarget{"mid": mid, "tail": tail}
			},
			expMoved: []string{"mid", "tail"},
			expMsgContains: map[string]string{
				"mid":  "You follow Leader.",
				"tail": "You follow Mid.",
			},
		},
		"follower in different room is skipped": {
			setup: func() (game.FollowTarget, map[string]*mockFollowTarget) {
				leader := &mockFollowTarget{id: "leader", name: "Leader", roomId: "from-room"}
				follower := &mockFollowTarget{id: "follower", name: "Follower", roomId: "other-room"}
				leader.followers = []game.FollowTarget{follower}
				return leader, map[string]*mockFollowTarget{"follower": follower}
			},
			expStayed: []string{"follower"},
		},
		"subtree pruned when follower in different room": {
			setup: func() (game.FollowTarget, map[string]*mockFollowTarget) {
				leader := &mockFollowTarget{id: "leader", name: "Leader", roomId: "from-room"}
				mid := &mockFollowTarget{id: "mid", name: "Mid", roomId: "other-room"}
				tail := &mockFollowTarget{id: "tail", name: "Tail", roomId: "from-room"}
				leader.followers = []game.FollowTarget{mid}
				mid.followers = []game.FollowTarget{tail}
				return leader, map[string]*mockFollowTarget{"mid": mid, "tail": tail}
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
			leader, actors := tt.setup()

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

			for id, expMsg := range tt.expMsgContains {
				if !containsSubstring(actors[id].notified, expMsg) {
					t.Errorf("expected message to %q containing %q, got %v", id, expMsg, actors[id].notified)
				}
			}
		})
	}
}
