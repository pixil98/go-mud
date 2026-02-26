package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
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
	targets.ForEachPlayer(func(charId string, _ *game.PlayerState) {
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

// mockPlayerLookup is a test double for PlayerLookup.
type mockPlayerLookup struct {
	players map[string]*game.PlayerState
}

func (m *mockPlayerLookup) GetPlayer(charId string) *game.PlayerState {
	return m.players[charId]
}

func newPlayerState(charId, name string) *game.PlayerState {
	return &game.PlayerState{
		Character: storage.NewResolvedSmartIdentifier(charId, &game.Character{Name: name}),
	}
}

func TestFollowHandler(t *testing.T) {
	tests := map[string]struct {
		setup       func(lookup *mockPlayerLookup, alice, bob *game.PlayerState)
		target      *TargetRef // nil means unfollow path
		expErr      string
		expFollowId string // expected FollowingId on alice after handler runs
		expMsgAlice string // substring expected in message to alice
		expMsgBob   string // substring expected in message to bob
	}{
		"follow a player": {
			target: &TargetRef{
				Type:   TargetTypePlayer,
				Player: &PlayerRef{CharId: "bob", Name: "Bob"},
			},
			expFollowId: "bob",
			expMsgAlice: "You now follow Bob.",
			expMsgBob:   "Alice now follows you.",
		},
		"follow yourself": {
			target: &TargetRef{
				Type:   TargetTypePlayer,
				Player: &PlayerRef{CharId: "alice", Name: "Alice"},
			},
			expErr: "You can't follow yourself.",
		},
		"already following target": {
			setup: func(lookup *mockPlayerLookup, alice, bob *game.PlayerState) {
				alice.FollowingId = "bob"
			},
			target: &TargetRef{
				Type:   TargetTypePlayer,
				Player: &PlayerRef{CharId: "bob", Name: "Bob"},
			},
			expErr: "You are already following Bob.",
		},
		"circular follow": {
			setup: func(lookup *mockPlayerLookup, alice, bob *game.PlayerState) {
				bob.FollowingId = "alice"
			},
			target: &TargetRef{
				Type:   TargetTypePlayer,
				Player: &PlayerRef{CharId: "bob", Name: "Bob"},
			},
			expErr: "Sorry, following in loops is not allowed.",
		},
		"switch leader": {
			setup: func(lookup *mockPlayerLookup, alice, bob *game.PlayerState) {
				alice.FollowingId = "bob"
				charlie := newPlayerState("charlie", "Charlie")
				lookup.players["charlie"] = charlie
			},
			target: &TargetRef{
				Type:   TargetTypePlayer,
				Player: &PlayerRef{CharId: "charlie", Name: "Charlie"},
			},
			expFollowId: "charlie",
			expMsgAlice: "You now follow Charlie.",
		},
		"unfollow when following": {
			setup: func(lookup *mockPlayerLookup, alice, bob *game.PlayerState) {
				alice.FollowingId = "bob"
			},
			target:      nil,
			expFollowId: "",
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
			alice := newPlayerState("alice", "Alice")
			bob := newPlayerState("bob", "Bob")
			lookup := &mockPlayerLookup{
				players: map[string]*game.PlayerState{"alice": alice, "bob": bob},
			}
			pub := &recordingPublisher{}

			if tt.setup != nil {
				tt.setup(lookup, alice, bob)
			}

			factory := NewFollowHandlerFactory(lookup, pub)
			handler, err := factory.Create()
			if err != nil {
				t.Fatalf("Create() error: %v", err)
			}

			targets := make(map[string]*TargetRef)
			if tt.target != nil {
				targets["target"] = tt.target
			}

			cmdCtx := &CommandContext{
				Actor:   alice.Character.Get(),
				Session: alice,
				Targets: targets,
				Config:  make(map[string]string),
			}

			err = handler(context.Background(), cmdCtx)

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

			if alice.FollowingId != tt.expFollowId {
				t.Errorf("FollowingId = %q, expected %q", alice.FollowingId, tt.expFollowId)
			}

			if tt.expMsgAlice != "" {
				msgs := pub.messagesTo("alice")
				found := false
				for _, m := range msgs {
					if strings.Contains(m, tt.expMsgAlice) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected message to alice containing %q, got %v", tt.expMsgAlice, msgs)
				}
			}

			if tt.expMsgBob != "" {
				msgs := pub.messagesTo("bob")
				found := false
				for _, m := range msgs {
					if strings.Contains(m, tt.expMsgBob) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected message to bob containing %q, got %v", tt.expMsgBob, msgs)
				}
			}
		})
	}
}

func TestWouldCreateLoop(t *testing.T) {
	tests := map[string]struct {
		setup      func(lookup *mockPlayerLookup)
		followerId string
		leaderId   string
		exp        bool
	}{
		"no loop": {
			setup: func(lookup *mockPlayerLookup) {
				lookup.players["a"] = newPlayerState("a", "A")
				lookup.players["b"] = newPlayerState("b", "B")
			},
			followerId: "a",
			leaderId:   "b",
			exp:        false,
		},
		"direct loop": {
			setup: func(lookup *mockPlayerLookup) {
				a := newPlayerState("a", "A")
				a.FollowingId = "b"
				lookup.players["a"] = a
				lookup.players["b"] = newPlayerState("b", "B")
			},
			followerId: "b",
			leaderId:   "a",
			exp:        true,
		},
		"indirect loop": {
			setup: func(lookup *mockPlayerLookup) {
				a := newPlayerState("a", "A")
				b := newPlayerState("b", "B")
				a.FollowingId = "c"
				b.FollowingId = "a"
				lookup.players["a"] = a
				lookup.players["b"] = b
				lookup.players["c"] = newPlayerState("c", "C")
			},
			followerId: "c",
			leaderId:   "b",
			exp:        true,
		},
		"leader not following anyone": {
			setup: func(lookup *mockPlayerLookup) {
				lookup.players["a"] = newPlayerState("a", "A")
				lookup.players["b"] = newPlayerState("b", "B")
			},
			followerId: "a",
			leaderId:   "b",
			exp:        false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			lookup := &mockPlayerLookup{players: make(map[string]*game.PlayerState)}
			tt.setup(lookup)

			got := wouldCreateLoop(lookup, tt.followerId, tt.leaderId)
			if got != tt.exp {
				t.Errorf("wouldCreateLoop(%q, %q) = %v, expected %v", tt.followerId, tt.leaderId, got, tt.exp)
			}
		})
	}
}
