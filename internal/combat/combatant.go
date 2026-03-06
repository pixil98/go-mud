package combat

import (
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// Combatant is anything that can participate in combat.
type Combatant interface {
	CombatID() string
	CombatName() string
	IsAlive() bool
	AC() int
	Attacks() []Attack
	AdjustHP(int)
	SetInCombat(bool)
	Level() int
	ThreatModifier() int
}

// Dyable is implemented by Combatants that have death side-effects.
type Dyable interface {
	OnDeath(ctx DeathContext)
}

// combatActor is the subset of game.CharacterInstance / game.MobileInstance
// methods used by the combat system. Both types satisfy this via ActorInstance
// embedding.
type combatActor interface {
	Resource(name string) (current, max int)
	AdjustResource(name string, delta int)
	SetInCombat(v bool)
	ModifierValue(key string) int
	GrantArgs(key string) []string
}

// baseCombatant implements the Combatant methods that are identical for
// players and mobs. Concrete types embed this and provide identity, AC,
// Attacks, and Level.
type baseCombatant struct {
	actor combatActor
}

func (b *baseCombatant) IsAlive() bool {
	cur, _ := b.actor.Resource(assets.ResourceHp)
	return cur > 0
}

func (b *baseCombatant) AdjustHP(delta int) {
	b.actor.AdjustResource(assets.ResourceHp, delta)
}

func (b *baseCombatant) SetInCombat(v bool) {
	b.actor.SetInCombat(v)
}

func (b *baseCombatant) ThreatModifier() int {
	return b.actor.ModifierValue(assets.PerkKeyCombatThreatMod)
}

// buildAttacks constructs the attack list from grant perks, reading attackMod
// and dmgMod from perk keys.
func (b *baseCombatant) buildAttacks() []Attack {
	attackMod := b.actor.ModifierValue(assets.PerkKeyCombatAttackMod)
	dmgMod := b.actor.ModifierValue(assets.PerkKeyCombatDmgMod)

	var attacks []Attack
	for _, expr := range b.actor.GrantArgs(assets.PerkGrantAttack) {
		dice, sides, bonus, ok := parseAttackDice(expr)
		if !ok {
			continue
		}
		attacks = append(attacks, Attack{
			Mod:         attackMod,
			DamageDice:  dice,
			DamageSides: sides,
			DamageMod:   dmgMod + bonus,
		})
	}
	return attacks
}

// parseAttackDice parses a dice expression like "2d6" or "2d6+3" into
// (dice, sides, bonus, ok). Returns ok=false for any unrecognized format.
func parseAttackDice(expr string) (dice, sides, bonus int, ok bool) {
	// Try NdN+N first, then NdN.
	n, err := fmt.Sscanf(expr, "%dd%d+%d", &dice, &sides, &bonus)
	if err == nil && n == 3 && dice >= 1 && sides >= 1 {
		return dice, sides, bonus, true
	}
	n, err = fmt.Sscanf(expr, "%dd%d", &dice, &sides)
	if err == nil && n == 2 && dice >= 1 && sides >= 1 {
		return dice, sides, 0, true
	}
	return 0, 0, 0, false
}

// PlayerCombatant adapts a CharacterInstance for the combat system.
type PlayerCombatant struct {
	baseCombatant
	Character   storage.SmartIdentifier[*assets.Character]
	Player      *game.CharacterInstance
	world       *game.WorldState
	pub         game.Publisher
	DefaultZone string
	DefaultRoom string
}

// NewPlayerCombatant creates a PlayerCombatant.
func NewPlayerCombatant(
	char storage.SmartIdentifier[*assets.Character],
	player *game.CharacterInstance,
	world *game.WorldState,
	pub game.Publisher,
	defaultZone, defaultRoom string,
) *PlayerCombatant {
	return &PlayerCombatant{
		baseCombatant: baseCombatant{actor: player},
		Character:     char,
		Player:        player,
		world:         world,
		pub:           pub,
		DefaultZone:   defaultZone,
		DefaultRoom:   defaultRoom,
	}
}

func (c *PlayerCombatant) CombatID() string   { return fmt.Sprintf("player:%s", c.Character.Id()) }
func (c *PlayerCombatant) CombatName() string { return c.Character.Get().Name }
func (c *PlayerCombatant) Level() int         { return c.Character.Get().Level }

func (c *PlayerCombatant) AC() int {
	stats := c.Player.EffectiveStats()
	return stats[assets.StatDEX].Mod() + c.actor.ModifierValue(assets.PerkKeyCombatAC)
}

func (c *PlayerCombatant) Attacks() []Attack {
	attacks := c.buildAttacks()
	// Unarmed fallback: players always have at least a 1d4 punch.
	if len(attacks) == 0 {
		attacks = append(attacks, Attack{
			Mod:         c.actor.ModifierValue(assets.PerkKeyCombatAttackMod),
			DamageDice:  1,
			DamageSides: 4,
			DamageMod:   c.actor.ModifierValue(assets.PerkKeyCombatDmgMod),
		})
	}
	// Add stat-based bonuses.
	char := c.Character.Get()
	stats := c.Player.EffectiveStats()
	strMod := stats[assets.StatSTR].Mod()
	for i := range attacks {
		attacks[i].Mod += strMod + char.Level/2
		attacks[i].DamageMod += strMod
	}
	return attacks
}

// AwardXP adds amount to the player's experience and returns the message to
// send them. Returns empty string if the award is zero.
func (c *PlayerCombatant) AwardXP(amount int) string {
	if amount <= 0 {
		return ""
	}
	char := c.Character.Get()
	char.Experience += amount
	msg := fmt.Sprintf("You receive %d experience points.", amount)
	if game.ExpToNextLevel(char.Level, char.Experience) <= 0 {
		msg += " You feel ready to advance!"
	}
	return msg
}

// OnDeath handles player death: clears followers, restores HP, moves to respawn.
func (c *PlayerCombatant) OnDeath(ctx DeathContext) {
	deadId := string(c.Character.Id())
	char := c.Character.Get()

	// Clear following on both sides.
	c.Player.SetFollowingId("")
	fromRoom := ctx.World.Instances()[ctx.ZoneID].GetRoom(ctx.RoomID)
	if fromRoom != nil {
		fromRoom.ForEachPlayer(func(charId string, ps *game.CharacterInstance) {
			if ps.GetFollowingId() == deadId {
				ps.SetFollowingId("")
				if err := ctx.Pub.Publish(game.SinglePlayer(charId), nil,
					[]byte(fmt.Sprintf("You stop following %s.", char.Name))); err != nil {
					slog.Warn("failed to notify follower of death", "error", err)
				}
			}
		})
	}

	if err := ctx.Pub.Publish(game.SinglePlayer(c.Character.Id()), nil,
		[]byte("You have been slain! You awaken in a familiar place...")); err != nil {
		slog.Warn("failed to notify slain player", "error", err)
	}

	// Restore HP to full.
	_, maxHP := c.Player.Resource(assets.ResourceHp)
	c.Player.SetResource(assets.ResourceHp, maxHP)

	toRoom := ctx.World.Instances()[c.DefaultZone].GetRoom(c.DefaultRoom)
	if fromRoom != nil && toRoom != nil {
		c.Player.Move(fromRoom, toRoom)
		roomDesc := toRoom.Describe(char.Name)
		if err := ctx.Pub.Publish(game.SinglePlayer(c.Character.Id()), nil, []byte(roomDesc)); err != nil {
			slog.Warn("failed to send room description after death", "error", err)
		}
	}
}

// MobCombatant adapts a MobileInstance for the combat system.
type MobCombatant struct {
	baseCombatant
	Instance *game.MobileInstance
	world    *game.WorldState
	pub      game.Publisher
}

// NewMobCombatant creates a MobCombatant.
func NewMobCombatant(instance *game.MobileInstance, world *game.WorldState, pub game.Publisher) *MobCombatant {
	return &MobCombatant{
		baseCombatant: baseCombatant{actor: instance},
		Instance:      instance,
		world:         world,
		pub:           pub,
	}
}

func (c *MobCombatant) CombatID() string   { return fmt.Sprintf("mob:%s", c.Instance.InstanceId) }
func (c *MobCombatant) CombatName() string { return c.Instance.Mobile.Get().ShortDesc }
func (c *MobCombatant) Level() int         { return c.Instance.Mobile.Get().Level }

func (c *MobCombatant) AC() int {
	return c.actor.ModifierValue(assets.PerkKeyCombatAC)
}

func (c *MobCombatant) Attacks() []Attack {
	return c.buildAttacks()
}

// ExpReward returns the XP value of killing this mob.
func (c *MobCombatant) ExpReward() int {
	def := c.Instance.Mobile.Get()
	if def.ExpReward > 0 {
		return def.ExpReward
	}
	return game.BaseExpForLevel(def.Level)
}

// OnDeath handles mob death: creates a corpse, transfers inventory/equipment, removes mob.
func (c *MobCombatant) OnDeath(ctx DeathContext) {
	mi := c.Instance
	def := mi.Mobile.Get()

	corpseAliases := append([]string{"corpse"}, def.Aliases...)
	corpseDef := &assets.Object{
		Aliases:      corpseAliases,
		ShortDesc:    fmt.Sprintf("the corpse of %s", def.ShortDesc),
		LongDesc:     fmt.Sprintf("The corpse of %s lies here.", def.ShortDesc),
		DetailedDesc: fmt.Sprintf("The lifeless remains of %s lie here, growing cold.", def.ShortDesc),
		Flags:        []string{"container", "immobile"},
	}

	corpseId := fmt.Sprintf("corpse-%s", mi.InstanceId)
	corpseSmartId := storage.NewResolvedSmartIdentifier[*assets.Object](corpseId, corpseDef)

	corpse := &game.ObjectInstance{
		InstanceId: uuid.New().String(),
		Object:     corpseSmartId,
		Contents:   game.NewInventory(),
	}

	for _, obj := range mi.GetInventory().Drain() {
		corpse.Contents.AddObj(obj)
	}
	for _, obj := range mi.GetEquipment().Drain() {
		corpse.Contents.AddObj(obj)
	}

	ri := ctx.World.Instances()[ctx.ZoneID].GetRoom(ctx.RoomID)
	if ri != nil {
		ri.AddObj(corpse)
		ri.RemoveMob(mi.InstanceId)
	}
}
