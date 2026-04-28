package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/gametest"
)

var _ AssistedPlayer = (*gametest.BaseActor)(nil)

// mockAssistPlayerLookup is a test double for AssistPlayerLookup.
type mockAssistPlayerLookup struct {
	players map[string]AssistedPlayer
}

func (m *mockAssistPlayerLookup) GetPlayer(charId string) AssistedPlayer {
	return m.players[charId]
}

// assistTestCtx bundles the witnesses a test wants to assert against.
type assistTestCtx struct {
	factory  *AssistHandlerFactory
	in       *CommandInput
	actor    *gametest.BaseActor
	assisted *gametest.BaseActor       // nil when not relevant
	witness  *game.CharacterInstance   // a third party in the room used to verify room broadcasts
	witCh    chan []byte
}

func TestAssistHandler(t *testing.T) {
	tests := map[string]struct {
		setup          func() *assistTestCtx
		expErr         string
		expMsgActor    string
		expMsgAssisted string
		expMsgRoom     string
	}{
		"assist explicit target": {
			setup: func() *assistTestCtx {
				room, err := newTestRoom("test-room", "Test Room", "test-zone")
				if err != nil {
					t.Fatalf("failed to create test room: %v", err)
				}
				zone, err := newTestZone("test-zone")
				if err != nil {
					t.Fatalf("failed to create test zone: %v", err)
				}
				zone.AddRoom(room)

				mob := newCombatMob("mob:test-mob", "Goblin")
				room.AddMob(mob)

				bob := &gametest.BaseActor{ActorId: "bob", ActorName: "Bob", Alive: true, ActorCombatTarget: mob, ActorRoom: room}
				actor := &gametest.BaseActor{ActorId: "alice", ActorName: "Alice", Alive: true, ActorRoom: room}

				witness, witCh := newRecordingPlayer("charlie", "Charlie", room)

				players := &mockAssistPlayerLookup{players: map[string]AssistedPlayer{"bob": bob}}
				f := &AssistHandlerFactory{players: players}

				in := &CommandInput{
					Actor: actor,
					Targets: map[string][]*TargetRef{
						"target": {{Type: targetTypeActor, Actor: &ActorRef{CharId: "bob", Name: "Bob"}}},
					},
					Config: make(map[string]string),
				}
				return &assistTestCtx{factory: f, in: in, actor: actor, assisted: bob, witness: witness, witCh: witCh}
			},
			expMsgActor:    "You jump to Bob's aid!",
			expMsgAssisted: "Alice jumps to your aid!",
			expMsgRoom:     "Alice jumps to Bob's aid!",
		},
		"assist follow leader": {
			setup: func() *assistTestCtx {
				room, err := newTestRoom("test-room", "Test Room", "test-zone")
				if err != nil {
					t.Fatalf("failed to create test room: %v", err)
				}
				zone, err := newTestZone("test-zone")
				if err != nil {
					t.Fatalf("failed to create test zone: %v", err)
				}
				zone.AddRoom(room)

				mob := newCombatMob("mob:test-mob", "Goblin")
				room.AddMob(mob)

				bob := &gametest.BaseActor{ActorId: "bob", ActorName: "Bob", Alive: true, ActorCombatTarget: mob, ActorRoom: room}
				actor := &gametest.BaseActor{ActorId: "alice", ActorName: "Alice", Alive: true, ActorFollowing: bob, ActorRoom: room}

				players := &mockAssistPlayerLookup{players: map[string]AssistedPlayer{"bob": bob}}
				f := &AssistHandlerFactory{players: players}

				in := &CommandInput{
					Actor:   actor,
					Targets: map[string][]*TargetRef{},
					Config:  make(map[string]string),
				}
				return &assistTestCtx{factory: f, in: in, actor: actor, assisted: bob}
			},
			expMsgActor:    "You jump to Bob's aid!",
			expMsgAssisted: "Alice jumps to your aid!",
		},
		"already in combat": {
			setup: func() *assistTestCtx {
				f := &AssistHandlerFactory{players: &mockAssistPlayerLookup{}}
				actor := &gametest.BaseActor{ActorId: "alice", ActorName: "Alice", InCombat: true}
				in := &CommandInput{
					Actor:   actor,
					Targets: map[string][]*TargetRef{},
					Config:  make(map[string]string),
				}
				return &assistTestCtx{factory: f, in: in, actor: actor}
			},
			expErr: "already fighting",
		},
		"no target and not following": {
			setup: func() *assistTestCtx {
				f := &AssistHandlerFactory{players: &mockAssistPlayerLookup{}}
				actor := &gametest.BaseActor{ActorId: "alice", ActorName: "Alice"}
				in := &CommandInput{
					Actor:   actor,
					Targets: map[string][]*TargetRef{},
					Config:  make(map[string]string),
				}
				return &assistTestCtx{factory: f, in: in, actor: actor}
			},
			expErr: "Assist whom?",
		},
		"assisted player not in combat": {
			setup: func() *assistTestCtx {
				bob := &gametest.BaseActor{ActorId: "bob", ActorName: "Bob"}
				f := &AssistHandlerFactory{players: &mockAssistPlayerLookup{players: map[string]AssistedPlayer{"bob": bob}}}
				actor := &gametest.BaseActor{ActorId: "alice", ActorName: "Alice"}
				in := &CommandInput{
					Actor: actor,
					Targets: map[string][]*TargetRef{
						"target": {{Type: targetTypeActor, Actor: &ActorRef{CharId: "bob", Name: "Bob"}}},
					},
					Config: make(map[string]string),
				}
				return &assistTestCtx{factory: f, in: in, actor: actor, assisted: bob}
			},
			expErr: "isn't fighting anyone",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := tt.setup()

			err := ctx.factory.handle(context.Background(), ctx.in)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Fatalf("expected error containing %q, got %q", tt.expErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expMsgActor != "" {
				if !containsSubstring(ctx.actor.PublishedStrings(), tt.expMsgActor) {
					t.Errorf("expected published msg to actor containing %q, got %v", tt.expMsgActor, ctx.actor.PublishedStrings())
				}
			}
			if tt.expMsgAssisted != "" {
				if ctx.assisted == nil {
					t.Fatalf("test expects assisted message but setup did not provide an assisted actor")
				}
				if !containsSubstring(ctx.assisted.PublishedStrings(), tt.expMsgAssisted) {
					t.Errorf("expected published msg to assisted containing %q, got %v", tt.expMsgAssisted, ctx.assisted.PublishedStrings())
				}
			}
			if tt.expMsgRoom != "" {
				if ctx.witness == nil {
					t.Fatalf("test expects room message but setup did not provide a witness")
				}
				msgs := drainAll(ctx.witCh)
				if !containsSubstring(msgs, tt.expMsgRoom) {
					t.Errorf("expected witness to receive room message containing %q, got %v", tt.expMsgRoom, msgs)
				}
			}
		})
	}
}

// drainAll reads all currently-buffered messages from ch and returns them as
// strings.
func drainAll(ch chan []byte) []string {
	var out []string
	for {
		select {
		case b := <-ch:
			out = append(out, string(b))
		default:
			return out
		}
	}
}

func containsSubstring(msgs []string, sub string) bool {
	for _, m := range msgs {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}
