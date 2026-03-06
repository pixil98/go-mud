# Combat System Design

## Context

Replacing the old combat system with a new one built around the `Combatant` interface. The system should support PvE, PvP, and summons/pets without special-casing any of them.

## Core Model: Threat-Based Combat (No Fight Abstraction)

There is no "fight" struct. The combat manager tracks **per-combatant threat tables**. Your enemies are anyone you have mutual threat with. Combat ends for a combatant when all their enemies are dead or gone.

### Why No Fights

- AoE naturally works: hitting 3 mobs adds threat to all 3, no fight merging
- PvP just works: attacking a player creates mutual threat
- Summons just work: they have their own threat tables like any other combatant
- Two separate encounters in a room can exist independently or merge organically through AoE
- Simpler data model

### Threat Tables

Each combatant in combat has a threat table tracking how much threat each of their enemies has generated:

```
Mob A's threat table: {PlayerX: 50, PlayerY: 30}
Mob B's threat table: {PlayerX: 10}
PlayerX's enemies: {Mob A, Mob B}  (derived: anyone who has PlayerX in their threat table, or vice versa)
```

Threat is directional. When PlayerX deals 20 damage to Mob A:
- Mob A's threat for PlayerX increases by `damage + PlayerX.ModifierValue("core.combat.threat_mod")`
- PlayerX is now "in combat with" Mob A (mutual threat relationship)

### Entering Combat

`StartCombat(attacker, target)`:
1. Mark both as in combat
2. Add initial threat entry: target's table gets `{attacker: 1}`, attacker's table gets `{target: 1}`
3. Register both in the manager's combatant index

If either is already in combat, they just gain a new enemy — no merging needed.

### Exiting Combat

A combatant leaves combat when they have no living enemies:
- All enemies dead → `SetInCombat(false)`, clear from manager
- Flee (future) → remove from all enemies' threat tables, `SetInCombat(false)`

## Action Points

Each combatant gets **action points (AP)** per tick, determined entirely by perks (`core.combat.action_points`). No perk = 0 AP = can't act in combat at all (unless the combatant has abilities that cost 0 AP).

- Abilities have an `ap_cost` field (replaces the old `cast_time`)
- Autoattack is a perk grant (`PerkGrantAutoAttack`). If a combatant has it AND has remaining AP after abilities, they auto-attack once (costs 1 AP)
- A fighter with 3 AP could: use a 2 AP ability + autoattack (1 AP)
- A mage with 2 AP could: cast a 2 AP spell (no autoattack, no perk)
- Haste buffs grant extra AP via the perk system
- Dual-wielding could grant a second autoattack consuming 1 more AP

### Auto-Attack

`PerkGrantAutoAttack = "autoattack"` — a grant perk (no arg). If present and AP ≥ 1, the combatant auto-attacks using their existing `PerkGrantAttack` dice grants. No arg on the autoattack perk itself — attack dice are already defined by attack grants from weapons/class/etc.

Costs 1 AP per auto-attack. Checked after manual abilities resolve.

### Autocast (Future — Not In This Design)

Autocast for casters is an interesting idea but raises balancing questions (resource costs, spell selection) that need more thought. For now, casters must actively use abilities each tick. This can be revisited once the base combat system is working.

### Tick Order for a Combatant

1. Compute AP for this tick: `ModifierValue("core.combat.action_points")`
2. Deduct AP for any ability used this tick (command system calls `SpendAP`)
3. If autoattack perk and AP remaining ≥ 1: auto-attack, deduct 1 AP
4. Remaining AP is lost (no banking)

## Target Selection

All combatants use threat to pick a new target when their current one dies — highest threat from their threat table, falling back to first living enemy.

The `Combatant` interface includes `CombatTargetId() string` and `SetCombatTargetId(string)`. This avoids type assertions and keeps mocks simple.

- **Players**: `CombatTargetId()` returns their chosen target. On death of current target, retarget to highest-threat living enemy and call `SetCombatTargetId()`.
- **Non-players** (mobs, summons): `CombatTargetId()` returns `""`. Always pick the enemy with the highest threat each tick.

The combat manager logic: if `CombatTargetId()` is non-empty and that target is alive, use it; otherwise pick highest threat from the threat table.

## Tick Loop

```
Lock mu

For each combatant in combat:
    if !alive or has no living enemies: skip

    ap = combatant.ModifierValue("core.combat.action_points")
    ap -= state.apSpent  // deduct AP used by abilities this tick

    target = resolveTarget(combatant)
    if target == nil: skip

    // Autoattack: if perk granted and enough AP
    if ap > 0 and hasAutoAttack(combatant):
        resolve melee attack (roll vs AC, damage, absorb)
        apply damage, add threat, send messages
        ap -= 1

    if !target.IsAlive():
        handle death

Reset apSpent for all combatants
Clean up combatants with no living enemies → SetInCombat(false)
Unlock mu
```

## Attack Resolution

```
attackMod = attacker.ModifierValue("core.combat.attack_mod")
roll = RollAttack(attackMod)  // d20 + mod
targetAC = 10 + target.ModifierValue("core.combat.ac")

if roll < targetAC: miss

dice = ParseDice(attacker.GrantArgs("attack")[0])  // or default 1d4
damage = RollDamage(dice.Count, dice.Sides, dice.Mod)
damage += attacker.ModifierValue("core.combat.damage_mod")

absorb = target.ModifierValue("core.defense.all.absorb")
damage = max(0, damage - absorb)
```

## Threat API for Abilities

The `CombatManager` interface exposes threat and AP manipulation:

```go
type CombatManager interface {
    StartCombat(attacker, target combat.Combatant) error
    AddThreat(source, target combat.Combatant, amount int)
    SpendAP(combatantId string, cost int) // called when an ability is used
}
```

The `damageEffect` in `handler_ability.go` calls `AddThreat` after dealing damage.
A future `threatEffect` handler could add/reduce threat for taunt-style abilities.
Ability execution calls `SpendAP` so AP is deducted before autoattack/autocast resolve.

## Combatant Interface Changes

Add to `Combatant`:
```go
GrantArgs(key string) []string           // needed for attack dice and autoattack check
CombatTargetId() string                  // player: chosen target; mob: "" (use threat)
SetCombatTargetId(id string)             // player: update target; mob: no-op
```

Both types already have `GrantArgs` on `PerkCache` — just expose with locking. `CombatTargetId`/`SetCombatTargetId` are new: players store the value, mobs return `""`/no-op. No type assertions needed — the combat manager works entirely through the interface.

## Data Structures

### Manager (combat/manager.go)

```go
type Manager struct {
    mu         sync.Mutex
    combatants map[string]*combatantState  // combatant ID → state
    pub        game.Publisher
}

type combatantState struct {
    combatant  Combatant
    threat     map[string]int  // enemy ID → threat amount
    apSpent    int             // AP spent by abilities this tick
}
```

### Dice Parser (combat/dice.go)

```go
type DiceRoll struct {
    Count int  // N in NdS
    Sides int  // S in NdS
    Mod   int  // +/- modifier
}

func ParseDice(expr string) (DiceRoll, error)
// Parses "2d6", "1d8+3", "2d4-1"
```

## Death Handling

When a combatant dies:
1. Call `OnDeath()` (stub — will handle respawn/loot/XP later)
2. Remove from all enemies' threat tables
3. `SetInCombat(false)`, `SetCombatTargetId("")`
4. Enemies whose threat table is now empty also leave combat
