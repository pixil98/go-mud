package game

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// newTestCI creates a minimal CharacterInstance for follow tree tests.
func newTestCI(id, name string) *CharacterInstance {
	charRef := storage.NewResolvedSmartIdentifier(id, &assets.Character{Name: name})
	ci := &CharacterInstance{
		Character: charRef,
		ActorInstance: ActorInstance{
			InstanceId: id,
			PerkCache:  *NewPerkCache(nil, nil),
		},
	}
	ci.self = ci
	return ci
}

// newTestMI creates a minimal MobileInstance for follow tree tests.
func newTestMI(id, name string) *MobileInstance {
	mobRef := storage.NewResolvedSmartIdentifier(id, &assets.Mobile{ShortDesc: name})
	mi := &MobileInstance{
		Mobile: mobRef,
		ActorInstance: ActorInstance{
			InstanceId: id,
			PerkCache:  *NewPerkCache(nil, nil),
		},
	}
	mi.self = mi
	return mi
}

func TestSetFollowing_ManagesReverseLinks(t *testing.T) {
	tests := map[string]struct {
		setup  func() (follower FollowTarget, leader FollowTarget)
		verify func(t *testing.T, follower, leader FollowTarget)
	}{
		"follow adds reverse link": {
			setup: func() (FollowTarget, FollowTarget) {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				a.SetFollowing(b)
				return a, b
			},
			verify: func(t *testing.T, follower, leader FollowTarget) {
				if follower.Following() == nil || follower.Following().Id() != "b" {
					t.Error("follower should be following leader")
				}
				followers := leader.Followers()
				if len(followers) != 1 || followers[0].Id() != "a" {
					t.Errorf("leader should have follower, got %v", followers)
				}
			},
		},
		"unfollow removes reverse link": {
			setup: func() (FollowTarget, FollowTarget) {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				a.SetFollowing(b)
				a.SetFollowing(nil)
				return a, b
			},
			verify: func(t *testing.T, follower, leader FollowTarget) {
				if follower.Following() != nil {
					t.Error("follower should not be following anyone")
				}
				if len(leader.Followers()) != 0 {
					t.Error("leader should have no followers")
				}
			},
		},
		"switch leader moves reverse link": {
			setup: func() (FollowTarget, FollowTarget) {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				a.SetFollowing(b)
				a.SetFollowing(c)
				return a, b
			},
			verify: func(t *testing.T, follower, oldLeader FollowTarget) {
				if follower.Following() == nil || follower.Following().Id() != "c" {
					t.Error("follower should be following new leader")
				}
				if len(oldLeader.Followers()) != 0 {
					t.Error("old leader should have no followers")
				}
			},
		},
		"mob can follow character": {
			setup: func() (FollowTarget, FollowTarget) {
				mi := newTestMI("mob1", "Wolf")
				ci := newTestCI("player1", "Hero")
				mi.SetFollowing(ci)
				return mi, ci
			},
			verify: func(t *testing.T, follower, leader FollowTarget) {
				if follower.Following() == nil || follower.Following().Id() != "player1" {
					t.Error("mob should be following player")
				}
				followers := leader.Followers()
				if len(followers) != 1 || followers[0].Id() != "mob1" {
					t.Errorf("player should have mob follower, got %v", followers)
				}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			follower, leader := tt.setup()
			tt.verify(t, follower, leader)
		})
	}
}

func TestGroupedFollowers(t *testing.T) {
	tests := map[string]struct {
		setup  func() FollowTarget
		expIds []string
	}{
		"no grouped followers": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				return a
			},
			expIds: nil,
		},
		"one grouped follower": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				return a
			},
			expIds: []string{"b"},
		},
		"mixed grouped and ungrouped": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				c.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				return a
			},
			expIds: []string{"b"},
		},
		"ungrouping removes from grouped list": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				a.SetFollowerGrouped("b", false)
				return a
			},
			expIds: nil,
		},
		"nested: sub-leader's grouped followers are separate": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				b.SetFollowerGrouped("c", true)
				// a's direct grouped followers should only be b, not c
				return a
			},
			expIds: []string{"b"},
		},
		"nested: sub-leader has own grouped followers": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				b.SetFollowerGrouped("c", true)
				return b
			},
			expIds: []string{"c"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			leader := tt.setup()
			grouped := leader.GroupedFollowers()

			if len(grouped) != len(tt.expIds) {
				t.Fatalf("expected %d grouped followers, got %d", len(tt.expIds), len(grouped))
			}

			ids := make(map[string]bool)
			for _, ft := range grouped {
				ids[ft.Id()] = true
			}
			for _, id := range tt.expIds {
				if !ids[id] {
					t.Errorf("expected grouped follower %q not found", id)
				}
			}
		})
	}
}

func TestGroupPublishTarget(t *testing.T) {
	tests := map[string]struct {
		setup  func() FollowTarget
		expIds []string
	}{
		"leader with direct grouped followers": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				return a
			},
			expIds: []string{"a", "b"},
		},
		"subgroup: sub-leader grouped followers included": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				b.SetFollowerGrouped("c", true)
				return a
			},
			expIds: []string{"a", "b", "c"},
		},
		"subgroup: ungrouped sub-follower excluded": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				// c is NOT grouped by b
				return a
			},
			expIds: []string{"a", "b"},
		},
		"mob followers skipped in publish": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				mi := newTestMI("mob1", "Wolf")
				mi.SetFollowing(a)
				a.SetFollowerGrouped("mob1", true)
				return a
			},
			expIds: []string{"a"},
		},
		"deep subgroup: three levels": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				d := newTestCI("d", "D")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				b.SetFollowerGrouped("c", true)
				d.SetFollowing(c)
				c.SetFollowerGrouped("d", true)
				return a
			},
			expIds: []string{"a", "b", "c", "d"},
		},
		"subgroup with mob pet: mob skipped, pet owner included": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				pet := newTestMI("pet1", "Wolf")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				pet.SetFollowing(b)
				b.SetFollowerGrouped("pet1", true)
				return a
			},
			// pet is a mob so skipped by publish; a and b are characters
			expIds: []string{"a", "b"},
		},
		"two sub-leaders with own subgroups": {
			setup: func() FollowTarget {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				d := newTestCI("d", "D")
				e := newTestCI("e", "E")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(a)
				a.SetFollowerGrouped("c", true)
				d.SetFollowing(b)
				b.SetFollowerGrouped("d", true)
				e.SetFollowing(c)
				c.SetFollowerGrouped("e", true)
				return a
			},
			expIds: []string{"a", "b", "c", "d", "e"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			leader := tt.setup()
			pg := GroupPublishTarget(leader)

			var gotIds []string
			pg.ForEachPlayer(func(id string, _ *CharacterInstance) {
				gotIds = append(gotIds, id)
			})

			if len(gotIds) != len(tt.expIds) {
				t.Fatalf("expected %d publish targets, got %d: %v", len(tt.expIds), len(gotIds), gotIds)
			}

			idSet := make(map[string]bool)
			for _, id := range gotIds {
				idSet[id] = true
			}
			for _, id := range tt.expIds {
				if !idSet[id] {
					t.Errorf("expected publish target %q not found in %v", id, gotIds)
				}
			}
		})
	}
}

var _ FollowTarget = (*CharacterInstance)(nil)
var _ FollowTarget = (*MobileInstance)(nil)
