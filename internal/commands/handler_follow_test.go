package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// recordingPublisher captures messages sent via Publish for test assertions.
type recordingPublisher struct {
	messages []publishedMessage
}

type publishedMessage struct {
	targetId string
	data     string
}

func (p *recordingPublisher) Publish(targets game.PlayerGroup, exclude []string, data []byte) error {
	targets.ForEachPlayer(func(charId string, _ *game.CharacterInstance) {
		p.messages = append(p.messages, publishedMessage{targetId: charId, data: string(data)})
	})
	return nil
}

func (p *recordingPublisher) messagesTo(charId string) []string {
	var msgs []string
	for _, m := range p.messages {
		if m.targetId == charId {
			msgs = append(msgs, m.data)
		}
	}
	return msgs
}

// TODO: create a common mockActor for the commands package that satisfies
// both shared.Actor and game.FollowTarget, replacing per-test mock types.

// mockActor satisfies shared.Actor and game.FollowTarget for follow handler tests.
type mockActor struct {
	id        string
	name      string
	notified  []string
	following game.FollowTarget
	followers []game.FollowTarget
}

func (m *mockActor) Id() string                               { return m.id }
func (m *mockActor) Name() string                             { return m.name }
func (m *mockActor) Notify(msg string)                        { m.notified = append(m.notified, msg) }
func (m *mockActor) Location() (string, string)               { return "", "" }
func (m *mockActor) IsInCombat() bool                         { return false }
func (m *mockActor) IsAlive() bool                            { return true }
func (m *mockActor) Level() int                               { return 1 }
func (m *mockActor) Resource(string) (int, int)               { return 0, 0 }
func (m *mockActor) AdjustResource(string, int, bool)         {}
func (m *mockActor) SpendAP(int) bool                         { return true }
func (m *mockActor) HasGrant(string, string) bool             { return false }
func (m *mockActor) ModifierValue(string) int                 { return 0 }
func (m *mockActor) GrantArgs(string) []string                { return nil }
func (m *mockActor) AddTimedPerks(string, []assets.Perk, int) {}
func (m *mockActor) SetInCombat(bool)                         {}
func (m *mockActor) CombatTargetId() string                   { return "" }
func (m *mockActor) SetCombatTargetId(string)                 {}
func (m *mockActor) OnDeath() []*game.ObjectInstance          { return nil }
func (m *mockActor) IsCharacter() bool                        { return true }
func (m *mockActor) Inventory() *game.Inventory               { return nil }
func (m *mockActor) Following() game.FollowTarget             { return m.following }
func (m *mockActor) SetFollowing(ft game.FollowTarget)        { m.following = ft }
func (m *mockActor) Followers() []game.FollowTarget           { return m.followers }
func (m *mockActor) AddFollower(ft game.FollowTarget)         { m.followers = append(m.followers, ft) }
func (m *mockActor) RemoveFollower(string)                    {}
func (m *mockActor) SetFollowerGrouped(string, bool)          {}
func (m *mockActor) IsFollowerGrouped(string) bool            { return false }
func (m *mockActor) GroupedFollowers() []game.FollowTarget    { return nil }
func (m *mockActor) Move(_, _ *game.RoomInstance)             {}

