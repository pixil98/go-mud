package combat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// Manager runs the combat loop and tracks threat relationships between combatants.
type Manager struct {
	mu         sync.Mutex
	combatants map[string]*combatantState
	pub        game.Publisher
	zones      ZoneLocator
}

type combatantState struct {
	c             Combatant
	threat        map[string]int // enemy ID → accumulated threat
	attackPending bool           // fire one manual attack next tick
}

// AttackResult holds the outcome of a single attack roll.
type AttackResult struct {
	Damage    int
	Hit       bool
	Reflected int // damage reflected back to attacker (0 if none)
}

// NewManager creates a combat Manager.
func NewManager(pub game.Publisher, zones ZoneLocator) *Manager {
	return &Manager{
		combatants: make(map[string]*combatantState),
		pub:        pub,
		zones:      zones,
	}
}

// StartCombat registers both combatants and initialises mutual threat.
// It is idempotent: re-entering after flee preserves existing threat entries.
func (m *Manager) StartCombat(attacker, target Combatant) error {
	if !attacker.IsAlive() {
		return fmt.Errorf("%s is not alive", attacker.Name())
	}
	if !target.IsAlive() {
		return fmt.Errorf("%s is not alive", target.Name())
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	aState := m.register(attacker)
	tState := m.register(target)

	if _, ok := aState.threat[target.Id()]; !ok {
		aState.threat[target.Id()] = 1
	}
	if _, ok := tState.threat[attacker.Id()]; !ok {
		tState.threat[attacker.Id()] = 1
	}

	attacker.SetInCombat(true)
	target.SetInCombat(true)
	return nil
}

// QueueAttack marks a combatant to fire one manual attack on the next Tick.
// The target is resolved from the combatant's CombatTargetId or highest threat.
func (m *Manager) QueueAttack(c Combatant) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, ok := m.combatants[c.Id()]; ok {
		state.attackPending = true
	}
}

// AddThreat increases the threat that source has generated toward target.
func (m *Manager) AddThreat(source, target Combatant, amount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.register(source)
	tState := m.register(target)
	bonus := source.ModifierValue(assets.PerkKeyCombatThreatMod)
	tState.threat[source.Id()] += amount + bonus
}

// PerformAttack executes one attack roll per attack grant perk the attacker has,
// applies damage for each hit (including any reflected damage back to the attacker),
// and returns the per-attack results.
// Falls back to a single untyped 1d4 attack if no attack grants are present.
func PerformAttack(attacker, target Combatant) []AttackResult {
	attackArgs := attacker.GrantArgs(assets.PerkGrantAttack)
	if len(attackArgs) == 0 {
		attackArgs = []string{"1d4"}
	}

	results := make([]AttackResult, 0, len(attackArgs))
	for _, arg := range attackArgs {
		result := rollOneAttack(attacker, target, arg)
		if result.Hit {
			target.AdjustResource(assets.ResourceHp, -result.Damage)
			if result.Reflected > 0 {
				attacker.AdjustResource(assets.ResourceHp, -result.Reflected)
			}
		}
		results = append(results, result)
	}
	return results
}

// parseAttackArg extracts the damage type and dice expression from an attack grant arg.
// Supports "<type>:<dice>" (e.g. "fire:2d6+3") or plain "<dice>" (defaults to "untyped").
func parseAttackArg(arg string) (dmgType, diceExpr string) {
	if i := strings.IndexByte(arg, ':'); i >= 0 {
		return arg[:i], arg[i+1:]
	}
	return assets.DamageTypeUntyped, arg
}

// rollOneAttack performs a single attack roll for the given attack grant arg.
// Does NOT apply damage — PerformAttack handles that.
func rollOneAttack(attacker, target Combatant, attackArg string) AttackResult {
	dmgType, diceExpr := parseAttackArg(attackArg)

	roll := RollAttack(attacker.ModifierValue(assets.PerkKeyCombatAttackMod))
	if roll < target.ModifierValue(assets.PerkKeyCombatAC) {
		return AttackResult{Hit: false}
	}

	dice, err := ParseDice(diceExpr)
	if err != nil {
		dice = DiceRoll{Count: 1, Sides: 4}
	}

	raw := dice.Roll()
	damage, reflected := CalcDamage(raw, dmgType, attacker, target)
	return AttackResult{Damage: damage, Hit: true, Reflected: reflected}
}

