package commands

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// TestGroupHealEffect verifies that group_heal walks the caster's group
// tree and heals every member present in the caster's room. Solo casters
// heal only themselves; non-grouped followers are skipped.
func TestGroupHealEffect(t *testing.T) {
	type fixture struct {
		leader  *game.CharacterInstance
		member  *game.MobileInstance // mixed-type: mob in a player's group
		other   *game.CharacterInstance
		wildMob *game.MobileInstance
	}

	tests := map[string]struct {
		casterRole string // "leader" (player in group) or "other" (solo player)
		setup      func(f *fixture)
		expHealed  []string // roles expected to be healed (and thus HP > missing)
	}{
		"solo caster heals only self": {
			casterRole: "other",
			expHealed:  []string{"other"},
		},
		"grouped caster heals mixed-type allies": {
			casterRole: "leader",
			setup: func(f *fixture) {
				f.member.SetFollowing(f.leader)
				f.leader.SetFollowerGrouped(f.member.Id(), true)
			},
			expHealed: []string{"leader", "member"},
		},
		"grouped caster skips non-group players in the room": {
			casterRole: "leader",
			setup: func(f *fixture) {
				f.member.SetFollowing(f.leader)
				f.leader.SetFollowerGrouped(f.member.Id(), true)
			},
			expHealed: []string{"leader", "member"}, // other is NOT in group
		},
		"grouped caster skips wild mobs": {
			casterRole: "leader",
			setup: func(f *fixture) {
				f.member.SetFollowing(f.leader)
				f.leader.SetFollowerGrouped(f.member.Id(), true)
			},
			expHealed: []string{"leader", "member"}, // wildMob is NOT in group
		},
		"non-grouped follower is skipped": {
			casterRole: "leader",
			setup: func(f *fixture) {
				// member follows the leader but is NOT grouped
				f.member.SetFollowing(f.leader)
			},
			expHealed: []string{"leader"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room, _ := newTestRoomInZone("r", "Room", "z")

			leader := newTestPlayer("leader", "Leader", room)
			setCombatReady(leader)
			leader.SetResource(assets.ResourceHp, 50) // damaged

			other := newTestPlayer("other", "Other", room)
			setCombatReady(other)
			other.SetResource(assets.ResourceHp, 50)

			member := newCombatMob("member", "member")
			room.AddMob(member)
			member.SetResource(assets.ResourceHp, 50)

			wildMob := newCombatMob("wild", "wild")
			room.AddMob(wildMob)
			wildMob.SetResource(assets.ResourceHp, 50)

			f := &fixture{leader: leader, member: member, other: other, wildMob: wildMob}
			if tc.setup != nil {
				tc.setup(f)
			}

			var caster game.Actor
			switch tc.casterRole {
			case "leader":
				caster = leader
			case "other":
				caster = other
			default:
				t.Fatalf("unknown casterRole %q", tc.casterRole)
			}

			effect := &groupHealEffect{}
			fn := effect.Create("test:0", map[string]string{"amount": "10"}, nil)
			if err := fn(caster, nil, &AbilityResult{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			roles := map[string]game.Actor{
				"leader":  leader,
				"other":   other,
				"member":  member,
				"wildMob": wildMob,
			}
			healed := make(map[string]bool, len(tc.expHealed))
			for _, r := range tc.expHealed {
				healed[r] = true
			}

			for role, a := range roles {
				cur, _ := a.Resource(assets.ResourceHp)
				if healed[role] {
					if cur <= 50 {
						t.Errorf("role %q should have been healed, HP=%d", role, cur)
					}
				} else {
					if cur != 50 {
						t.Errorf("role %q should be unchanged, HP=%d", role, cur)
					}
				}
			}
		})
	}
}
