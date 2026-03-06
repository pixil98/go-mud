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

// DeathContext provides context to a Dyable when it dies.
type DeathContext struct {
	Victim Combatant
	ZoneID string
	RoomID string
	World  *game.WorldState
	Pub    game.Publisher
}

// Manager processes combat rounds. It is the only writer of combat state across
// all rooms; players and mobs are only mutated inside Tick while the manager
// lock is held.
type Manager struct {
	mu          sync.Mutex
	pub         game.Publisher
	world       *game.WorldState
	defaultZone string
	defaultRoom string
}

// NewManager creates a new combat Manager.
func NewManager(pub game.Publisher, world *game.WorldState, defaultZone, defaultRoom string) *Manager {
	return &Manager{
		pub:         pub,
		world:       world,
		defaultZone: defaultZone,
		defaultRoom: defaultRoom,
	}
}

// StartCombat places a player in combat with the mob at the given location.
// Seeds initial threat so the mob responds on its next tick. Returns an error
// if the player is already in combat or the mob cannot be found.
func (m *Manager) StartCombat(player *game.CharacterInstance, zoneId, roomId, mobInstanceId string) error {
	if player.GetCombatTargetId() != "" {
		return fmt.Errorf("already in combat")
	}
	zone := m.world.Instances()[zoneId]
	if zone == nil {
		return fmt.Errorf("zone not found")
	}
	room := zone.GetRoom(roomId)
	if room == nil {
		return fmt.Errorf("room not found")
	}
	mob := room.GetMob(mobInstanceId)
	if mob == nil {
		return fmt.Errorf("mob not found")
	}
	player.SetCombatTargetId(mob.InstanceId)
	mob.AddThreat(playerCombatId(player), 1)
	mob.SetInCombat(true)
	return nil
}

// roomResult holds the output of processing one room's combat round.
type roomResult struct {
	zoneID    string
	roomID    string
	roomMsgs  []string
	perPlayer map[string][]string // charId → per-player lines (XP, etc.)
	deaths    []DeathContext
}

// Tick processes one combat round in three phases: compute, deliver, effects.
func (m *Manager) Tick(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []roomResult
	for zoneID, zi := range m.world.Instances() {
		zi.ForEachRoom(func(roomID string, ri *game.RoomInstance) {
			r := m.processRoom(zoneID, roomID, ri)
			if r != nil {
				results = append(results, *r)
			}
		})
	}

	for _, r := range results {
		m.deliver(r)
	}

	for _, r := range results {
		for _, dctx := range r.deaths {
			if d, ok := dctx.Victim.(Dyable); ok {
				d.OnDeath(dctx)
			}
		}
	}

	return nil
}

