package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/game"
)

func TestGroupLeader(t *testing.T) {
	tests := map[string]struct {
		setup     func() *mockActor
		expLeader string // expected leader Id, or "" for nil
	}{
		"not in group": {
			setup: func() *mockActor {
				return &mockActor{id: "a", name: "A"}
			},
			expLeader: "",
		},
		"is the leader": {
			setup: func() *mockActor {
				a := &mockActor{id: "a", name: "A", groupedIds: map[string]bool{}}
				b := &mockActor{id: "b", name: "B", following: a}
				a.followers = []game.Actor{b}
				a.groupedIds["b"] = true
				return a
			},
			expLeader: "a",
		},
		"is a member": {
			setup: func() *mockActor {
				a := &mockActor{id: "a", name: "A", groupedIds: map[string]bool{}}
				b := &mockActor{id: "b", name: "B", following: a}
				a.followers = []game.Actor{b}
				a.groupedIds["b"] = true
				return b
			},
			expLeader: "a",
		},
		"nested: member of sub-leader finds root": {
			setup: func() *mockActor {
				a := &mockActor{id: "a", name: "A", groupedIds: map[string]bool{}}
				b := &mockActor{id: "b", name: "B", following: a, groupedIds: map[string]bool{}}
				c := &mockActor{id: "c", name: "C", following: b}
				a.followers = []game.Actor{b}
				a.groupedIds["b"] = true
				b.followers = []game.Actor{c}
				b.groupedIds["c"] = true
				return c
			},
			expLeader: "a",
		},
		"nested: sub-leader finds root": {
			setup: func() *mockActor {
				a := &mockActor{id: "a", name: "A", groupedIds: map[string]bool{}}
				b := &mockActor{id: "b", name: "B", following: a, groupedIds: map[string]bool{}}
				c := &mockActor{id: "c", name: "C", following: b}
				a.followers = []game.Actor{b}
				a.groupedIds["b"] = true
				b.followers = []game.Actor{c}
				b.groupedIds["c"] = true
				return b
			},
			expLeader: "a",
		},
		"following but not grouped": {
			setup: func() *mockActor {
				a := &mockActor{id: "a", name: "A", groupedIds: map[string]bool{}}
				b := &mockActor{id: "b", name: "B", following: a}
				a.followers = []game.Actor{b}
				// b is NOT grouped
				return b
			},
			expLeader: "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			actor := tt.setup()
			leader := groupLeader(actor)

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