func TestFollowHandler(t *testing.T) {
	tests := map[string]struct {
		setup       func(alice, bob *mockActor, charlie *mockActor)
		target      *TargetRef // nil means unfollow path
		expErr      string
		expFollow   string // expected Id() of who alice follows after handler, "" means nil
		expMsgAlice string // substring expected in alice's notifications
		expMsgBob   string // substring expected in bob's notifications
	}{
		"follow a player": {
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "bob", Name: "Bob"},
			},
			expFollow:   "bob",
			expMsgAlice: "You now follow Bob.",
			expMsgBob:   "Alice now follows you.",
		},
		"follow yourself": {
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "alice", Name: "Alice"},
			},
			expErr: "You can't follow yourself.",
		},
		"already following target": {
			setup: func(alice, bob, charlie *mockActor) {
				alice.following = bob
			},
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "bob", Name: "Bob"},
			},
			expErr: "You are already following Bob.",
		},
		"circular follow": {
			setup: func(alice, bob, charlie *mockActor) {
				bob.following = alice
			},
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "bob", Name: "Bob"},
			},
			expErr: "Sorry, following in loops is not allowed.",
		},
		"switch leader": {
			setup: func(alice, bob, charlie *mockActor) {
				alice.following = bob
			},
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "charlie", Name: "Charlie"},
			},
			expFollow:   "charlie",
			expMsgAlice: "You now follow Charlie.",
		},
		"unfollow when following": {
			setup: func(alice, bob, charlie *mockActor) {
				alice.following = bob
			},
			target:      nil,
			expFollow:   "",
			expMsgAlice: "You stop following Bob.",
			expMsgBob:   "Alice stops following you.",
		},
		"unfollow when not following": {
			target: nil,
			expErr: "You aren't following anyone.",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			alice := &mockActor{id: "alice", name: "Alice"}
			bob := &mockActor{id: "bob", name: "Bob"}
			charlie := &mockActor{id: "charlie", name: "Charlie"}

			if tt.setup != nil {
				tt.setup(alice, bob, charlie)
			}

			// Wire the target's ActorRef to point at the right mock.
			if tt.target != nil && tt.target.Actor != nil {
				switch tt.target.Actor.CharId {
				case "bob":
					tt.target.Actor.actor = bob
				case "charlie":
					tt.target.Actor.actor = charlie
				case "alice":
					tt.target.Actor.actor = alice
				}
			}

			factory := &FollowHandlerFactory{}

			targets := make(map[string]*TargetRef)
			if tt.target != nil {
				targets["target"] = tt.target
			}

			in := &CommandInput{
				Actor:   alice,
				Targets: targets,
				Config:  make(map[string]string),
			}

			err := factory.handle(context.Background(), alice, in)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error = %q, expected to contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expFollow == "" {
				if alice.following != nil {
					t.Errorf("expected following to be nil, got %q", alice.following.Id())
				}
			} else {
				if alice.following == nil {
					t.Errorf("expected following %q, got nil", tt.expFollow)
				} else if alice.following.Id() != tt.expFollow {
					t.Errorf("following = %q, expected %q", alice.following.Id(), tt.expFollow)
				}
			}

			if tt.expMsgAlice != "" {
				if !containsSubstring(alice.notified, tt.expMsgAlice) {
					t.Errorf("expected Notify to alice containing %q, got %v", tt.expMsgAlice, alice.notified)
				}
			}

			if tt.expMsgBob != "" {
				if !containsSubstring(bob.notified, tt.expMsgBob) {
					t.Errorf("expected Notify to bob containing %q, got %v", tt.expMsgBob, bob.notified)
				}
			}
		})
	}
}

func TestWouldCreateLoop(t *testing.T) {
	tests := map[string]struct {
		setup func() (followerId string, leader game.FollowTarget)
		exp   bool
	}{
		"no loop": {
			setup: func() (string, game.FollowTarget) {
				b := &mockActor{id: "b", name: "B"}
				return "a", b
			},
			exp: false,
		},
		"direct loop": {
			setup: func() (string, game.FollowTarget) {
				a := &mockActor{id: "a", name: "A"}
				b := &mockActor{id: "b", name: "B", following: a}
				return "a", b
			},
			exp: true,
		},
		"indirect loop": {
			setup: func() (string, game.FollowTarget) {
				c := &mockActor{id: "c", name: "C"}
				a := &mockActor{id: "a", name: "A", following: c}
				b := &mockActor{id: "b", name: "B", following: a}
				return "c", b
			},
			exp: true,
		},
		"leader not following anyone": {
			setup: func() (string, game.FollowTarget) {
				b := &mockActor{id: "b", name: "B"}
				return "a", b
			},
			exp: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			followerId, leader := tt.setup()
			got := wouldCreateLoop(followerId, leader)
			if got != tt.exp {
				t.Errorf("wouldCreateLoop(%q, %q) = %v, expected %v", followerId, leader.Id(), got, tt.exp)
			}
		})
	}
}
