package commands

import (
	"context"
	"strings"
	"testing"

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

func TestAssistHandler(t *testing.T) {
	tests := map[string]struct {
		setup          func() (*AssistHandlerFactory, *CommandInput, *gametest.BaseActor)
		expErr         string
		expMsgActor    string
		expMsgAssisted string
		expMsgRoom     string
	}{
		"assist explicit target": {
			setup: func() (*AssistHandlerFactory, *CommandInput, *gametest.BaseActor) {
				room, err := newTestRoom("test-room", "Test Room", "test-zone")
				if err != nil {
					t.Fatalf("failed to create test room: %v", err)
				}
				zone, err := newTestZone("test-zone")
				if err != nil {
					t.Fatalf("failed to create test zone: %v", err)
				}
				zone.AddRoom(room)
				pub := &recordingPublisher{}

				mob := newCombatMob("mob:test-mob", "Goblin")
				room.AddMob(mob)

				bob := &gametest.BaseActor{ActorId: "bob", ActorName: "Bob", Alive: true, CombatTarget: mob.Id(), ActorRoom: room}
				actor := &gametest.BaseActor{ActorId: "alice", ActorName: "Alice", Alive: true, ActorRoom: room}

				_ = newTestPlayer("charlie", "Charlie", room)

				players := &mockAssistPlayerLookup{players: map[string]AssistedPlayer{"bob": bob}}
				f := &AssistHandlerFactory{
					players: players,
					pub:     pub,
				}

				cmdCtx := &CommandInput{
					Actor: actor,
					Targets: map[string]*TargetRef{
						"target": {Type: targetTypeActor, Actor: &ActorRef{CharId: "bob", Name: "Bob"}},
					},
					Config: make(map[string]string),
				}
				return f, cmdCtx, actor
			},
			expMsgActor:    "You jump to Bob's aid!",
			expMsgAssisted: "Alice jumps to your aid!",
			expMsgRoom:     "Alice jumps to Bob's aid!",
		},
		"assist follow leader": {
			setup: func() (*AssistHandlerFactory, *CommandInput, *gametest.BaseActor) {
				room, err := newTestRoom("test-room", "Test Room", "test-zone")
				if err != nil {
					t.Fatalf("failed to create test room: %v", err)
				}
				zone, err := newTestZone("test-zone")
				if err != nil {
					t.Fatalf("failed to create test zone: %v", err)
				}
				zone.AddRoom(room)
				pub := &recordingPublisher{}

				mob := newCombatMob("mob:test-mob", "Goblin")
				room.AddMob(mob)

				bob := &gametest.BaseActor{ActorId: "bob", ActorName: "Bob", Alive: true, CombatTarget: mob.Id(), ActorRoom: room}
				actor := &gametest.BaseActor{ActorId: "alice", ActorName: "Alice", Alive: true, ActorFollowing: bob, ActorRoom: room}

				players := &mockAssistPlayerLookup{players: map[string]AssistedPlayer{"bob": bob}}
				f := &AssistHandlerFactory{
					players: players,
					pub:     pub,
				}

				cmdCtx := &CommandInput{
					Actor:   actor,
					Targets: map[string]*TargetRef{},
					Config:  make(map[string]string),
				}
				return f, cmdCtx, actor
			},
			expMsgActor:    "You jump to Bob's aid!",
			expMsgAssisted: "Alice jumps to your aid!",
		},
		"already in combat": {
			setup: func() (*AssistHandlerFactory, *CommandInput, *gametest.BaseActor) {
				f := &AssistHandlerFactory{players: &mockAssistPlayerLookup{}, pub: &recordingPublisher{}}
				actor := &gametest.BaseActor{ActorId: "alice", ActorName: "Alice", InCombat: true}
				cmdCtx := &CommandInput{
					Actor:   actor,
					Targets: map[string]*TargetRef{},
					Config:  make(map[string]string),
				}
				return f, cmdCtx, actor
			},
			expErr: "already fighting",
		},
		"no target and not following": {
			setup: func() (*AssistHandlerFactory, *CommandInput, *gametest.BaseActor) {
				f := &AssistHandlerFactory{players: &mockAssistPlayerLookup{}, pub: &recordingPublisher{}}
				actor := &gametest.BaseActor{ActorId: "alice", ActorName: "Alice"}
				cmdCtx := &CommandInput{
					Actor:   actor,
					Targets: map[string]*TargetRef{},
					Config:  make(map[string]string),
				}
				return f, cmdCtx, actor
			},
			expErr: "Assist whom?",
		},
		"assisted player not in combat": {
			setup: func() (*AssistHandlerFactory, *CommandInput, *gametest.BaseActor) {
				bob := &gametest.BaseActor{ActorId: "bob", ActorName: "Bob"}
				f := &AssistHandlerFactory{players: &mockAssistPlayerLookup{players: map[string]AssistedPlayer{"bob": bob}}, pub: &recordingPublisher{}}
				actor := &gametest.BaseActor{ActorId: "alice", ActorName: "Alice"}
				cmdCtx := &CommandInput{
					Actor: actor,
					Targets: map[string]*TargetRef{
						"target": {Type: targetTypeActor, Actor: &ActorRef{CharId: "bob", Name: "Bob"}},
					},
					Config: make(map[string]string),
				}
				return f, cmdCtx, actor
			},
			expErr: "isn't fighting anyone",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			factory, cmdCtx, actor := tt.setup()

			err := factory.handle(context.Background(), cmdCtx)

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

			pub := factory.pub.(*recordingPublisher)
			actorId := actor.Id()

			if tt.expMsgActor != "" {
				if !containsSubstring(actor.Notified, tt.expMsgActor) {
					t.Errorf("expected Notify to actor containing %q, got %v", tt.expMsgActor, actor.Notified)
				}
			}
			if tt.expMsgAssisted != "" {
				var assistedId string
				if ref := cmdCtx.Targets["target"]; ref != nil {
					assistedId = ref.Actor.CharId
				} else if leader := actor.Following(); leader != nil {
					assistedId = leader.Id()
				}
				msgs := pub.messagesTo(assistedId)
				if !containsSubstring(msgs, tt.expMsgAssisted) {
					t.Errorf("expected message to assisted containing %q, got %v", tt.expMsgAssisted, msgs)
				}
			}
			if tt.expMsgRoom != "" {
				found := false
				for _, m := range pub.messages {
					if m.targetId != actorId && strings.Contains(m.data, tt.expMsgRoom) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected room message containing %q, got %v", tt.expMsgRoom, pub.messages)
				}
			}
		})
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