// processRoom runs one combat round for a room and returns messages and deaths.
// Returns nil if there is no combat activity in the room.
func (m *Manager) processRoom(zoneID, roomID string, ri *game.RoomInstance) *roomResult {
	// Snapshot players and mobs to avoid holding room locks during attacks.
	roomPlayers := make(map[string]*game.CharacterInstance)
	var roomMobs []*game.MobileInstance

	ri.ForEachPlayer(func(charId string, ci *game.CharacterInstance) {
		roomPlayers[charId] = ci
	})
	ri.ForEachMob(func(mi *game.MobileInstance) {
		roomMobs = append(roomMobs, mi)
	})

	// Skip rooms with no combat activity.
	hasActivity := false
	for _, ci := range roomPlayers {
		if ci.GetCombatTargetId() != "" {
			hasActivity = true
			break
		}
	}
	if !hasActivity {
		for _, mi := range roomMobs {
			if mi.IsInCombat() {
				hasActivity = true
				break
			}
		}
	}
	if !hasActivity {
		return nil
	}

	r := &roomResult{
		zoneID:    zoneID,
		roomID:    roomID,
		perPlayer: make(map[string][]string),
	}

	// --- Player auto-attacks ---
	for _, ci := range roomPlayers {
		targetId := ci.GetCombatTargetId()
		if targetId == "" {
			continue
		}

		// Find target mob in this room's snapshot.
		var targetMob *game.MobileInstance
		for _, mi := range roomMobs {
			if mi.InstanceId == targetId {
				targetMob = mi
				break
			}
		}

		if targetMob == nil || !targetMob.IsAlive() {
			switchOrExitCombat(ci, roomMobs)
			continue
		}

		attacker := NewPlayerCombatant(ci.Character, ci, m.world, m.pub, m.defaultZone, m.defaultRoom)
		target := NewMobCombatant(targetMob, m.world, m.pub)
		msgs, dmg := executeAttacks(attacker, target)
		r.roomMsgs = append(r.roomMsgs, msgs...)

		if dmg > 0 {
			playerCID := playerCombatId(ci)
			scaled := scaledThreat(dmg, attacker.ThreatModifier())
			targetMob.RecordContribution(playerCID, scaled)
			targetMob.AddThreat(playerCID, scaled)
		}

		if !targetMob.IsAlive() {
			r.roomMsgs = append(r.roomMsgs,
				display.Colorize(display.Color.Red,
					fmt.Sprintf("%s is dead! R.I.P.", display.Capitalize(targetMob.Mobile.Get().ShortDesc))))

			for pid, lines := range computeMobKillXP(targetMob, roomPlayers) {
				r.perPlayer[pid] = append(r.perPlayer[pid], lines...)
			}

			targetMob.ClearAllThreats()

			r.deaths = append(r.deaths, DeathContext{
				Victim: target,
				ZoneID: zoneID,
				RoomID: roomID,
				World:  m.world,
				Pub:    m.pub,
			})

			// Retarget any player who was attacking this mob.
			for _, pci := range roomPlayers {
				if pci.GetCombatTargetId() == targetMob.InstanceId {
					switchOrExitCombat(pci, roomMobs)
				}
			}
		}
	}

	// --- Mob auto-attacks ---
	for _, mi := range roomMobs {
		if !mi.IsInCombat() || !mi.IsAlive() {
			continue
		}

		target := pickTarget(mi, roomPlayers)
		if target == nil {
			mi.ClearAllThreats()
			continue
		}

		attacker := NewMobCombatant(mi, m.world, m.pub)
		defender := NewPlayerCombatant(target.Character, target, m.world, m.pub, m.defaultZone, m.defaultRoom)
		msgs, _ := executeAttacks(attacker, defender)
		r.roomMsgs = append(r.roomMsgs, msgs...)

		if !target.IsAlive() {
			char := target.Character.Get()
			r.roomMsgs = append(r.roomMsgs,
				display.Colorize(display.Color.Red,
					fmt.Sprintf("%s is dead! R.I.P.", char.Name)))

			playerCID := playerCombatId(target)
			for _, mob := range roomMobs {
				mob.ClearThreatFor(playerCID)
			}
			target.ClearCombatTargetId()

			r.deaths = append(r.deaths, DeathContext{
				Victim: defender,
				ZoneID: zoneID,
				RoomID: roomID,
				World:  m.world,
				Pub:    m.pub,
			})
		}
	}

	return r
}

// computeMobKillXP calculates per-player XP for a mob kill.
// Returns a map of charId → lines to send to that player.
func computeMobKillXP(mob *game.MobileInstance, roomPlayers map[string]*game.CharacterInstance) map[string][]string {
	contributions := mob.SnapshotContributions()
	result := make(map[string][]string)

	total := 0
	for _, v := range contributions {
		total += v
	}
	if total == 0 {
		return result
	}

	def := mob.Mobile.Get()
	baseExp := def.ExpReward
	if baseExp <= 0 {
		baseExp = game.BaseExpForLevel(def.Level)
	}

	for cid, contrib := range contributions {
		charId := strings.TrimPrefix(cid, "player:")
		ci, ok := roomPlayers[charId]
		if !ok {
			continue
		}
		char := ci.Character.Get()
		mult := game.LevelDiffMultiplier(char.Level, def.Level)
		xp := int(float64(baseExp) * mult * float64(contrib) / float64(total))
		if xp < 1 && mult > 0 {
			xp = 1
		}
		if xp <= 0 {
			continue
		}
		char.Experience += xp
		msg := fmt.Sprintf("You receive %d experience points.", xp)
		if game.ExpToNextLevel(char.Level, char.Experience) <= 0 {
			msg += " You feel ready to advance!"
		}
		result[charId] = append(result[charId], msg)
	}

	return result
}

