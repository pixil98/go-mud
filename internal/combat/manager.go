package combat

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
)

// Manager runs the combat loop and tracks threat relationships between combatants.
type Manager struct {
	mu         sync.Mutex
	combatants map[string]*combatantState
	pub        game.Publisher
	abilities  AbilityHandler
}

type combatantState struct {
	c        shared.Actor
	threat   map[string]int   // enemy ID → accumulated threat
	cooldown map[string][]int // auto_use arg → per-duplicate cooldown counters
}

// NewManager creates a combat Manager.
func NewManager(pub game.Publisher) *Manager {
	return &Manager{
		combatants: make(map[string]*combatantState),
		pub:        pub,
	}
}

// SetAbilityHandler sets the handler used to execute auto_use abilities during
// the combat tick. Called after handler creation to break the init cycle.
func (m *Manager) SetAbilityHandler(h AbilityHandler) {
	m.abilities = h
}

// StartCombat registers both combatants and initialises mutual threat.
// It is idempotent: re-entering after flee preserves existing threat entries.
func (m *Manager) StartCombat(attacker, target shared.Actor) error {
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


// AddThreat increases the threat that source has generated toward target.
func (m *Manager) AddThreat(source, target shared.Actor, amount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.register(source)
	tState := m.register(target)
	tState.threat[source.Id()] += CalcThreat(amount, source)
}

// SetThreat sets the threat that source has on target's threat table to an
// absolute value, ignoring the threat modifier.
func (m *Manager) SetThreat(source, target shared.Actor, amount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.register(source)
	tState := m.register(target)
	tState.threat[source.Id()] = amount
}

// TopThreat sets source's threat on target to one more than the current
// highest entry, guaranteeing source becomes the top-threat enemy.
func (m *Manager) TopThreat(source, target shared.Actor) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.register(source)
	tState := m.register(target)

	maxThreat := 0
	for _, v := range tState.threat {
		if v > maxThreat {
			maxThreat = v
		}
	}
	tState.threat[source.Id()] = maxThreat + 1
}

// NotifyHeal adds heal-generated threat from healer to every combatant whose
// threat table contains target. Threat modifiers are applied via CalcThreat.
func (m *Manager) NotifyHeal(healer, target shared.Actor, amount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	modified := CalcThreat(amount, healer)
	if modified <= 0 {
		return
	}

	healerId := healer.Id()
	targetId := target.Id()
	for _, state := range m.combatants {
		if _, has := state.threat[targetId]; !has {
			continue
		}
		if state.c.Id() == healerId {
			continue
		}
		m.register(healer)
		healer.SetInCombat(true)
		state.threat[healerId] += modified
	}
}

// ParseAttackArg extracts the damage type and dice expression from an attack grant arg.
// Supports "<type>:<dice>" (e.g. "fire:2d6+3") or plain "<dice>" (defaults to "untyped").
func ParseAttackArg(arg string) (dmgType, diceExpr string) {
	if i := strings.IndexByte(arg, ':'); i >= 0 {
		return arg[:i], arg[i+1:]
	}
	return assets.DamageTypeUntyped, arg
}

