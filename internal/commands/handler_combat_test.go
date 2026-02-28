package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// mockCombatManager is a test double for CombatManager.
type mockCombatManager struct {
	fighters   map[string]*combat.Fighter // charId -> fighter
	startedErr error                      // error to return from StartCombat
	started    bool                       // set to true when StartCombat is called
}

func (m *mockCombatManager) StartCombat(attacker, target combat.Combatant, zoneID, roomID string) error {
	m.started = true
	return m.startedErr
}

func (m *mockCombatManager) GetPlayerFighter(charId string) *combat.Fighter {
	if m.fighters == nil {
		return nil
	}
	return m.fighters[charId]
}

// mockRoomLocator is a test double for RoomLocator.
type mockRoomLocator struct {
	room *game.RoomInstance
}

func (m *mockRoomLocator) GetRoom(zoneId, roomId string) *game.RoomInstance {
	return m.room
}

// mockCombatant is a minimal Combatant for test targets.
type mockCombatant struct {
	id   string
	name string
}

func (m *mockCombatant) CombatID() string         { return m.id }
func (m *mockCombatant) CombatName() string       { return m.name }
func (m *mockCombatant) IsAlive() bool            { return true }
func (m *mockCombatant) AC() int                  { return 10 }
func (m *mockCombatant) Attacks() []combat.Attack { return nil }
func (m *mockCombatant) ApplyDamage(int)          {}
func (m *mockCombatant) SetInCombat(bool)         {}
func (m *mockCombatant) Level() int               { return 1 }

func TestAssistHandler(t *testing.T) {
	tests := map[string]struct {
		setup          func() (*AssistHandlerFactory, *CommandContext)
		expErr         string
		expStarted     bool
		expMsgActor    string // substring in message to actor
		expMsgAssisted string // substring in message to assisted
		expMsgRoom     string // substring in message to room bystander
	}{
		"assist explicit target in combat": {
			setup: func() (*AssistHandlerFactory, *CommandContext) {
				mob := &mockCombatant{id: "mob:test-mob", name: "test-mob"}
				cm := &mockCombatManager{
					fighters: map[string]*combat.Fighter{
						"bob": {Target: mob},
					},
				}
				room, err := newTestRoom("test-room", "Test Room", "test-zone")
			if err != nil {
				t.Fatalf("failed to create test room: %v", err)
			}
				pub := &recordingPublisher{}
				players := &mockPlayerLookup{}
				f := NewAssistHandlerFactory(cm, &mockRoomLocator{room: room}, players, pub)

				actor := newTestPlayer("alice", "Alice", room)
				newTestPlayer("bob", "Bob", room)

				cmdCtx := &CommandContext{
					Actor:   actor.Character.Get(),
					Session: actor,
					Targets: map[string]*TargetRef{
						"target": {Type: TargetTypePlayer, Player: &PlayerRef{CharId: "bob", Name: "Bob"}},
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
			setup: func() (*AssistHandlerFactory, *CommandContext) {
				mob := &mockCombatant{id: "mob:test-mob", name: "test-mob"}
				cm := &mockCombatManager{
					fighters: map[string]*combat.Fighter{
						"bob": {Target: mob},
					},
				}
				room, err := newTestRoom("test-room", "Test Room", "test-zone")
			if err != nil {
				t.Fatalf("failed to create test room: %v", err)
			}
				pub := &recordingPublisher{}

				bob := newTestPlayer("bob", "Bob", room)
				players := &mockPlayerLookup{players: map[string]*game.PlayerState{"bob": bob}}
				f := NewAssistHandlerFactory(cm, &mockRoomLocator{room: room}, players, pub)

				actor := newTestPlayer("alice", "Alice", room)
				actor.FollowingId = "bob"

				cmdCtx := &CommandContext{
					Actor:   actor.Character.Get(),
					Session: actor,
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
			setup: func() (*AssistHandlerFactory, *CommandContext) {
				cm := &mockCombatManager{}
				pub := &recordingPublisher{}
				players := &mockPlayerLookup{}
				f := NewAssistHandlerFactory(cm, &mockRoomLocator{}, players, pub)

				actor := &game.PlayerState{
					Character: storage.NewResolvedSmartIdentifier("alice", &game.Character{Name: "Alice"}),
					InCombat:  true,
				}
				cmdCtx := &CommandContext{
					Actor:   actor.Character.Get(),
					Session: actor,
					Targets: map[string]*TargetRef{},
					Config:  make(map[string]string),
				}
				return f, cmdCtx
			},
			expErr: "already fighting",
		},
		"no target and not following": {
			setup: func() (*AssistHandlerFactory, *CommandContext) {
				cm := &mockCombatManager{}
				pub := &recordingPublisher{}
				players := &mockPlayerLookup{}
				f := NewAssistHandlerFactory(cm, &mockRoomLocator{}, players, pub)

				actor := &game.PlayerState{
					Character: storage.NewResolvedSmartIdentifier("alice", &game.Character{Name: "Alice"}),
				}
				cmdCtx := &CommandContext{
					Actor:   actor.Character.Get(),
					Session: actor,
					Targets: map[string]*TargetRef{},
					Config:  make(map[string]string),
				}
				return f, cmdCtx
			},
			expErr: "Assist whom?",
		},
		"assisted player not in combat": {
			setup: func() (*AssistHandlerFactory, *CommandContext) {
				cm := &mockCombatManager{} // no fighters
				pub := &recordingPublisher{}
				players := &mockPlayerLookup{}
				f := NewAssistHandlerFactory(cm, &mockRoomLocator{}, players, pub)

				actor := &game.PlayerState{
					Character: storage.NewResolvedSmartIdentifier("alice", &game.Character{Name: "Alice"}),
				}
				cmdCtx := &CommandContext{
					Actor:   actor.Character.Get(),
					Session: actor,
					Targets: map[string]*TargetRef{
						"target": {Type: TargetTypePlayer, Player: &PlayerRef{CharId: "bob", Name: "Bob"}},
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
			actorId := cmdCtx.Session.Character.Id()

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
					assistedId = cmdCtx.Session.FollowingId
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