// deliver sends combined messages to each player in the room.
func (m *Manager) deliver(r roomResult) {
	if len(r.roomMsgs) == 0 && len(r.perPlayer) == 0 {
		return
	}
	ri := m.world.Instances()[r.zoneID].GetRoom(r.roomID)
	if ri == nil {
		return
	}

	roomText := strings.Join(r.roomMsgs, "\n")
	ri.ForEachPlayer(func(charId string, _ *game.CharacterInstance) {
		var parts []string
		if roomText != "" {
			parts = append(parts, roomText)
		}
		parts = append(parts, r.perPlayer[charId]...)
		if len(parts) > 0 {
			if err := m.pub.Publish(game.SinglePlayer(charId), nil, []byte(strings.Join(parts, "\n"))); err != nil {
				slog.Error("publishing combat message", "player", charId, "err", err)
			}
		}
	})
}

// executeAttacks runs all of attacker's attacks against target.
// Returns attack messages and total damage dealt.
func executeAttacks(attacker, target Combatant) ([]string, int) {
	var msgs []string
	totalDmg := 0
	targetAC := target.AC()

	for _, atk := range attacker.Attacks() {
		if !target.IsAlive() {
			break
		}

		attackRoll := RollAttack(atk.Mod)
		var damage int
		if attackRoll >= targetAC {
			damage = RollDamage(atk.DamageDice, atk.DamageSides, atk.DamageMod)
			target.AdjustHP(-damage)
			totalDmg += damage
		}

		verb := DamageVerb(damage)
		msgs = append(msgs, fmt.Sprintf("%s %s %s!", display.Capitalize(attacker.CombatName()), verb, target.CombatName()))
	}
	return msgs, totalDmg
}

// pickTarget returns the living player in players with the highest threat on
// the mob, or nil if there is no valid target.
func pickTarget(mob *game.MobileInstance, players map[string]*game.CharacterInstance) *game.CharacterInstance {
	threats := mob.SnapshotThreats()
	bestThreat := -1
	var bestPlayer *game.CharacterInstance

	for cid, threat := range threats {
		charId := strings.TrimPrefix(cid, "player:")
		ci, ok := players[charId]
		if !ok || !ci.IsAlive() {
			continue
		}
		if threat > bestThreat {
			bestThreat = threat
			bestPlayer = ci
		}
	}
	return bestPlayer
}

// switchOrExitCombat switches the player to the first alive mob that has them
// in its threat table, or clears the player's combat state if none is found.
func switchOrExitCombat(player *game.CharacterInstance, roomMobs []*game.MobileInstance) {
	playerCID := playerCombatId(player)
	for _, mi := range roomMobs {
		if mi.IsAlive() && mi.HasThreat(playerCID) {
			player.SetCombatTargetId(mi.InstanceId)
			return
		}
	}
	player.ClearCombatTargetId()
}

// scaledThreat applies the core.combat.threat_mod perk modifier to a raw amount.
// Returns at least 1 if amount > 0.
func scaledThreat(amount, modifier int) int {
	if amount <= 0 {
		return 0
	}
	t := amount * (100 + modifier) / 100
	if t < 1 {
		t = 1
	}
	return t
}

// playerCombatId returns the opaque threat-table key for a player character.
func playerCombatId(ci *game.CharacterInstance) string {
	return fmt.Sprintf("player:%s", ci.Character.Id())
}
