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

// mockFollowActor is a lightweight test double for FollowActor.
// It also satisfies FollowedPlayer, so it can be used for looked-up players too.
type mockFollowActor struct {
	id          string
	name        string
	followingId string
}

func (m *mockFollowActor) Id() string                               { return m.id }
func (m *mockFollowActor) Name() string                             { return m.name }
func (m *mockFollowActor) Location() (string, string)               { return "", "" }
func (m *mockFollowActor) IsInCombat() bool                         { return false }
func (m *mockFollowActor) IsAlive() bool                            { return true }
func (m *mockFollowActor) Level() int                               { return 1 }
func (m *mockFollowActor) Resource(string) (int, int)               { return 0, 0 }
func (m *mockFollowActor) AdjustResource(string, int, bool)         {}
func (m *mockFollowActor) SpendAP(int) bool                         { return true }
func (m *mockFollowActor) HasGrant(string, string) bool             { return false }
func (m *mockFollowActor) ModifierValue(string) int                 { return 0 }
func (m *mockFollowActor) GrantArgs(string) []string                { return nil }
func (m *mockFollowActor) AddTimedPerks(string, []assets.Perk, int) {}
func (m *mockFollowActor) SetInCombat(bool)                         {}
func (m *mockFollowActor) CombatTargetId() string                   { return "" }
func (m *mockFollowActor) SetCombatTargetId(string)                 {}
func (m *mockFollowActor) OnDeath() []any                           { return nil }
func (m *mockFollowActor) IsCharacter() bool                        { return true }
func (m *mockFollowActor) GetFollowingId() string                   { return m.followingId }
func (m *mockFollowActor) SetFollowingId(id string)                 { m.followingId = id }

var _ FollowActor = (*mockFollowActor)(nil)
var _ FollowedPlayer = (*mockFollowActor)(nil)

// mockFollowPlayerLookup is a test double for FollowPlayerLookup.
type mockFollowPlayerLookup struct {
	players map[string]*mockFollowActor
}

func (m *mockFollowPlayerLookup) GetPlayer(charId string) FollowedPlayer {
	if m.players == nil {
		return nil
	}
	p := m.players[charId]
	if p == nil {
		return nil
	}
	return p
}

func TestFollowHandler(t *testing.T) {
	tests := map[string]struct {
		setup       func(lookup *mockFollowPlayerLookup, alice, bob *mockFollowActor)
		target      *TargetRef // nil means unfollow path
		expErr      string
		expFollowId string // expected FollowingId on alice after handler runs
		expMsgAlice string // substring expected in message to alice
		expMsgBob   string // substring expected in message to bob
	}{
		"follow a player": {
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "bob", Name: "Bob"},
			},
			expFollowId: "bob",
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
			setup: func(lookup *mockFollowPlayerLookup, alice, bob *mockFollowActor) {
				alice.followingId = "bob"
			},
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "bob", Name: "Bob"},
			},
			expErr: "You are already following Bob.",
		},
		"circular follow": {
			setup: func(lookup *mockFollowPlayerLookup, alice, bob *mockFollowActor) {
				bob.followingId = "alice"
			},
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "bob", Name: "Bob"},
			},
			expErr: "Sorry, following in loops is not allowed.",
		},
		"switch leader": {
			setup: func(lookup *mockFollowPlayerLookup, alice, bob *mockFollowActor) {
				alice.followingId = "bob"
				lookup.players["charlie"] = &mockFollowActor{id: "charlie", name: "Charlie"}
			},
			target: &TargetRef{
				Type:  targetTypeActor,
				Actor: &ActorRef{CharId: "charlie", Name: "Charlie"},
			},
			expFollowId: "charlie",
			expMsgAlice: "You now follow Charlie.",
		},
		"unfollow when following": {
			setup: func(lookup *mockFollowPlayerLookup, alice, bob *mockFollowActor) {
				alice.followingId = "bob"
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
			alice := &mockFollowActor{id: "alice", name: "Alice"}
			bob := &mockFollowActor{id: "bob", name: "Bob"}
			lookup := &mockFollowPlayerLookup{
				players: map[string]*mockFollowActor{"alice": alice, "bob": bob},
			}
			pub := &recordingPublisher{}

			if tt.setup != nil {
				tt.setup(lookup, alice, bob)
			}

			factory := &FollowHandlerFactory{players: lookup, pub: pub}

			targets := make(map[string]*TargetRef)
			if tt.target != nil {
				targets["target"] = tt.target
			}

			cmdCtx := &CommandInput{
				Actor:   alice,
				Targets: targets,
				Config:  make(map[string]string),
			}

			err := factory.handle(context.Background(), alice, cmdCtx)

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

			if alice.GetFollowingId() != tt.expFollowId {
				t.Errorf("FollowingId = %q, expected %q", alice.GetFollowingId(), tt.expFollowId)
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
		setup      func(lookup *mockFollowPlayerLookup)
		followerId string
		leaderId   string
		exp        bool
	}{
		"no loop": {
			setup: func(lookup *mockFollowPlayerLookup) {
				lookup.players["a"] = &mockFollowActor{id: "a", name: "A"}
				lookup.players["b"] = &mockFollowActor{id: "b", name: "B"}
			},
			followerId: "a",
			leaderId:   "b",
			exp:        false,
		},
		"direct loop": {
			setup: func(lookup *mockFollowPlayerLookup) {
				lookup.players["a"] = &mockFollowActor{id: "a", name: "A", followingId: "b"}
				lookup.players["b"] = &mockFollowActor{id: "b", name: "B"}
			},
			followerId: "b",
			leaderId:   "a",
			exp:        true,
		},
		"indirect loop": {
			setup: func(lookup *mockFollowPlayerLookup) {
				lookup.players["a"] = &mockFollowActor{id: "a", name: "A", followingId: "c"}
				lookup.players["b"] = &mockFollowActor{id: "b", name: "B", followingId: "a"}
				lookup.players["c"] = &mockFollowActor{id: "c", name: "C"}
			},
			followerId: "c",
			leaderId:   "b",
			exp:        true,
		},
		"leader not following anyone": {
			setup: func(lookup *mockFollowPlayerLookup) {
				lookup.players["a"] = &mockFollowActor{id: "a", name: "A"}
				lookup.players["b"] = &mockFollowActor{id: "b", name: "B"}
			},
			followerId: "a",
			leaderId:   "b",
			exp:        false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			lookup := &mockFollowPlayerLookup{players: make(map[string]*mockFollowActor)}
			tt.setup(lookup)

			got := wouldCreateLoop(lookup, tt.followerId, tt.leaderId)
			if got != tt.exp {
				t.Errorf("wouldCreateLoop(%q, %q) = %v, expected %v", tt.followerId, tt.leaderId, got, tt.exp)
			}
		})
	}
}
