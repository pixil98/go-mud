package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/gametest"
)

func TestFollowHandler(t *testing.T) {
	tests := map[string]struct {
		setup       func(alice, bob *gametest.BaseActor, charlie *gametest.BaseActor)
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
			setup: func(alice, bob, charlie *gametest.BaseActor) {
				alice.ActorFollowing = bob
			},
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "bob", Name: "Bob"},
			},
			expErr: "You are already following Bob.",
		},
		"circular follow": {
			setup: func(alice, bob, charlie *gametest.BaseActor) {
				bob.ActorFollowing = alice
			},
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "bob", Name: "Bob"},
			},
			expErr: "Sorry, following in loops is not allowed.",
		},
		"switch leader": {
			setup: func(alice, bob, charlie *gametest.BaseActor) {
				alice.ActorFollowing = bob
			},
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "charlie", Name: "Charlie"},
			},
			expFollow:   "charlie",
			expMsgAlice: "You now follow Charlie.",
		},
		"unfollow when following": {
			setup: func(alice, bob, charlie *gametest.BaseActor) {
				alice.ActorFollowing = bob
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
			alice := &gametest.BaseActor{ActorId: "alice", ActorName: "Alice"}
			bob := &gametest.BaseActor{ActorId: "bob", ActorName: "Bob"}
			charlie := &gametest.BaseActor{ActorId: "charlie", ActorName: "Charlie"}

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

			targets := make(map[string][]*TargetRef)
			if tt.target != nil {
				targets["target"] = []*TargetRef{tt.target}
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
				if alice.ActorFollowing != nil {
					t.Errorf("expected following to be nil, got %q", alice.ActorFollowing.Id())
				}
			} else {
				if alice.ActorFollowing == nil {
					t.Errorf("expected following %q, got nil", tt.expFollow)
				} else if alice.ActorFollowing.Id() != tt.expFollow {
					t.Errorf("following = %q, expected %q", alice.ActorFollowing.Id(), tt.expFollow)
				}
			}

			if tt.expMsgAlice != "" {
				if !containsSubstring(alice.PublishedStrings(), tt.expMsgAlice) {
					t.Errorf("expected Notify to alice containing %q, got %v", tt.expMsgAlice, alice.PublishedStrings())
				}
			}

			if tt.expMsgBob != "" {
				if !containsSubstring(bob.PublishedStrings(), tt.expMsgBob) {
					t.Errorf("expected Notify to bob containing %q, got %v", tt.expMsgBob, bob.PublishedStrings())
				}
			}
		})
	}
}

func TestWouldCreateLoop(t *testing.T) {
	tests := map[string]struct {
		setup func() (followerId string, leader game.Actor)
		exp   bool
	}{
		"no loop": {
			setup: func() (string, game.Actor) {
				b := &gametest.BaseActor{ActorId: "b", ActorName: "B"}
				return "a", b
			},
			exp: false,
		},
		"direct loop": {
			setup: func() (string, game.Actor) {
				a := &gametest.BaseActor{ActorId: "a", ActorName: "A"}
				b := &gametest.BaseActor{ActorId: "b", ActorName: "B", ActorFollowing: a}
				return "a", b
			},
			exp: true,
		},
		"indirect loop": {
			setup: func() (string, game.Actor) {
				c := &gametest.BaseActor{ActorId: "c", ActorName: "C"}
				a := &gametest.BaseActor{ActorId: "a", ActorName: "A", ActorFollowing: c}
				b := &gametest.BaseActor{ActorId: "b", ActorName: "B", ActorFollowing: a}
				return "c", b
			},
			exp: true,
		},
		"leader not following anyone": {
			setup: func() (string, game.Actor) {
				b := &gametest.BaseActor{ActorId: "b", ActorName: "B"}
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
