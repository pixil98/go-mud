package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/game"
)

// mockCombatManager is a test double for CombatManager.
type mockCombatManager struct {
	startedErr   error // error to return from StartCombat
	started      bool
	lastAttacker combat.Combatant
	lastTarget   combat.Combatant
	queued       bool
	threatAdded  bool
	threatAmount int
}

func (m *mockCombatManager) StartCombat(attacker, target combat.Combatant) error {
	m.started = true
	m.lastAttacker = attacker
	m.lastTarget = target
	return m.startedErr
}

func (m *mockCombatManager) AddThreat(_, _ combat.Combatant, amount int) {
	m.threatAdded = true
	m.threatAmount = amount
}

func (m *mockCombatManager) QueueAttack(_ combat.Combatant) {
	m.queued = true
}

func TestAssistHandler(t *testing.T) {
	tests := map[string]struct {
		setup          func() (*AssistHandlerFactory, *CommandInput)
		expErr         string
		expStarted     bool
		expMsgActor    string // substring in message to actor
		expMsgAssisted string // substring in message to assisted
		expMsgRoom     string // substring in message to room bystander
	}{
		"assist explicit target in combat": {
			setup: func() (*AssistHandlerFactory, *CommandInput) {
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

				actor := newTestPlayer("alice", "Alice", room)
				bob := newTestPlayer("bob", "Bob", room)
				bob.SetCombatTargetId("mob:test-mob")

				players := &mockPlayerLookup{players: map[string]*game.CharacterInstance{"bob": bob}}
				f := NewAssistHandlerFactory(cm, &mockZoneLocator{zones: map[string]*game.ZoneInstance{"test-zone": zone}}, players, pub)

				cmdCtx := &CommandInput{
					Char: actor,
					Targets: map[string]*TargetRef{
						"target": {Type: targetTypePlayer, Player: &PlayerRef{CharId: "bob", Name: "Bob"}},
					},
					Config: make(map[string]string),
				}
				return f, cmdCtx
			},
			expStarted:     true,
			expMsgActor:    "You jump to Bob's aid!",
			expMsgAssisted: "Alice jumps to your aid!",
			expMsgRoom:     "Alice jumps to Bob's aid!",
		},
		"assist follow leader": {
			setup: func() (*AssistHandlerFactory, *CommandInput) {
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

				bob := newTestPlayer("bob", "Bob", room)
				bob.SetCombatTargetId("mob:test-mob")

				players := &mockPlayerLookup{players: map[string]*game.CharacterInstance{"bob": bob}}
				f := NewAssistHandlerFactory(cm, &mockZoneLocator{zones: map[string]*game.ZoneInstance{"test-zone": zone}}, players, pub)

				actor := newTestPlayer("alice", "Alice", room)
				actor.SetFollowingId("bob")

				cmdCtx := &CommandInput{
					Char:    actor,
					Targets: map[string]*TargetRef{},
					Config:  make(map[string]string),
				}
				return f, cmdCtx
			},
			expStarted:     true,
			expMsgActor:    "You jump to Bob's aid!",
			expMsgAssisted: "Alice jumps to your aid!",
		},
		"already in combat": {
			setup: func() (*AssistHandlerFactory, *CommandInput) {
				cm := &mockCombatManager{}
				pub := &recordingPublisher{}
				players := &mockPlayerLookup{}
				f := NewAssistHandlerFactory(cm, &mockZoneLocator{}, players, pub)

				actor := newCharacterInstance("alice", "Alice")
				actor.SetInCombat(true)
				cmdCtx := &CommandInput{
					Char:    actor,
					Targets: map[string]*TargetRef{},
					Config:  make(map[string]string),
				}
				return f, cmdCtx
			},
			expErr: "already fighting",
		},
		"no target and not following": {
			setup: func() (*AssistHandlerFactory, *CommandInput) {
				cm := &mockCombatManager{}
				pub := &recordingPublisher{}
				players := &mockPlayerLookup{}
				f := NewAssistHandlerFactory(cm, &mockZoneLocator{}, players, pub)

				actor := newCharacterInstance("alice", "Alice")
				cmdCtx := &CommandInput{
					Char:    actor,
					Targets: map[string]*TargetRef{},
					Config:  make(map[string]string),
				}
				return f, cmdCtx
			},
			expErr: "Assist whom?",
		},
		"assisted player not in combat": {
			setup: func() (*AssistHandlerFactory, *CommandInput) {
				cm := &mockCombatManager{}
				pub := &recordingPublisher{}
				bob := newCharacterInstance("bob", "Bob") // GetCombatTargetId returns ""
				players := &mockPlayerLookup{players: map[string]*game.CharacterInstance{"bob": bob}}
				f := NewAssistHandlerFactory(cm, &mockZoneLocator{}, players, pub)

				actor := newCharacterInstance("alice", "Alice")
				cmdCtx := &CommandInput{
					Char: actor,
					Targets: map[string]*TargetRef{
						"target": {Type: targetTypePlayer, Player: &PlayerRef{CharId: "bob", Name: "Bob"}},
					},
					Config: make(map[string]string),
				}
				return f, cmdCtx
			},
			expErr: "isn't fighting anyone",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			factory, cmdCtx := tt.setup()
			cmdFunc, err := factory.Create()
			if err != nil {
				t.Fatalf("Create() error: %v", err)
			}

			err = cmdFunc(context.Background(), cmdCtx)

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
			actorId := cmdCtx.Char.Id()

			if tt.expMsgActor != "" {
				msgs := pub.messagesTo(string(actorId))
				if !containsSubstring(msgs, tt.expMsgActor) {
					t.Errorf("expected message to actor containing %q, got %v", tt.expMsgActor, msgs)
				}
			}
			if tt.expMsgAssisted != "" {
				var assistedId string
				if ref := cmdCtx.Targets["target"]; ref != nil {
					assistedId = ref.Player.CharId
				} else {
					assistedId = cmdCtx.Char.GetFollowingId()
				}
				msgs := pub.messagesTo(assistedId)
				if !containsSubstring(msgs, tt.expMsgAssisted) {
					t.Errorf("expected message to assisted containing %q, got %v", tt.expMsgAssisted, msgs)
				}
			}
			if tt.expMsgRoom != "" {
				found := false
				for _, m := range pub.messages {
					if m.targetId != string(actorId) && strings.Contains(m.data, tt.expMsgRoom) {
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
