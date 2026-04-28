package commands

import (
	"slices"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/gametest"
)

func TestGroupLeader(t *testing.T) {
	tests := map[string]struct {
		setup     func() *gametest.BaseActor
		expLeader string // expected leader Id, or "" for nil
	}{
		"not in group": {
			setup: func() *gametest.BaseActor {
				return &gametest.BaseActor{ActorId: "a", ActorName: "A"}
			},
			expLeader: "",
		},
		"is the leader": {
			setup: func() *gametest.BaseActor {
				a := &gametest.BaseActor{ActorId: "a", ActorName: "A", GroupedIds: map[string]bool{}}
				b := &gametest.BaseActor{ActorId: "b", ActorName: "B", ActorFollowing: a}
				a.ActorFollowers = []game.Actor{b}
				a.GroupedIds["b"] = true
				return a
			},
			expLeader: "a",
		},
		"is a member": {
			setup: func() *gametest.BaseActor {
				a := &gametest.BaseActor{ActorId: "a", ActorName: "A", GroupedIds: map[string]bool{}}
				b := &gametest.BaseActor{ActorId: "b", ActorName: "B", ActorFollowing: a}
				a.ActorFollowers = []game.Actor{b}
				a.GroupedIds["b"] = true
				return b
			},
			expLeader: "a",
		},
		"nested: member of sub-leader finds root": {
			setup: func() *gametest.BaseActor {
				a := &gametest.BaseActor{ActorId: "a", ActorName: "A", GroupedIds: map[string]bool{}}
				b := &gametest.BaseActor{ActorId: "b", ActorName: "B", ActorFollowing: a, GroupedIds: map[string]bool{}}
				c := &gametest.BaseActor{ActorId: "c", ActorName: "C", ActorFollowing: b}
				a.ActorFollowers = []game.Actor{b}
				a.GroupedIds["b"] = true
				b.ActorFollowers = []game.Actor{c}
				b.GroupedIds["c"] = true
				return c
			},
			expLeader: "a",
		},
		"nested: sub-leader finds root": {
			setup: func() *gametest.BaseActor {
				a := &gametest.BaseActor{ActorId: "a", ActorName: "A", GroupedIds: map[string]bool{}}
				b := &gametest.BaseActor{ActorId: "b", ActorName: "B", ActorFollowing: a, GroupedIds: map[string]bool{}}
				c := &gametest.BaseActor{ActorId: "c", ActorName: "C", ActorFollowing: b}
				a.ActorFollowers = []game.Actor{b}
				a.GroupedIds["b"] = true
				b.ActorFollowers = []game.Actor{c}
				b.GroupedIds["c"] = true
				return b
			},
			expLeader: "a",
		},
		"following but not grouped": {
			setup: func() *gametest.BaseActor {
				a := &gametest.BaseActor{ActorId: "a", ActorName: "A", GroupedIds: map[string]bool{}}
				b := &gametest.BaseActor{ActorId: "b", ActorName: "B", ActorFollowing: a}
				a.ActorFollowers = []game.Actor{b}
				// b is NOT grouped
				return b
			},
			expLeader: "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			actor := tt.setup()
			leader := game.GroupLeader(actor)

			if tt.expLeader == "" {
				if leader != nil {
					t.Errorf("expected nil leader, got %q", leader.Id())
				}
			} else {
				if leader == nil {
					t.Fatalf("expected leader %q, got nil", tt.expLeader)
				}
				if leader.Id() != tt.expLeader {
					t.Errorf("leader = %q, expected %q", leader.Id(), tt.expLeader)
				}
			}
		})
	}
}

