package game

import "testing"

func TestGroupLeader(t *testing.T) {
	tests := map[string]struct {
		setup  func() Actor
		wantId string // empty = expect nil
	}{
		"solo actor with no followers returns nil": {
			setup:  func() Actor { return newTestCI("a", "A") },
			wantId: "",
		},
		"solo actor with grouped followers returns self": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				return a
			},
			wantId: "a",
		},
		"actor following but not grouped returns nil": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				a.SetFollowing(b) // b does not group a
				return a
			},
			wantId: "",
		},
		"grouped follower returns leader": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				return b
			},
			wantId: "a",
		},
		"nested group: deepest member returns top leader": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				b.SetFollowerGrouped("c", true)
				return c
			},
			wantId: "a",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := GroupLeader(tc.setup())
			if tc.wantId == "" {
				if got != nil {
					t.Errorf("expected nil, got %q", got.Id())
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %q, got nil", tc.wantId)
			}
			if got.Id() != tc.wantId {
				t.Errorf("GroupLeader Id = %q, want %q", got.Id(), tc.wantId)
			}
		})
	}
}

func TestWalkGroup(t *testing.T) {
	tests := map[string]struct {
		setup   func() Actor
		wantIds []string
	}{
		"solo actor visits just itself": {
			setup:   func() Actor { return newTestCI("a", "A") },
			wantIds: []string{"a"},
		},
		"leader with grouped followers visits all": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				return a
			},
			wantIds: []string{"a", "b"},
		},
		"non-grouped follower is skipped": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a) // not grouped
				return a
			},
			wantIds: []string{"a"},
		},
		"nested group visits all grouped descendants": {
			setup: func() Actor {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				c.SetFollowing(b)
				b.SetFollowerGrouped("c", true)
				return a
			},
			wantIds: []string{"a", "b", "c"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var visited []string
			WalkGroup(tc.setup(), func(a Actor) {
				visited = append(visited, a.Id())
			})
			if len(visited) != len(tc.wantIds) {
				t.Fatalf("WalkGroup visited %v, want %v", visited, tc.wantIds)
			}
			idSet := make(map[string]bool)
			for _, id := range visited {
				idSet[id] = true
			}
			for _, id := range tc.wantIds {
				if !idSet[id] {
					t.Errorf("expected %q in visited set %v", id, visited)
				}
			}
		})
	}
}

func TestAreAllies(t *testing.T) {
	tests := map[string]struct {
		setup      func() (Actor, Actor)
		wantAllies bool
	}{
		"actor is always own ally": {
			setup: func() (Actor, Actor) {
				a := newTestCI("a", "A")
				return a, a
			},
			wantAllies: true,
		},
		"two actors in same group are allies": {
			setup: func() (Actor, Actor) {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				return a, b
			},
			wantAllies: true,
		},
		"two solo actors are not allies": {
			setup: func() (Actor, Actor) {
				return newTestCI("a", "A"), newTestCI("b", "B")
			},
			wantAllies: false,
		},
		"actors in different groups are not allies": {
			setup: func() (Actor, Actor) {
				a := newTestCI("a", "A")
				b := newTestCI("b", "B")
				c := newTestCI("c", "C")
				d := newTestCI("d", "D")
				b.SetFollowing(a)
				a.SetFollowerGrouped("b", true)
				d.SetFollowing(c)
				c.SetFollowerGrouped("d", true)
				return b, d
			},
			wantAllies: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			a, b := tc.setup()
			if got := AreAllies(a, b); got != tc.wantAllies {
				t.Errorf("AreAllies = %v, want %v", got, tc.wantAllies)
			}
		})
	}
}

func TestIsPlayerSide(t *testing.T) {
	tests := map[string]struct {
		setup func() Actor
		want  bool
	}{
		"character is player side": {
			setup: func() Actor { return newTestCI("a", "A") },
			want:  true,
		},
		"mob with no follow chain is mob side": {
			setup: func() Actor { return newTestMI("m", "Mob") },
			want:  false,
		},
		"mob following character is player side": {
			setup: func() Actor {
				ci := newTestCI("a", "A")
				mi := newTestMI("m", "Mob")
				mi.SetFollowing(ci)
				return mi
			},
			want: true,
		},
		"mob following mob is mob side": {
			setup: func() Actor {
				m1 := newTestMI("m1", "Mob1")
				m2 := newTestMI("m2", "Mob2")
				m2.SetFollowing(m1)
				return m2
			},
			want: false,
		},
		"mob following mob following character is player side": {
			setup: func() Actor {
				ci := newTestCI("a", "A")
				m1 := newTestMI("m1", "Mob1")
				m2 := newTestMI("m2", "Mob2")
				m1.SetFollowing(ci)
				m2.SetFollowing(m1)
				return m2
			},
			want: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := IsPlayerSide(tc.setup()); got != tc.want {
				t.Errorf("IsPlayerSide = %v, want %v", got, tc.want)
			}
		})
	}
}
