# Combat System Design

## Context

Replacing the old combat system with a new one built around the `Combatant` interface. The system should support PvE, PvP, and summons/pets without special-casing any of them.

## Core Model: Threat-Based Combat (No Fight Abstraction)

There is no "fight" struct. The combat manager tracks **per-combatant threat tables**. Your enemies are anyone with whom you have mutual threat. Combat ends for a combatant when all their enemies are dead or gone.

### Why No Fights

- AoE naturally works: hitting 3 mobs adds threat to all 3, no fight merging
- PvP just works: attacking a player creates mutual threat
- Summons just work: they have their own threat tables like any other combatant
- Two separate encounters in a room can exist independently or merge organically through AoE
- Simpler data model

### Threat Tables

Every combatant in combat has a threat table on both sides of the relationship. When PlayerX deals 20 damage to Mob A:
- Mob A's table gains `{PlayerX: 20 + PlayerX.ModifierValue("core.combat.threat_mod")}`
- PlayerX's table gains `{Mob A: 1}` (set by `StartCombat` when combat begins)

```
Mob A's threat table:   {PlayerX: 50, PlayerY: 30}
PlayerX's threat table: {Mob A: 1, Mob B: 1}
PlayerY's threat table: {Mob A: 1}
Mob B's threat table:   {PlayerX: 10}
```

Threat is directional and tracks "how much threat have my enemies generated toward me" — i.e., who should I attack? Every active combatant has a state in the manager, including players.

### Entering Combat

`StartCombat(attacker, target)` is idempotent — safe to call when already in combat:
1. Register both in the manager's combatant index if absent
2. If attacker's table doesn't yet have an entry for target: add `{target: 1}`
3. If target's table doesn't yet have an entry for attacker: add `{attacker: 1}`
4. `SetInCombat(true)` on both

Preserving existing entries (steps 2–3) handles re-entry after fleeing: if a player fled but the mob retained their threat, walking back in and calling `StartCombat` again picks up where the threat left off.

**Any damage initiates combat.** The `damageEffect` handler calls `StartCombat` before `AddThreat`, so a mage opening with a fireball automatically puts both parties into combat — no explicit `kill`/`attack` command required.

### Exiting Combat

A combatant leaves combat when they have no living enemies:
- All enemies dead → `SetInCombat(false)`, remove from manager
- Enemies whose threat table becomes empty after a death also leave combat

### Fleeing

`Flee(c)` removes a combatant from the active manager and clears their in-combat state, but **does not** remove their entry from enemies' threat tables. This means:

- If the group is still fighting, the mob retains the fleeing player's threat value.
- If the player re-enters the room and uses `attack`/`kill`, `StartCombat` re-registers them and the mob immediately recognises their prior threat level.
- If the fight ends while the player is gone, normal death/cleanup removes the threat tables entirely.

## Resource Costs and Action Points

These are two independent systems that are both checked before an ability executes, but at different layers:

- **Resource cost** (`mana`, `stamina`, etc.): deducted immediately in `executeAbility`. Applies everywhere.
- **Action Points (AP)**: also checked and spent in `executeAbility`, before the effect runs. AP lives on `CharacterInstance` — the combat manager has no involvement with AP at all.

### Action Points

AP is tracked as `currentAP int` on `CharacterInstance` and **reset to max at the start of each world tick**, unconditionally (unlike resource regen, which is gated on `!IsInCombat()`). Max is `max(1, ModifierValue("core.action_points.max"))`.

Abilities have an `ap_cost` field (replacing the old `cast_time`). Omitted or 0 is treated as 1.

`executeAbility` enforces AP before running any effect:
```
if char.CurrentAP() < ability.APCost → error "You're not ready to do that yet."
char.SpendAP(ability.APCost)
// then check resource cost, run effect, send messages
```

- A fighter with 3 AP could use a 2 AP ability and still have 1 AP for another
- A mage with 2 AP could cast a 2 AP spell and be done for the tick
- Haste buffs grant extra AP via the perk system
- Out of combat, AP still applies — one bash per tick by default

AP is the universal rate limiter for abilities. Cooldown (`cooldown` on abilities) is not enforced in this implementation.

### Auto-Attack

Auto-attack and AP are **independent systems**. The combat tick fires autoattack based purely on perk presence — it does not check or touch AP at all.

`PerkGrantAutoAttack = "autoattack"` — a grant perk in the melee tree (no arg). Every combat tick, any combatant with this perk attacks their target. Attack dice come from `GrantArgs("attack")` grants (weapons, class, etc.), defaulting to 1d4 if none.

This means a fighter/mage hybrid who invests in both trees gets both their spells (limited by AP) and their autoattack (always fires). This is intentional — they paid for both trees.

A mage who hasn't taken the autoattack perk will never punch automatically, regardless of AP remaining.

### Autocast (Future — Not In This Design)

Autocast for casters raises balancing questions (resource costs, spell selection) that need more thought. For now, casters must actively use abilities each tick.

## The attack/kill Command

`attack` (aliased as `kill`) is a built-in command available to every combatant — no perk or tree node required. It:

1. Enters combat with the target if not already in combat
2. Checks and spends 1 AP (same path as any ability in `executeAbility`)
3. Performs one melee attack using the attack resolution below

This gives mages the option to punch a mob if they want. They'll roll 1d4 with no attack bonuses unless they've invested in melee grants. Fighters who have the autoattack perk don't need to type `attack` every tick — it fires automatically.

## Target Selection