// Tick processes one round of auto_use abilities for all active combatants.
func (m *Manager) Tick(_ context.Context) error {
	type roomEntry struct {
		room  *game.RoomInstance
		lines []string
	}
	roomMessages := make(map[*game.RoomInstance]*roomEntry)

	addRoomLine := func(room *game.RoomInstance, line string) {
		if e, ok := roomMessages[room]; ok {
			e.lines = append(e.lines, line)
		} else {
			roomMessages[room] = &roomEntry{room: room, lines: []string{line}}
		}
	}

	// Collect auto_use tasks under the lock, then execute them unlocked
	// to avoid deadlock (ability effects call StartCombat/AddThreat which also lock).
	type autoUseTask struct {
		abilityId string
		actor     shared.Actor
		target    shared.Actor
	}
	var autoUses []autoUseTask
	// roomPublish holds a per-attack room message with a per-target exclusion.
	type roomPublish struct {
		room    *game.RoomInstance
		msg     string
		exclude string // charId of the target player to exclude, or ""
	}
	var roomPublishes []roomPublish

	m.mu.Lock()

	for _, state := range m.combatants {
		c := state.c
		if !c.IsAlive() || len(state.threat) == 0 {
			continue
		}

		target := m.resolveTarget(state)
		if target == nil {
			continue
		}

		if m.abilities != nil {
			seen := make(map[string]int) // arg → how many times we've seen it so far
			for _, arg := range c.GrantArgs(assets.PerkGrantAutoUse) {
				abilityId := arg
				cooldownTicks := 1
				if i := strings.IndexByte(arg, ':'); i >= 0 {
					abilityId = arg[:i]
					if n, err := strconv.Atoi(arg[i+1:]); err == nil && n > 0 {
						cooldownTicks = n
					}
				}

				dupIdx := seen[arg]
				seen[arg]++

				// Grow the per-arg slice if this is a new duplicate.
				for len(state.cooldown[arg]) <= dupIdx {
					state.cooldown[arg] = append(state.cooldown[arg], 0)
				}

				remaining := state.cooldown[arg][dupIdx]
				if remaining > 0 {
					state.cooldown[arg][dupIdx] = remaining - 1
					continue
				}
				state.cooldown[arg][dupIdx] = cooldownTicks - 1
				autoUses = append(autoUses, autoUseTask{abilityId: abilityId, actor: c, target: target})
			}
		}
	}

	m.mu.Unlock()

	for _, task := range autoUses {
		result, err := m.abilities.ExecCombatAbility(task.abilityId, task.actor, task.target)
		if err != nil {
			continue
		}
		if result.RoomMsg != "" {
			roomPublishes = append(roomPublishes, roomPublish{
				room:    task.actor.Room(),
				msg:     result.RoomMsg,
				exclude: result.TargetId,
			})
		}
		if result.TargetMsg != "" {
			task.target.Notify(result.TargetMsg)
		}
	}

	m.mu.Lock()

	// Handle deaths.
	type deadEntry struct {
		c      shared.Actor
		threat map[string]int // snapshot of threat table at time of death
	}
	var dead []deadEntry
	for id, state := range m.combatants {
		if !state.c.IsAlive() {
			addRoomLine(state.c.Room(), fmt.Sprintf("%s is DEAD!  R.I.P.", state.c.Name()))
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

	// Publish per-attack room messages (3rd-person), excluding the target player
	// who already received the 2nd-person TargetMsg via Notify above.
	for _, rp := range roomPublishes {
		var exclude []string
		if rp.exclude != "" {
			exclude = []string{rp.exclude}
		}
		if err := m.pub.Publish(rp.room, exclude, []byte(rp.msg)); err != nil {
			slog.Warn("failed to publish room combat message", "error", err)
		}
	}

	// Call OnDeath, remove mob, and place drops outside the lock (room operations acquire ri.mu).
	for _, d := range dead {
		drops := d.c.OnDeath()
		ri := d.c.Room()
		ri.RemoveMob(d.c.Id())
		for _, obj := range drops {
			ri.AddObj(obj)
		}
	}

	// Publish bundled death room messages after OnDeath.
	for _, entry := range roomMessages {
		if err := m.pub.Publish(entry.room, nil, []byte(strings.Join(entry.lines, "\n"))); err != nil {
			slog.Warn("failed to publish combat room messages", "error", err)
		}
	}

	// Distribute XP to player contributors after room messages so XP arrives in order.
	for _, d := range dead {
		if len(d.threat) == 0 {
			continue
		}
		mobLevel := d.c.Level()
		baseXP := game.BaseExpForLevel(mobLevel)
		world := d.c.Room().Zone().World()
		for actorId := range d.threat {
			ci := world.GetPlayer(actorId)
			if ci == nil {
				continue
			}
			xp := int(float64(baseXP) * game.LevelDiffMultiplier(ci.Character.Get().Level, mobLevel))
			canAdvance := ci.GainXP(xp)
			msg := fmt.Sprintf("You receive %d experience points.", xp)
			if canAdvance {
				msg += "\nYou feel ready to advance to the next level!"
			}
			ci.Notify(msg)
		}
	}

	return nil
}

// resolveTarget picks the attack target for a combatant.
// Prefers the combatant's stored target ID; falls back to highest-threat alive enemy.
// Caller must hold m.mu.
func (m *Manager) resolveTarget(state *combatantState) shared.Actor {
	if tid := state.c.CombatTargetId(); tid != "" {
		if ts, ok := m.combatants[tid]; ok && ts.c.IsAlive() {
			return ts.c
		}
	}

	var best shared.Actor
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
func (m *Manager) register(c shared.Actor) *combatantState {
	if state, ok := m.combatants[c.Id()]; ok {
		return state
	}
	state := &combatantState{
		c:        c,
		threat:   make(map[string]int),
		cooldown: make(map[string][]int),
	}
	m.combatants[c.Id()] = state
	return state
}
