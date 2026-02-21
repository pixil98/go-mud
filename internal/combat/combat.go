package combat

import (
	"context"
	"fmt"
	"sync"
)

// Side identifies which team a combatant is on for retargeting.
type Side int

const (
	SidePlayer Side = iota
	SideMob
)

// Combatant is anything that can participate in combat.
type Combatant interface {
	CombatID() string
	CombatName() string
	CombatSide() Side
	IsAlive() bool
	AC() int
	AttackMod() int
	DamageDice() int
	DamageSides() int
	DamageMod() int
	ApplyDamage(int)
	SetInCombat(bool)
}

// MessagePublisher sends combat messages.
type MessagePublisher interface {
	SendToRoom(zoneID, roomID string, msg string)
	SendToPlayer(id string, msg string)
}

// EventHandler handles combat events that require game-level logic.
type EventHandler interface {
	OnDeath(victim Combatant, zoneID, roomID string)
}

// Fighter is one entry in the combat list.
type Fighter struct {
	Combatant Combatant
	Target    Combatant
	ZoneID    string
	RoomID    string
}

// Manager tracks all active combatants and processes combat rounds.
type Manager struct {
	mu       sync.Mutex
	pub      MessagePublisher
	handler  EventHandler
	fighters map[string]*Fighter
}

// NewManager creates a new combat Manager.
func NewManager(pub MessagePublisher, handler EventHandler) *Manager {
	return &Manager{
		pub:      pub,
		handler:  handler,
		fighters: make(map[string]*Fighter),
	}
}

// IsInCombat returns true if the given combatant ID is in the fighter list.
func (m *Manager) IsInCombat(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.fighters[id]
	return ok
}

// GetFighter returns the Fighter for the given combatant ID, or nil.
func (m *Manager) GetFighter(id string) *Fighter {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fighters[id]
}

// StartCombat adds an attacker to the combat list targeting the given target.
// If the target is not already fighting, it is also added targeting the attacker.
func (m *Manager) StartCombat(attacker, target Combatant, zoneID, roomID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	attackerID := attacker.CombatID()
	if _, exists := m.fighters[attackerID]; exists {
		return fmt.Errorf("already in combat")
	}

	m.fighters[attackerID] = &Fighter{
		Combatant: attacker,
		Target:    target,
		ZoneID:    zoneID,
		RoomID:    roomID,
	}
	attacker.SetInCombat(true)

	targetID := target.CombatID()
	if _, exists := m.fighters[targetID]; !exists {
		m.fighters[targetID] = &Fighter{
			Combatant: target,
			Target:    attacker,
			ZoneID:    zoneID,
			RoomID:    roomID,
		}
		target.SetInCombat(true)
	}

	return nil
}

// StopCombat removes a combatant from combat by ID.
func (m *Manager) StopCombat(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopFighting(id)
}

// StopAllInRoom removes all fighters in the given room.
func (m *Manager) StopAllInRoom(zoneID, roomID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, f := range m.fighters {
		if f.ZoneID == zoneID && f.RoomID == roomID {
			m.stopFighting(id)
		}
	}
}

func (m *Manager) stopFighting(id string) {
	fighter, ok := m.fighters[id]
	if !ok {
		return
	}
	fighter.Combatant.SetInCombat(false)
	delete(m.fighters, id)
}

// Tick processes one combat round. Called every tick by the MudDriver.
func (m *Manager) Tick(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.fighters) == 0 {
		return nil
	}

	// Snapshot keys for safe iteration during removal.
	keys := make([]string, 0, len(m.fighters))
	for k := range m.fighters {
		keys = append(keys, k)
	}

	for _, key := range keys {
		fighter, ok := m.fighters[key]
		if !ok {
			continue
		}
		if !fighter.Combatant.IsAlive() {
			continue
		}

		if fighter.Target == nil || !fighter.Target.IsAlive() {
			m.retarget(fighter)
			if fighter.Target == nil {
				m.stopFighting(key)
				continue
			}
		}

		m.performAttack(fighter)
	}

	m.processDeaths()

	return nil
}

func (m *Manager) performAttack(fighter *Fighter) {
	attacker := fighter.Combatant
	target := fighter.Target

	attackRoll := RollAttack(attacker.AttackMod())
	targetAC := target.AC()

	var damage int
	if attackRoll >= targetAC {
		damage = RollDamage(attacker.DamageDice(), attacker.DamageSides(), attacker.DamageMod())
		target.ApplyDamage(damage)
	}

	verb := DamageVerb(damage)
	msg := fmt.Sprintf("%s %s %s!", attacker.CombatName(), verb, target.CombatName())
	m.pub.SendToRoom(fighter.ZoneID, fighter.RoomID, msg)
}

func (m *Manager) processDeaths() {
	var deadKeys []string
	for key, fighter := range m.fighters {
		if !fighter.Combatant.IsAlive() {
			deadKeys = append(deadKeys, key)
		}
	}

	for _, key := range deadKeys {
		fighter := m.fighters[key]
		victim := fighter.Combatant

		m.handler.OnDeath(victim, fighter.ZoneID, fighter.RoomID)
		m.stopFighting(key)

		// Clear target references to the dead combatant.
		for _, f := range m.fighters {
			if f.Target == victim {
				f.Target = nil
			}
		}
	}

	// Retarget pass: any fighter with nil target.
	for key, f := range m.fighters {
		if f.Target == nil {
			m.retarget(f)
			if f.Target == nil {
				m.stopFighting(key)
			}
		}
	}
}

// retarget picks a new target from the same fight. To keep disjoint fights
// separate, it only considers enemies that are targeting an ally of the fighter
// (i.e. someone connected to the same engagement).
func (m *Manager) retarget(fighter *Fighter) {
	allies := m.fightAllies(fighter)

	for _, other := range m.fighters {
		if other.Combatant.CombatSide() == fighter.Combatant.CombatSide() {
			continue
		}
		if other.ZoneID != fighter.ZoneID || other.RoomID != fighter.RoomID {
			continue
		}
		if !other.Combatant.IsAlive() {
			continue
		}
		// Only retarget to enemies connected to our fight.
		if other.Target != nil && allies[other.Target.CombatID()] {
			fighter.Target = other.Combatant
			return
		}
	}

	fighter.Target = nil
}

// fightAllies returns the set of combatant IDs on the same side in the same
// room as the given fighter, including the fighter itself.
func (m *Manager) fightAllies(fighter *Fighter) map[string]bool {
	allies := map[string]bool{fighter.Combatant.CombatID(): true}
	for _, f := range m.fighters {
		if f.Combatant.CombatSide() == fighter.Combatant.CombatSide() &&
			f.ZoneID == fighter.ZoneID && f.RoomID == fighter.RoomID {
			allies[f.Combatant.CombatID()] = true
		}
	}
	return allies
}
