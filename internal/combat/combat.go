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

// PlayerMessage is a message targeted at a specific player by ID.
type PlayerMessage struct {
	PlayerID string
	Text     string
}

// EventHandler handles death side effects after room messages are delivered
// (corpse creation, player respawn, etc.).
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

// GetPlayerFighter returns the Fighter for a player character ID, or nil.
func (m *Manager) GetPlayerFighter(charId string) *Fighter {
	return m.getFighter(fmt.Sprintf("player:%s", charId))
}

// getFighter returns the Fighter for the given combatant ID, or nil.
func (m *Manager) getFighter(id string) *Fighter {
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

// fightResult holds the output of processing one fight round.
type fightResult struct {
	zoneID     string
	roomID     string
	roomMsgs   []string        // broadcast to all players in the room
	playerMsgs []PlayerMessage // targeted at specific players (e.g. XP awards)
	deaths     []DeathContext  // for deferred death side effects
}

// Tick processes one combat round in three phases:
//  1. Compute — process attacks, resolve deaths, award XP.
//  2. Deliver — send each player a single combined message.
//  3. Effects — run death side effects (corpse creation, respawn).
func (m *Manager) Tick(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []fightResult
	for _, rooms := range m.fights {
		for _, fights := range rooms {
			for _, fight := range fights {
				results = append(results, m.processFight(fight))
			}
		}
	}

	for _, r := range results {
		m.deliver(r)
	}

	for _, r := range results {
		for _, dctx := range r.deaths {
			m.handler.OnDeath(dctx)
		}
	}

	m.cleanupFights()
	return nil
}

// processFight runs one round for a fight and returns the result.
func (m *Manager) processFight(fight *Fight) fightResult {
	r := fightResult{zoneID: fight.ZoneID, roomID: fight.RoomID}
	r.roomMsgs = fight.process()
	deathMsgs, deaths := m.processFightDeaths(fight)
	r.roomMsgs = append(r.roomMsgs, deathMsgs...)
	r.deaths = deaths
	r.playerMsgs = computeKillRewards(deaths)
	return r
}

// deliver sends each player in the fight room a single combined message.
func (m *Manager) deliver(r fightResult) {
	if len(r.roomMsgs) == 0 && len(r.playerMsgs) == 0 {
		return
	}
	ri := m.world.Instances()[r.zoneID].GetRoom(r.roomID)
	if ri == nil {
		return
	}

	roomText := strings.Join(r.roomMsgs, "\n")
	extra := make(map[string][]string, len(r.playerMsgs))
	for _, pm := range r.playerMsgs {
		extra[pm.PlayerID] = append(extra[pm.PlayerID], pm.Text)
	}

	ri.ForEachPlayer(func(id string, _ *game.PlayerState) {
		var parts []string
		if roomText != "" {
			parts = append(parts, roomText)
		}
		parts = append(parts, extra[id]...)
		if len(parts) > 0 {
			if err := m.pub.Publish(game.SinglePlayer(id), nil, []byte(strings.Join(parts, "\n"))); err != nil {
				slog.Error("publishing combat message", "player", id, "err", err)
			}
		}
	})
}

// computeKillRewards calculates XP for all mob deaths in a fight round and
// returns per-player messages. Called during the compute phase so rewards
// arrive in the same delivery as the kill messages.
func computeKillRewards(deaths []DeathContext) []PlayerMessage {
	var msgs []PlayerMessage
	for _, dctx := range deaths {
		mob, ok := dctx.Victim.(*MobCombatant)
		if !ok {
			continue
		}
		var participants []game.XPParticipant
		for _, opp := range dctx.Opponents {
			pc, ok := opp.(*PlayerCombatant)
			if !ok {
				continue
			}
			participants = append(participants, game.XPParticipant{
				CombatID: pc.CombatID(),
				Level:    pc.Level(),
				Damage:   dctx.DamageBy[pc.CombatID()],
			})
		}
		awards := game.CalculateXPAwards(mob.Level(), mob.ExpReward(), participants)
		for _, award := range awards {
			for _, opp := range dctx.Opponents {
				pc, ok := opp.(*PlayerCombatant)
				if !ok || pc.CombatID() != award.CombatID {
					continue
				}
				if msg := pc.AwardXP(award.Amount); msg != "" {
					msgs = append(msgs, PlayerMessage{PlayerID: pc.Character.Id(), Text: msg})
				}
			}
		}
	}
	return msgs
}

// process runs one round of combat for this fight, returning attack messages.
func (f *Fight) process() []string {
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
