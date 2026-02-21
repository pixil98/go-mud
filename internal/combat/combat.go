package combat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/pixil98/go-mud/internal/display"
)

// Side identifies which team a combatant is on.
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

// Fighter is one entry in a fight.
type Fighter struct {
	Combatant
	Target Combatant
}

// Fight represents a single engagement between two sides in a room.
type Fight struct {
	ZoneID string
	RoomID string
	SideA  []*Fighter
	SideB  []*Fighter
}

// Manager tracks all active fights and processes combat rounds.
type Manager struct {
	mu         sync.Mutex
	pub        MessagePublisher
	handler    EventHandler
	combatants map[string]*Fight              // combatant ID -> their fight
	fights     map[string]map[string][]*Fight // zone -> room -> fights
}

// NewManager creates a new combat Manager.
func NewManager(pub MessagePublisher, handler EventHandler) *Manager {
	return &Manager{
		pub:        pub,
		handler:    handler,
		combatants: make(map[string]*Fight),
		fights:     make(map[string]map[string][]*Fight),
	}
}

// GetFighter returns the Fighter for the given combatant ID, or nil.
func (m *Manager) GetFighter(id string) *Fighter {
	m.mu.Lock()
	defer m.mu.Unlock()
	fight, ok := m.combatants[id]
	if !ok {
		return nil
	}
	if f := findFighter(fight.SideA, id); f != nil {
		return f
	}
	return findFighter(fight.SideB, id)
}

// StartCombat begins combat between attacker and target in the given room.
// If the target is already in a fight, the attacker joins the opposite side.
func (m *Manager) StartCombat(attacker, target Combatant, zoneID, roomID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	attackerID := attacker.CombatID()
	if _, exists := m.combatants[attackerID]; exists {
		return fmt.Errorf("already in combat")
	}

	af := &Fighter{Combatant: attacker, Target: target}

	targetID := target.CombatID()
	if fight, exists := m.combatants[targetID]; exists {
		// Join the opposite side of the target's existing fight.
		if isOnSideA(fight, targetID) {
			fight.SideB = append(fight.SideB, af)
		} else {
			fight.SideA = append(fight.SideA, af)
		}
		m.combatants[attackerID] = fight
	} else {
		// Create a new fight.
		tf := &Fighter{Combatant: target, Target: attacker}
		fight := &Fight{
			ZoneID: zoneID,
			RoomID: roomID,
			SideA:  []*Fighter{af},
			SideB:  []*Fighter{tf},
		}
		m.combatants[attackerID] = fight
		m.combatants[targetID] = fight
		m.addFight(fight)
		target.SetInCombat(true)
	}

	attacker.SetInCombat(true)
	return nil
}

// Tick processes one combat round.
func (m *Manager) Tick(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, rooms := range m.fights {
		for _, fights := range rooms {
			for _, fight := range fights {
				msgs := m.processFight(fight)
				deathMsgs := m.processFightDeaths(fight)
				msgs = append(msgs, deathMsgs...)
				if len(msgs) > 0 {
					m.pub.SendToRoom(fight.ZoneID, fight.RoomID, strings.Join(msgs, "\n"))
				}
			}
		}
	}

	m.cleanupFights()
	return nil
}

