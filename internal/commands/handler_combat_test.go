package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
)

// mockCombatManager is a test double for CombatManager.
type mockCombatManager struct {
	startedErr      error // error to return from StartCombat
	started         bool
	startCount      int
	lastAttacker    game.Actor
	lastTarget      game.Actor
	threatAdded     bool
	threatAmount    int
	threatCount     int
	setThreatCalled bool
	setThreatAmount int
	topThreatCalled bool
}

func (m *mockCombatManager) StartCombat(attacker, target game.Actor) error {
	m.started = true
	m.startCount++
	m.lastAttacker = attacker
	m.lastTarget = target
	return m.startedErr
}

func (m *mockCombatManager) AddThreat(_, _ game.Actor, amount int) {
	m.threatAdded = true
	m.threatCount++
	m.threatAmount += amount
}

func (m *mockCombatManager) SetThreat(_, _ game.Actor, amount int) {
	m.setThreatCalled = true
	m.setThreatAmount = amount
}

func (m *mockCombatManager) TopThreat(_, _ game.Actor) {
	m.topThreatCalled = true
}

func (m *mockCombatManager) NotifyHeal(_, _ game.Actor, _ int) {}

var _ AssistedPlayer = (*mockActor)(nil)

// mockAssistPlayerLookup is a test double for AssistPlayerLookup.
type mockAssistPlayerLookup struct {
	players map[string]AssistedPlayer
}

func (m *mockAssistPlayerLookup) GetPlayer(charId string) AssistedPlayer {
	if m.players == nil {
		return nil
	}
	return m.players[charId]
}

func TestAssistHandler(t *testing.T) {
	type testCase struct {
		setup          func() (*AssistHandlerFactory, *CommandInput, *mockActor)
		expErr         string
		expStarted     bool
		expMsgActor    string // substring in message to actor
		expMsgAssisted string // substring in message to assisted
		expMsgRoom     string // substring in message to room bystander
	}
	tests := map[string]testCase{
		"assist explicit target in combat": {
			setup: func() (*AssistHandlerFactory, *CommandInput, *mockActor) {
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
				cm := &mockCombatManager{}

				bob := &mockActor{id: "bob", name: "Bob", combatTargetId: "mob:test-mob", room: room}
				actor := &mockActor{id: "alice", name: "Alice", room: room}

				// Add a bystander so the room broadcast has a recipient.
				_ = newTestPlayer("charlie", "Charlie", room)

				players := &mockAssistPlayerLookup{players: map[string]AssistedPlayer{"bob": bob}}
				f := &AssistHandlerFactory{
					combat:  cm,
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
			expStarted:     true,
			expMsgActor:    "You jump to Bob's aid!",
			expMsgAssisted: "Alice jumps to your aid!",
			expMsgRoom:     "Alice jumps to Bob's aid!",
		},
		"assist follow leader": {
			setup: func() (*AssistHandlerFactory, *CommandInput, *mockActor) {
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
				cm := &mockCombatManager{}

				bob := &mockActor{id: "bob", name: "Bob", combatTargetId: "mob:test-mob", room: room}
				actor := &mockActor{id: "alice", name: "Alice", following: bob, room: room}

				players := &mockAssistPlayerLookup{players: map[string]AssistedPlayer{"bob": bob}}
				f := &AssistHandlerFactory{
					combat:  cm,
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
			expStarted:     true,
			expMsgActor:    "You jump to Bob's aid!",
			expMsgAssisted: "Alice jumps to your aid!",
		},
		"already in combat": {
			setup: func() (*AssistHandlerFactory, *CommandInput, *mockActor) {
				f := &AssistHandlerFactory{combat: &mockCombatManager{}, players: &mockAssistPlayerLookup{}, pub: &recordingPublisher{}}
				actor := &mockActor{id: "alice", name: "Alice", inCombat: true}
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
			setup: func() (*AssistHandlerFactory, *CommandInput, *mockActor) {
				f := &AssistHandlerFactory{combat: &mockCombatManager{}, players: &mockAssistPlayerLookup{}, pub: &recordingPublisher{}}
				actor := &mockActor{id: "alice", name: "Alice"}
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
			setup: func() (*AssistHandlerFactory, *CommandInput, *mockActor) {
				bob := &mockActor{id: "bob", name: "Bob"}
				f := &AssistHandlerFactory{combat: &mockCombatManager{}, players: &mockAssistPlayerLookup{players: map[string]AssistedPlayer{"bob": bob}}, pub: &recordingPublisher{}}
				actor := &mockActor{id: "alice", name: "Alice"}
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

			cm := factory.combat.(*mockCombatManager)
			if cm.started != tt.expStarted {
				t.Errorf("StartCombat called = %v, want %v", cm.started, tt.expStarted)
			}

			pub := factory.pub.(*recordingPublisher)
			actorId := actor.Id()

			if tt.expMsgActor != "" {
				if !containsSubstring(actor.notified, tt.expMsgActor) {
					t.Errorf("expected Notify to actor containing %q, got %v", tt.expMsgActor, actor.notified)
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