// Tick processes one round of autoattacks for all active combatants.
func (m *Manager) Tick(_ context.Context) error {
	type roomEntry struct {
		zoneId string
		roomId string
		lines  []string
	}
	roomMessages := make(map[string]*roomEntry)

	addRoomLine := func(zoneId, roomId, line string) {
		key := zoneId + "|" + roomId
		if e, ok := roomMessages[key]; ok {
			e.lines = append(e.lines, line)
		} else {
			roomMessages[key] = &roomEntry{zoneId: zoneId, roomId: roomId, lines: []string{line}}
		}
	}

	m.mu.Lock()

	for _, state := range m.combatants {
		c := state.c
		if !c.IsAlive() || len(state.threat) == 0 {
			continue
		}

		wantsAutoAttack := len(c.GrantArgs(assets.PerkGrantAutoAttack)) > 0
		if !wantsAutoAttack && !state.attackPending {
			continue
		}
		state.attackPending = false

		target := m.resolveTarget(state)
		if target == nil {
			continue
		}

		results := PerformAttack(c, target)
		totalDamage := 0
		zoneId, roomId := c.Location()
		for _, r := range results {
			if r.Hit {
				totalDamage += r.Damage
				verb := DamageVerb(r.Damage)
				addRoomLine(zoneId, roomId, fmt.Sprintf("%s %s %s!", c.Name(), verb, target.Name()))
				if r.Reflected > 0 {
					reflectVerb := DamageVerb(r.Reflected)
					addRoomLine(zoneId, roomId, fmt.Sprintf("%s's ward %s %s back!", target.Name(), reflectVerb, c.Name()))
				}
			} else {
				addRoomLine(zoneId, roomId, fmt.Sprintf("%s misses %s.", c.Name(), target.Name()))
			}
		}
		if tState, ok := m.combatants[target.Id()]; ok {
			tState.threat[c.Id()] += totalDamage + c.ModifierValue(assets.PerkKeyCombatThreatMod)
		}
	}

	// Handle deaths.
	type deadEntry struct {
		c      Combatant
		threat map[string]int // snapshot of threat table at time of death
	}
	var dead []deadEntry
	for id, state := range m.combatants {
		if !state.c.IsAlive() {
			zoneId, roomId := state.c.Location()
			addRoomLine(zoneId, roomId, fmt.Sprintf("%s is DEAD!  R.I.P.", state.c.Name()))
			snap := make(map[string]int, len(state.threat))
			for k, v := range state.threat {
				snap[k] = v
			}
			dead = append(dead, deadEntry{c: state.c, threat: snap})
			for _, other := range m.combatants {
				delete(other.threat, id)
			}
			state.c.SetInCombat(false)
			state.c.SetCombatTargetId("")
			delete(m.combatants, id)
		}
	}

	// Remove combatants with empty threat tables.
	var cleanup []string
	for id, state := range m.combatants {
		if len(state.threat) == 0 {
			cleanup = append(cleanup, id)
		}
	}
	for _, id := range cleanup {
		m.combatants[id].c.SetInCombat(false)
		delete(m.combatants, id)
	}

	m.mu.Unlock()

	// Call OnDeath, remove mob, and place drops outside the lock (room operations acquire ri.mu).
	for _, d := range dead {
		drops := d.c.OnDeath()
		zoneId, roomId := d.c.Location()
		if zi := m.zones.GetZone(zoneId); zi != nil {
			if ri := zi.GetRoom(roomId); ri != nil {
				ri.RemoveMob(d.c.Id())
				for _, obj := range drops {
					ri.AddObj(obj)
				}
			}
		}
	}

	// Publish bundled room messages after releasing the lock.
	for _, entry := range roomMessages {
		zi := m.zones.GetZone(entry.zoneId)
		if zi == nil {
			continue
		}
		ri := zi.GetRoom(entry.roomId)
		if ri == nil {
			continue
		}
		if err := m.pub.Publish(ri, nil, []byte(strings.Join(entry.lines, "\n"))); err != nil {
			slog.Warn("failed to publish combat room messages", "error", err)
		}
	}

	// Distribute XP to player contributors after room messages so XP arrives in order.
	for _, d := range dead {
		if len(d.threat) == 0 {
			continue
		}
		zoneId, _ := d.c.Location()
		zi := m.zones.GetZone(zoneId)
		if zi == nil {
			continue
		}
		mobLevel := d.c.Level()
		baseXP := game.BaseExpForLevel(mobLevel)
		zi.ForEachPlayer(func(charId string, ci *game.CharacterInstance) {
			if _, ok := d.threat[ci.Id()]; !ok {
				return
			}
			xp := int(float64(baseXP) * game.LevelDiffMultiplier(ci.Character.Get().Level, mobLevel))
			canAdvance := ci.GainXP(xp)
			msg := fmt.Sprintf("You receive %d experience points.", xp)
			if canAdvance {
				msg += "\nYou feel ready to advance to the next level!"
			}
			if err := m.pub.Publish(game.SinglePlayer(charId), nil, []byte(msg)); err != nil {
				slog.Warn("failed to publish xp message", "error", err)
			}
		})
	}

	return nil
}

// resolveTarget picks the attack target for a combatant.
// Prefers the combatant's stored target ID; falls back to highest-threat alive enemy.
// Caller must hold m.mu.
func (m *Manager) resolveTarget(state *combatantState) Combatant {
	if tid := state.c.CombatTargetId(); tid != "" {
		if ts, ok := m.combatants[tid]; ok && ts.c.IsAlive() {
			return ts.c
		}
	}

	var best Combatant
	bestThreat := 0
	for enemyId, threat := range state.threat {
		if ts, ok := m.combatants[enemyId]; ok && ts.c.IsAlive() {
			if best == nil || threat > bestThreat {
				best = ts.c
				bestThreat = threat
			}
		}
	}
	return best
}

// register ensures a combatant is in the combatants map, returning its state.
// Caller must hold m.mu.
func (m *Manager) register(c Combatant) *combatantState {
	if state, ok := m.combatants[c.Id()]; ok {
		return state
	}
	state := &combatantState{
		c:      c,
		threat: make(map[string]int),
	}
	m.combatants[c.Id()] = state
	return state
}