func (m *Manager) processFight(fight *Fight) []string {
	var msgs []string
	for _, f := range fight.SideA {
		if msg := m.processAttack(f, fight); msg != "" {
			msgs = append(msgs, msg)
		}
	}
	for _, f := range fight.SideB {
		if msg := m.processAttack(f, fight); msg != "" {
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

func (m *Manager) processAttack(fighter *Fighter, fight *Fight) string {
	if !fighter.Combatant.IsAlive() {
		return ""
	}

	if fighter.Target == nil || !fighter.Target.IsAlive() {
		fighter.Target = m.pickTarget(fighter, fight)
		if fighter.Target == nil {
			return ""
		}
	}

	slog.Debug("performing attack",
		"zone", fight.ZoneID, "room", fight.RoomID,
		"name", fighter.Combatant.CombatName())

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
	return fmt.Sprintf("%s %s %s!", display.Capitalize(attacker.CombatName()), verb, target.CombatName())
}

func (m *Manager) pickTarget(fighter *Fighter, fight *Fight) Combatant {
	enemies := fight.SideB
	if !isOnSideA(fight, fighter.CombatID()) {
		enemies = fight.SideA
	}
	return pickLiving(enemies)
}

func (m *Manager) processFightDeaths(fight *Fight) []string {
	var dead []Combatant
	for _, f := range fight.SideA {
		if !f.Combatant.IsAlive() {
			dead = append(dead, f.Combatant)
		}
	}
	for _, f := range fight.SideB {
		if !f.Combatant.IsAlive() {
			dead = append(dead, f.Combatant)
		}
	}

	if len(dead) == 0 {
		return nil
	}

	// First pass: remove dead from fight sides and update all combat state.
	// This must complete before OnDeath so that side effects (e.g. room
	// descriptions sent during player respawn) reflect the correct state.
	deadSet := make(map[string]bool, len(dead))
	var msgs []string
	for _, victim := range dead {
		id := victim.CombatID()
		deadSet[id] = true
		msgs = append(msgs, display.Colorize(display.Red, fmt.Sprintf("%s is dead! R.I.P.", display.Capitalize(victim.CombatName()))))
		fight.SideA = removeFighter(fight.SideA, id)
		fight.SideB = removeFighter(fight.SideB, id)
		victim.SetInCombat(false)
		delete(m.combatants, id)
	}

	// If either side is now empty, end combat for the remaining side.
	if len(fight.SideA) == 0 || len(fight.SideB) == 0 {
		for _, f := range fight.SideA {
			f.Combatant.SetInCombat(false)
			delete(m.combatants, f.CombatID())
		}
		for _, f := range fight.SideB {
			f.Combatant.SetInCombat(false)
			delete(m.combatants, f.CombatID())
		}
		fight.SideA = nil
		fight.SideB = nil
	} else {
		// Retarget fighters whose target died or is missing.
		for _, f := range fight.SideA {
			if f.Target == nil || deadSet[f.Target.CombatID()] {
				f.Target = pickLiving(fight.SideB)
			}
		}
		for _, f := range fight.SideB {
			if f.Target == nil || deadSet[f.Target.CombatID()] {
				f.Target = pickLiving(fight.SideA)
			}
		}
	}

	// Second pass: handle death effects (corpse creation, player respawn).
	// Runs after all combat state is cleaned up so room descriptions
	// generated by OnDeath show the correct InCombat flags.
	for _, victim := range dead {
		m.handler.OnDeath(victim, fight.ZoneID, fight.RoomID)
	}

	return msgs
}

// cleanupFights removes fights where either side is empty and stops all
// remaining combatants in those fights.
func (m *Manager) cleanupFights() {
	for zoneID, rooms := range m.fights {
		for roomID, fights := range rooms {
			var active []*Fight
			for _, fight := range fights {
				if len(fight.SideA) == 0 || len(fight.SideB) == 0 {
					for _, f := range fight.SideA {
						f.Combatant.SetInCombat(false)
						delete(m.combatants, f.CombatID())
					}
					for _, f := range fight.SideB {
						f.Combatant.SetInCombat(false)
						delete(m.combatants, f.CombatID())
					}
				} else {
					active = append(active, fight)
				}
			}
			if len(active) == 0 {
				delete(rooms, roomID)
			} else {
				rooms[roomID] = active
			}
		}
		if len(rooms) == 0 {
			delete(m.fights, zoneID)
		}
	}
}

func (m *Manager) addFight(fight *Fight) {
	if m.fights[fight.ZoneID] == nil {
		m.fights[fight.ZoneID] = make(map[string][]*Fight)
	}
	m.fights[fight.ZoneID][fight.RoomID] = append(m.fights[fight.ZoneID][fight.RoomID], fight)
}

func isOnSideA(fight *Fight, id string) bool {
	return findFighter(fight.SideA, id) != nil
}

func findFighter(fighters []*Fighter, id string) *Fighter {
	for _, f := range fighters {
		if f.CombatID() == id {
			return f
		}
	}
	return nil
}

func removeFighter(fighters []*Fighter, id string) []*Fighter {
	for i, f := range fighters {
		if f.CombatID() == id {
			return append(fighters[:i], fighters[i+1:]...)
		}
	}
	return fighters
}

func filterAlive(fighters []*Fighter) []*Fighter {
	var alive []*Fighter
	for _, f := range fighters {
		if f.Combatant.IsAlive() {
			alive = append(alive, f)
		}
	}
	return alive
}

func pickLiving(fighters []*Fighter) Combatant {
	for _, f := range fighters {
		if f.Combatant.IsAlive() {
			return f.Combatant
		}
	}
	return nil
}