The `Combatant` interface includes `CombatTargetId() string` and `SetCombatTargetId(id string)`. No type assertions needed anywhere in the manager.

- **Players**: `CombatTargetId()` returns their chosen target. Updated by `attack`/`kill` command or when current target dies (retarget to highest-threat living enemy).
- **Non-players** (mobs, summons): `CombatTargetId()` returns `""`. Always pick the enemy with the highest threat each tick.

Manager resolution: if `CombatTargetId()` is non-empty and that target is alive, use it; otherwise pick the highest-threat living entry from the threat table.

## Tick Loop

```
Lock mu
Accumulate room messages (map of room key → []string)

For each combatant in combat:
    if !alive or threat table empty: skip
    if len(GrantArgs("autoattack")) == 0: skip

    target = resolveTarget(combatant)
    if target == nil: skip

    resolve melee attack (roll vs AC, damage, absorb)
    apply damage, add threat, append to room messages

Handle deaths:
    for each dead combatant:
        OnDeath() (stub)
        remove their ID from all enemies' threat tables
        SetInCombat(false), SetCombatTargetId("")
        remove from manager

Clean up combatants with empty threat tables → SetInCombat(false), remove from manager

Unlock mu
Publish bundled room messages (after unlock)
```

## Attack Resolution

Used by both autoattack and the manual `attack`/`kill` command:

```
attackMod = attacker.ModifierValue("core.combat.attack_mod")
roll = RollAttack(attackMod)  // d20 + mod
targetAC = 10 + target.ModifierValue("core.combat.ac")

if roll < targetAC: miss

diceExprs = attacker.GrantArgs("attack")
dice = ParseDice(diceExprs[0])  // first grant, or default "1d4" if none
damage = RollDamage(dice.Count, dice.Sides, dice.Mod)
damage += attacker.ModifierValue("core.combat.damage_mod")

absorb = target.ModifierValue("core.defense.all.absorb")
damage = max(1, damage - absorb)
```

## Threat API for Abilities

The `CombatManager` interface (in the `commands` package, where it is consumed):

```go
type CombatManager interface {
    StartCombat(attacker, target combat.Combatant) error
    AddThreat(source, target combat.Combatant, amount int)
    Flee(c combat.Combatant)
}
```

- `damageEffect` calls `StartCombat` then `AddThreat` after dealing damage — any damage initiates combat automatically.
- A future `threatEffect` handler could add/reduce threat for taunt/fade abilities.
- AP is handled entirely in `executeAbility` on `CharacterInstance`; the combat manager never touches it.

## Combatant Interface

Full interface including new additions:

```go
type Combatant interface {
    Id() string
    Name() string

    IsInCombat() bool
    SetInCombat(bool)
    IsAlive() bool

    Resource(name string) (current, max int)
    AdjustResource(name string, delta int)

    ModifierValue(key string) int
    GrantArgs(key string) []string   // expose PerkCache.GrantArgs with locking

    CombatTargetId() string          // player: stored target; mob: "" (always use threat)
    SetCombatTargetId(id string)     // player: store; mob: no-op

    OnDeath()
}
```

AP is not on this interface — it lives on `CharacterInstance` directly and is handled before the combat layer.

## Data Structures

### Manager (`internal/combat/manager.go`)

```go
type Manager struct {
    mu         sync.Mutex
    combatants map[string]*combatantState  // combatant ID → state
    pub        game.Publisher
}

type combatantState struct {
    c      Combatant
    threat map[string]int  // enemy ID → threat amount
}
```

### CharacterInstance additions (`internal/game/character.go`)

```go
currentAP int  // remaining AP this tick; reset to max by world tick

func (ci *CharacterInstance) CurrentAP() int
func (ci *CharacterInstance) SpendAP(cost int)
func (ci *CharacterInstance) ResetAP()  // sets currentAP = max(1, ModifierValue("core.action_points.max"))
```

### Dice Parser (`internal/combat/dice.go`)

```go
type DiceRoll struct {
    Count int  // N in NdS
    Sides int  // S in NdS
    Mod   int  // +/- modifier
}

func ParseDice(expr string) (DiceRoll, error)
// Parses "2d6", "1d8+3", "2d4-1", "d6" (Count defaults to 1)
```

## Death Handling

When a combatant dies (detected during tick):
1. Call `OnDeath()` (stub — respawn/loot/XP handled later)
2. Remove their ID from all other combatants' threat tables
3. `SetInCombat(false)`, `SetCombatTargetId("")`
4. Remove from combatants map
5. Any enemy whose threat table is now empty also exits combat

## Resource Regen

No passive regen during combat. The world tick already gates regen on `!IsInCombat()`. Recovery happens between fights. Healing abilities fill the in-combat recovery role.

## Perk Keys

| Constant | Key | Type |
|---|---|---|
| `PerkKeyActionPointsMax` | `"core.action_points.max"` | modifier |
| `PerkKeyCombatAttackMod` | `"core.combat.attack_mod"` | modifier |
| `PerkKeyCombatDmgMod` | `"core.combat.damage_mod"` | modifier |
| `PerkKeyCombatThreatMod` | `"core.combat.threat_mod"` | modifier |
| `PerkKeyCombatAC` | `"core.combat.ac"` | modifier |
| `PerkGrantAttack` | `"attack"` | grant (arg: dice expression e.g. `"2d6"`) |
| `PerkGrantAutoAttack` | `"autoattack"` | grant (arg: `""`, boolean presence check) |
