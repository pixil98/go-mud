package combat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
)

// Side identifies which team a combatant is on.
type Side int

const (
	SidePlayer Side = iota
	SideMob
)

// Attack describes a single attack a combatant can make per round.
type Attack struct {
	Mod         int // added to d20 roll
	DamageDice  int // number of dice
	DamageSides int // sides per die
	DamageMod   int // added to damage roll
}

// Combatant is anything that can participate in combat.
type Combatant interface {
	CombatID() string
	CombatName() string
	CombatSide() Side
	IsAlive() bool
	AC() int
	Attacks() []Attack
	ApplyDamage(int)
	SetInCombat(bool)
	Level() int
}

// DeathContext provides fight information to the event handler when a combatant dies.
type DeathContext struct {
	Victim    Combatant
	ZoneID    string
	RoomID    string
	Opponents []Combatant      // surviving (or recently alive) members of the opposing side
	DamageBy  map[string]int   // combatant ID -> total damage dealt to victim
}

// EventHandler handles combat events that require game-level logic.
type EventHandler interface {
	OnDeath(ctx DeathContext)
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
	// Damage tracks cumulative damage: attacker CombatID -> victim CombatID -> total.
	Damage map[string]map[string]int
}

// DamageToVictim returns a map of combatant ID -> total damage dealt to the given victim.
func (f *Fight) DamageToVictim(victimID string) map[string]int {
	result := make(map[string]int)
	for attackerID, victims := range f.Damage {
		if dmg, ok := victims[victimID]; ok {
			result[attackerID] = dmg
		}
	}
	return result
}

// fightSide identifies one of the two sides in a fight.
type fightSide int

const (
	sideA fightSide = iota
	sideB
)

func (s fightSide) opposite() fightSide {
	if s == sideA {
		return sideB
	}
	return sideA
}

// Manager tracks all active fights and processes combat rounds.
type Manager struct {
	mu         sync.Mutex
	pub        game.Publisher
	world      *game.WorldState
	handler    EventHandler
	combatants map[string]*Fight              // combatant ID -> their fight
	fights     map[string]map[string][]*Fight // zone -> room -> fights
}

