package combat

import (
	"context"
	"sync"

	"github.com/pixil98/go-mud/internal/game"
)

// Manager runs the combat loop and tracks threat relationships between combatants.
type Manager struct {
	mu         sync.Mutex
	combatants map[string]*combatantState
	pub        game.Publisher
}

type combatantState struct {
	c      Combatant
	threat map[string]int
}

// NewManager creates a combat Manager.
func NewManager(pub game.Publisher) *Manager {
	return &Manager{
		combatants: make(map[string]*combatantState),
		pub:        pub,
	}
}

// StartCombat registers both combatants in the manager and initialises mutual threat.
// It is idempotent: calling it on combatants already in combat only adds missing threat entries.
func (m *Manager) StartCombat(attacker, target Combatant) error {
	return nil
}

// AddThreat increases the threat that source has generated toward target.
func (m *Manager) AddThreat(source, target Combatant, amount int) {}

// Flee removes a combatant from active combat without clearing their threat entries
// in enemies' tables, preserving their aggro position if they re-enter the fight.
func (m *Manager) Flee(c Combatant) {}

// Tick processes one round of combat for all active combatants.
func (m *Manager) Tick(_ context.Context) error {
	return nil
}
