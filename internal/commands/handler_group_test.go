package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// mockGroupActor satisfies shared.Actor for group handler tests.
type mockGroupActor struct {
	id         string
	name       string
	notified   []string
	following  game.FollowTarget
	followers  []game.FollowTarget
	groupedIds map[string]bool
}

func (m *mockGroupActor) Id() string                               { return m.id }
func (m *mockGroupActor) Name() string                             { return m.name }
func (m *mockGroupActor) Notify(msg string)                        { m.notified = append(m.notified, msg) }
func (m *mockGroupActor) Location() (string, string)               { return "", "" }
func (m *mockGroupActor) IsInCombat() bool                         { return false }
func (m *mockGroupActor) IsAlive() bool                            { return true }
func (m *mockGroupActor) Level() int                               { return 1 }
func (m *mockGroupActor) Resource(string) (int, int)               { return 0, 0 }
func (m *mockGroupActor) AdjustResource(string, int, bool)         {}
func (m *mockGroupActor) SpendAP(int) bool                         { return true }
func (m *mockGroupActor) HasGrant(string, string) bool             { return false }
func (m *mockGroupActor) ModifierValue(string) int                 { return 0 }
func (m *mockGroupActor) GrantArgs(string) []string                { return nil }
func (m *mockGroupActor) AddTimedPerks(string, []assets.Perk, int) {}
func (m *mockGroupActor) SetInCombat(bool)                         {}
func (m *mockGroupActor) CombatTargetId() string                   { return "" }
func (m *mockGroupActor) SetCombatTargetId(string)                 {}
func (m *mockGroupActor) OnDeath() []*game.ObjectInstance          { return nil }
func (m *mockGroupActor) IsCharacter() bool                        { return true }
func (m *mockGroupActor) Inventory() *game.Inventory               { return nil }
func (m *mockGroupActor) Following() game.FollowTarget             { return m.following }
func (m *mockGroupActor) SetFollowing(ft game.FollowTarget)        { m.following = ft }
func (m *mockGroupActor) Followers() []game.FollowTarget           { return m.followers }
func (m *mockGroupActor) AddFollower(ft game.FollowTarget)         { m.followers = append(m.followers, ft) }
func (m *mockGroupActor) RemoveFollower(string)                    {}
func (m *mockGroupActor) Move(_, _ *game.RoomInstance)             {}

func (m *mockGroupActor) SetFollowerGrouped(id string, grouped bool) {
	if m.groupedIds == nil {
		m.groupedIds = make(map[string]bool)
	}
	if grouped {
		m.groupedIds[id] = true
	} else {
		delete(m.groupedIds, id)
	}
}

func (m *mockGroupActor) IsFollowerGrouped(id string) bool {
	return m.groupedIds[id]
}

func (m *mockGroupActor) GroupedFollowers() []game.FollowTarget {
	var out []game.FollowTarget
	for _, ft := range m.followers {
		if m.groupedIds[ft.Id()] {
			out = append(out, ft)
		}
	}
	return out
}

func TestGroupLeader(t *testing.T) {
	tests := map[string]struct {
		setup     func() *mockGroupActor
		expLeader string // expected leader Id, or "" for nil
	}{
		"not in group": {
			setup: func() *mockGroupActor {
				return &mockGroupActor{id: "a", name: "A"}
			},
			expLeader: "",
		},
		"is the leader": {
			setup: func() *mockGroupActor {
				a := &mockGroupActor{id: "a", name: "A", groupedIds: map[string]bool{}}
				b := &mockGroupActor{id: "b", name: "B", following: a}
				a.followers = []game.FollowTarget{b}
				a.groupedIds["b"] = true
				return a
			},
			expLeader: "a",
		},
		"is a member": {
			setup: func() *mockGroupActor {
				a := &mockGroupActor{id: "a", name: "A", groupedIds: map[string]bool{}}
				b := &mockGroupActor{id: "b", name: "B", following: a}
				a.followers = []game.FollowTarget{b}
				a.groupedIds["b"] = true
				return b
			},
			expLeader: "a",
		},
		"nested: member of sub-leader finds root": {
			setup: func() *mockGroupActor {
				a := &mockGroupActor{id: "a", name: "A", groupedIds: map[string]bool{}}
				b := &mockGroupActor{id: "b", name: "B", following: a, groupedIds: map[string]bool{}}
				c := &mockGroupActor{id: "c", name: "C", following: b}
				a.followers = []game.FollowTarget{b}
				a.groupedIds["b"] = true
				b.followers = []game.FollowTarget{c}
				b.groupedIds["c"] = true
				return c
			},
			expLeader: "a",
		},
		"nested: sub-leader finds root": {
			setup: func() *mockGroupActor {
				a := &mockGroupActor{id: "a", name: "A", groupedIds: map[string]bool{}}
				b := &mockGroupActor{id: "b", name: "B", following: a, groupedIds: map[string]bool{}}
				c := &mockGroupActor{id: "c", name: "C", following: b}
				a.followers = []game.FollowTarget{b}
				a.groupedIds["b"] = true
				b.followers = []game.FollowTarget{c}
				b.groupedIds["c"] = true
				return b
			},
			expLeader: "a",
		},
		"following but not grouped": {
			setup: func() *mockGroupActor {
				a := &mockGroupActor{id: "a", name: "A", groupedIds: map[string]bool{}}
				b := &mockGroupActor{id: "b", name: "B", following: a}
				a.followers = []game.FollowTarget{b}
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