// NewManager creates a new combat Manager.
func NewManager(pub game.Publisher, world *game.WorldState, handler EventHandler) *Manager {
	return &Manager{
		pub:        pub,
		world:      world,
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
	if f := fight.findFighter(sideA, id); f != nil {
		return f
	}
	return fight.findFighter(sideB, id)
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
		joinSide := fight.sideOf(targetID).opposite()
		*fight.side(joinSide) = append(*fight.side(joinSide), af)
		m.combatants[attackerID] = fight
	} else {
		// Create a new fight.
		tf := &Fighter{Combatant: target, Target: attacker}
		fight := &Fight{
			ZoneID: zoneID,
			RoomID: roomID,
			SideA:  []*Fighter{af},
			SideB:  []*Fighter{tf},
			Damage: make(map[string]map[string]int),
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

	var deferred []DeathContext
	for _, rooms := range m.fights {
		for _, fights := range rooms {
			for _, fight := range fights {
				msgs := fight.Process()
				deathMsgs, deathContexts := m.processFightDeaths(fight)
				msgs = append(msgs, deathMsgs...)
				if len(msgs) > 0 {
					ri := m.world.Instances()[fight.ZoneID].GetRoom(fight.RoomID)
					if err := m.pub.Publish(ri, nil, []byte(strings.Join(msgs, "\n"))); err != nil {
						return fmt.Errorf("publishing combat messages: %w", err)
					}
				}
				deferred = append(deferred, deathContexts...)
			}
		}
	}

	// Handle death effects (corpse creation, XP awards, player respawn)
	// after all room broadcasts so R.I.P. messages arrive first.
	for _, dctx := range deferred {
		m.handler.OnDeath(dctx)
	}

	m.cleanupFights()
	return nil
}

// Process runs one round of combat for this fight, returning combat messages.
func (f *Fight) Process() []string {
	var msgs []string
	for _, fighter := range f.SideA {
		msgs = append(msgs, f.processAttacks(fighter)...)
	}
	for _, fighter := range f.SideB {
		msgs = append(msgs, f.processAttacks(fighter)...)
	}
	return msgs
}

func (f *Fight) processAttacks(fighter *Fighter) []string {
	if !fighter.Combatant.IsAlive() {
		return nil
	}

	if fighter.Target == nil || !fighter.Target.IsAlive() {
		fighter.Target = f.pickTarget(fighter)
		if fighter.Target == nil {
			return nil
		}
	}

	slog.Debug("performing attack",
		"zone", f.ZoneID, "room", f.RoomID,
		"name", fighter.Combatant.CombatName())

	attacker := fighter.Combatant
	target := fighter.Target
	targetAC := target.AC()

	var msgs []string
	for _, atk := range attacker.Attacks() {
		if !target.IsAlive() {
			break
		}

		attackRoll := RollAttack(atk.Mod)
		var damage int
		if attackRoll >= targetAC {
			damage = RollDamage(atk.DamageDice, atk.DamageSides, atk.DamageMod)
			target.ApplyDamage(damage)

			attackerID := attacker.CombatID()
			if f.Damage[attackerID] == nil {
				f.Damage[attackerID] = make(map[string]int)
			}
			f.Damage[attackerID][target.CombatID()] += damage
		}

		verb := DamageVerb(damage)
		msgs = append(msgs, fmt.Sprintf("%s %s %s!", display.Capitalize(attacker.CombatName()), verb, target.CombatName()))
	}
	return msgs
}

func (f *Fight) pickTarget(fighter *Fighter) Combatant {
	return f.pickLiving(f.sideOf(fighter.CombatID()).opposite())
}

func (m *Manager) processFightDeaths(fight *Fight) ([]string, []DeathContext) {
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
		return nil, nil
	}

	// Snapshot both sides before removing the dead so OnDeath can see
	// who participated (sides are cleared when one side is wiped out).
	sideACombatants := make([]Combatant, len(fight.SideA))
	for i, f := range fight.SideA {
		sideACombatants[i] = f.Combatant
	}
	sideBCombatants := make([]Combatant, len(fight.SideB))
	for i, f := range fight.SideB {
		sideBCombatants[i] = f.Combatant
	}

	// Record which side each dead combatant was on before removal.
	deadSide := make(map[string]fightSide, len(dead))
	for _, victim := range dead {
		deadSide[victim.CombatID()] = fight.sideOf(victim.CombatID())
	}

	// First pass: remove dead from fight sides and update all combat state.
	// This must complete before OnDeath so that side effects (e.g. room
	// descriptions sent during player respawn) reflect the correct state.
	deadSet := make(map[string]bool, len(dead))
	var msgs []string
	for _, victim := range dead {
		id := victim.CombatID()
		deadSet[id] = true
		msgs = append(msgs, display.Colorize(display.Color.Red, fmt.Sprintf("%s is dead! R.I.P.", display.Capitalize(victim.CombatName()))))
		fight.removeFighter(id)
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
				f.Target = fight.pickLiving(sideB)
			}
		}
		for _, f := range fight.SideB {
			if f.Target == nil || deadSet[f.Target.CombatID()] {
				f.Target = fight.pickLiving(sideA)
			}
		}
	}

	// Build death contexts for deferred handling. These are returned to the
	// caller so that OnDeath runs after the room broadcast, ensuring R.I.P.
	// messages are delivered before XP awards and player respawn effects.
	var deathContexts []DeathContext
	for _, victim := range dead {
		// Opponents are the pre-removal snapshot of the opposite side.
		var opponents []Combatant
		if deadSide[victim.CombatID()] == sideA {
			opponents = sideBCombatants
		} else {
			opponents = sideACombatants
		}

		deathContexts = append(deathContexts, DeathContext{
			Victim:    victim,
			ZoneID:    fight.ZoneID,
			RoomID:    fight.RoomID,
			Opponents: opponents,
			DamageBy:  fight.DamageToVictim(victim.CombatID()),
		})
	}

	return msgs, deathContexts
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

func (f *Fight) side(s fightSide) *[]*Fighter {
	if s == sideA {
		return &f.SideA
	}
	return &f.SideB
}

func (f *Fight) sideOf(id string) fightSide {
	if f.findFighter(sideA, id) != nil {
		return sideA
	}
	return sideB
}

func (f *Fight) findFighter(s fightSide, id string) *Fighter {
	for _, ft := range *f.side(s) {
		if ft.CombatID() == id {
			return ft
		}
	}
	return nil
}

func (f *Fight) removeFighter(id string) {
	for _, s := range [2]fightSide{sideA, sideB} {
		side := f.side(s)
		for i, ft := range *side {
			if ft.CombatID() == id {
				*side = append((*side)[:i], (*side)[i+1:]...)
				return
			}
		}
	}
}

func (f *Fight) pickLiving(s fightSide) Combatant {
	for _, ft := range *f.side(s) {
		if ft.Combatant.IsAlive() {
			return ft.Combatant
		}
	}
	return nil
}