func TestDetachFromFollowTree(t *testing.T) {
	tests := map[string]struct {
		setup    func() (actor *gametest.BaseActor, others []*gametest.BaseActor)
		checkMsg func(t *testing.T, actor *gametest.BaseActor, others []*gametest.BaseActor)
	}{
		"solo actor is a no-op": {
			setup: func() (*gametest.BaseActor, []*gametest.BaseActor) {
				a := &gametest.BaseActor{ActorId: "a", ActorName: "A"}
				return a, nil
			},
			checkMsg: func(t *testing.T, a *gametest.BaseActor, _ []*gametest.BaseActor) {
				if len(a.Published) != 0 {
					t.Errorf("expected no messages, got %v", a.PublishedStrings())
				}
			},
		},
		"follower unfollows leader with messages": {
			setup: func() (*gametest.BaseActor, []*gametest.BaseActor) {
				leader := &gametest.BaseActor{ActorId: "leader", ActorName: "Leader"}
				follower := &gametest.BaseActor{ActorId: "f", ActorName: "Follower", ActorFollowing: leader}
				leader.ActorFollowers = []game.Actor{follower}
				return follower, []*gametest.BaseActor{leader}
			},
			checkMsg: func(t *testing.T, actor *gametest.BaseActor, others []*gametest.BaseActor) {
				leader := others[0]
				if actor.ActorFollowing != nil {
					t.Error("actor should not be following anyone")
				}
				if !slices.Contains(actor.PublishedStrings(), "You stop following Leader.") {
					t.Errorf("actor messages = %v, missing unfollow msg", actor.PublishedStrings())
				}
				if !slices.Contains(leader.PublishedStrings(), "Follower stops following you.") {
					t.Errorf("leader messages = %v, missing unfollow msg", leader.PublishedStrings())
				}
			},
		},
		"leader with followers detaches them with messages": {
			setup: func() (*gametest.BaseActor, []*gametest.BaseActor) {
				leader := &gametest.BaseActor{ActorId: "leader", ActorName: "Leader"}
				f1 := &gametest.BaseActor{ActorId: "f1", ActorName: "F1", ActorFollowing: leader}
				leader.ActorFollowers = []game.Actor{f1}
				return leader, []*gametest.BaseActor{f1}
			},
			checkMsg: func(t *testing.T, _ *gametest.BaseActor, others []*gametest.BaseActor) {
				f1 := others[0]
				if f1.ActorFollowing != nil {
					t.Error("follower should not be following anyone")
				}
				if !slices.Contains(f1.PublishedStrings(), "You stop following Leader.") {
					t.Errorf("follower messages = %v, missing unfollow msg", f1.PublishedStrings())
				}
			},
		},
		"group leader disbands group with messages": {
			setup: func() (*gametest.BaseActor, []*gametest.BaseActor) {
				leader := &gametest.BaseActor{ActorId: "leader", ActorName: "Leader", GroupedIds: map[string]bool{"m": true}}
				member := &gametest.BaseActor{ActorId: "m", ActorName: "Member", ActorFollowing: leader}
				leader.ActorFollowers = []game.Actor{member}
				return leader, []*gametest.BaseActor{member}
			},
			checkMsg: func(t *testing.T, actor *gametest.BaseActor, others []*gametest.BaseActor) {
				member := others[0]
				if member.ActorFollowing != nil {
					t.Error("member should not be following anyone")
				}
				if !slices.Contains(actor.PublishedStrings(), "You have disbanded the group.") {
					t.Errorf("leader messages = %v, missing disband msg", actor.PublishedStrings())
				}
				if !slices.Contains(member.PublishedStrings(), "The group has been disbanded.") {
					t.Errorf("member messages = %v, missing disband msg", member.PublishedStrings())
				}
			},
		},
		"grouped member leaves group with messages": {
			setup: func() (*gametest.BaseActor, []*gametest.BaseActor) {
				leader := &gametest.BaseActor{ActorId: "leader", ActorName: "Leader", Character: true, GroupedIds: map[string]bool{"m1": true, "m2": true}}
				m1 := &gametest.BaseActor{ActorId: "m1", ActorName: "M1", Character: true, ActorFollowing: leader}
				m2 := &gametest.BaseActor{ActorId: "m2", ActorName: "M2", Character: true, ActorFollowing: leader}
				leader.ActorFollowers = []game.Actor{m1, m2}
				return m1, []*gametest.BaseActor{leader, m2}
			},
			checkMsg: func(t *testing.T, actor *gametest.BaseActor, others []*gametest.BaseActor) {
				leader := others[0]
				if actor.ActorFollowing != nil {
					t.Error("member should not be following anyone")
				}
				if !slices.Contains(actor.PublishedStrings(), "You leave the group.") {
					t.Errorf("actor messages = %v, missing leave msg", actor.PublishedStrings())
				}
				if !slices.Contains(leader.PublishedStrings(), "M1 has left the group.") {
					t.Errorf("leader messages = %v, missing leave msg", leader.PublishedStrings())
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actor, others := tc.setup()
			DetachFromFollowTree(actor)
			tc.checkMsg(t, actor, others)
		})
	}
}
