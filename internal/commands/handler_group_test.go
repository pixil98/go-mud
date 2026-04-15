package commands

import (
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
