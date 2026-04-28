# Skill Trees

This document defines how skill/spell progression trees are authored in JSON assets.

## Overview

The game uses **trees** to represent progression. Players spend:

- **Minor Points (SP)**: 1 per level (40 total at max level)
- **Major Points (BP)**: 1 every 5 levels — 5, 10, 15, 20, 25, 30, 35, 40 (8 total)

Max level is **40**.

Trees are designed to encourage specialization without hard classes. A player can
reach a capstone in one tree (6 BP) with 2 BP left to dip into a second tree's
spine, but cannot capstone two trees (would need 12 BP). SP pressure comes from
trees being wide (~60 SP of nodes each) — even going all-in on a single tree
leaves a significant number of nodes unpurchased, and splitting across two trees
forces hard choices.

A tree contains three node lists:

- **Spine**: ordered “tier gates” purchased with BP (vertical chain)
- **Nodes**: non-spine nodes purchased with SP (actives and passives)
- **Capstones**: mutually exclusive endgame packages purchased with 2 BP

### Point budget summary

| Resource | Total at 40 | One capstone path | Two capstone paths |
|----------|-------------|-------------------|--------------------|
| BP       | 8           | 6 (4 spine + 2 cap) | 12 — impossible |
| SP       | 40          | ~60 available per tree | ~120 across two trees |

## Point Costs and Purchasing Rules

### Costs by node category
- **Spine nodes**: cost **1 BP** each (by convention; enforced by purchase logic)
- **Non-spine nodes**: cost **1 SP** per purchase (ranked nodes cost 1 SP per rank)
- **Capstones**: cost **2 BP** (per capstone)

> Note: point cost is determined by which list the node appears in (Spine/Nodes/Capstones), not by a field on the node.

### Rank rules
- Nodes that grant an **active ability** must have **MaxRank = 1** (or omit MaxRank).
- Nodes that grant only passive perks may have **MaxRank > 1**.
- Multi-rank nodes apply perks **once per rank purchased** (additive per-rank).

## Tree Structure

### Spine (tier gates)
- Spine nodes must be purchased **in order**.
- Spine nodes should always grant an immediate perk so spending BP never feels empty.
- Spine nodes unlock deeper portions of the tree via prerequisites.

**Spine count:** Most trees use **4 spine nodes** (I–IV), but this is a convention, not a hard requirement. Trees may have fewer or more if justified.

### Tier 4 “endgame” nodes
Tier 4 nodes require the final spine node. They provide powerful endgame passives
and actives that compete with each other for SP. Capstones require several Tier 4
nodes, so reaching a capstone demands meaningful SP investment at the top of the tree.

### Tier 0 “taster” nodes
Trees may include a small number of prereq-less nodes in `Nodes[]` to provide early flavor.

Tier 0 nodes:
- may have **no prereqs**
- may be **passive** (recommended) or **cantrip-tier active** (allowed)
- should be intentionally low impact and not define a build on their own

## Preventing “bank points until level 5” behavior

To encourage early investment and prevent players from skipping the early tree identity:

- **Spine I should usually require owning some Tier 0 nodes** (expressed in JSON prereqs).
- Higher spine nodes should require **N-of** earlier nodes so deeper tiers cannot be rushed immediately.

Recommended gating pattern (guideline, not a hard rule):
- Spine I: requires **any 2** Tier 0 nodes in that tree
- Spine II: requires Spine I + **any 3** Tier I nodes
- Spine III: requires Spine II + **any 3** Tier II nodes
- Spine IV: requires Spine III + **any 3** Tier III nodes
- Capstones: require Spine IV + **any 3** Tier IV nodes

The higher gate counts (3 instead of 2) reflect wider trees (~60 SP of nodes).
Exact node lists and counts are tree-author decisions.

### Spell chains
Actives within an element line should form **chains** across tiers:

- **Horizontal chains** (within a tier): a second spell requires the first spell in
  the same tier. E.g. Flame Lash requires Firebolt; both are Tier 1.
- **Vertical chains** (cross-tier): a higher-tier spell requires both the spine gate
  **and** a lower-tier spell. E.g. Fireball requires Spine II + Flame Lash.

Chains make element investment feel deliberate — you build mastery, not just unlock
tiers. Unchained "utility" actives (e.g. Force Dart) can exist as cheap picks that
don't lead anywhere deeper.

## Prerequisites

### Canonical prereq model
Prereqs are expressed as a recursive boolean structure:

- `type`: `"and"` (default), `"or"`, `"not"`
- `n`: optional minimum number of satisfied **terms** (k-of-n)
- `terms`: list of conditions
  - a term is either `{ "node": "<NODE_ID>" }` or `{ "group": <Prereq> }`

#### Semantics
Let `k` = number of satisfied terms, `m` = total terms.

- If `type` omitted, it defaults to `"and"`.
- If `n` is omitted/0:
  - `"and"` requires `k == m` (all terms)
  - `"or"` requires `k >= 1`
- If `n` > 0 for `"and"` or `"or"`:
  - the prereq is satisfied if `k >= n`
- `"not"` means **none** of the terms may be satisfied: `k == 0`
  - `n` must be omitted/0 for `"not"`

#### Empty prereqs
- If a node has no prerequisites, omit `prereqs` entirely.
- An explicit empty prereq object is invalid.

### Examples

#### Simple AND (default)
Requires both nodes:
```json
"prereqs": {
  "terms": [
    { "node": "EVO1" },
    { "node": "FIREBOLT" }
  ]
}
```

#### OR group
Requires either node:
```json
"prereqs": {
  "type": "or",
  "terms": [
    { "node": "ICE_SHARD" },
    { "node": "ARC_SPARK" }
  ]
}
```

#### NOT group (mutual exclusion helper)
Requires that neither node is owned:
```json
"prereqs": {
  "type": "not",
  "terms": [
    { "node": "CAPSTONE_A" },
    { "node": "CAPSTONE_B" }
  ]
}
```

> Capstone mutual exclusion is enforced by system rules (one capstone per tree). Use NOT prereqs only for non-capstone exclusivity if needed.

#### Tier gate: “any 2 of these”
Requires Spine I and any 2 of four Tier I nodes:
```json
"prereqs": {
  "terms": [
    { "node": "EVO1" },
    {
      "group": {
        "n": 2,
        "terms": [
          { "node": "FIREBOLT" },
          { "node": "ICE_SHARD" },
          { "node": "ARC_SPARK" },
          { "node": "FORCE_DART" }
        ]
      }
    }
  ]
}
```

#### Spine I gate using Tier 0 nodes
Requires any 2 Tier 0 “attunement” nodes:
```json
"prereqs": {
  "n": 2,
  "terms": [
    { "node": "T0-EMBER_ATTUNEMENT" },
    { "node": "T0-RIME_ATTUNEMENT" },
    { "node": "T0-STATIC_ATTUNEMENT" },
    { "node": "T0-KINETIC_ATTUNEMENT" }
  ]
}
```

## Capstones

Capstones are intended to be a meaningful specialization “jump”.

Rules:
- Each capstone requires the **final spine node** and several **Tier IV nodes** (see gating pattern).
- A character may own **at most one capstone per tree**.
- Capstones cost **2 BP**.

Capstones should be **packages**, typically:
- 1 `grant` with `key: "unlock_ability"` (signature active)
- + 2–5 passive perks (`modifier`) that strongly reinforce a specific line identity

## Perks

Perks are granted by nodes and races. There are two perk types: `modifier` and `grant`.

### Supported perk types

#### `modifier`
Modifies a numeric key. The engine stores and sums these keys per character. Well-known keys use the `core.*` namespace and are interpreted by the engine. Asset-defined keys use tree-scoped names and are read by abilities, items, or other assets.
```json
{ "type": "modifier", "key": "core.stats.int", "value": 1 }
{ "type": "modifier", "key": "core.damage.fire.pct", "value": 5 }
```

#### `grant`
Grants a keyword or parameterized effect. The `key` identifies the grant type; `arg` provides an optional parameter.

Well-known grant keys:
- `unlock_ability` — grants access to an ability. Arg is the ability id.
- `attack` — grants an extra attack. Arg is a dice expression (e.g. `"2d6"`).

```json
{ "type": "grant", "key": "unlock_ability", "arg": "firebolt" }
{ "type": "grant", "key": "attack", "arg": "1d6" }
{ "type": "grant", "key": "ignore_room_flag", "arg": "dark" }
```

### Key naming convention (recommended)
Use namespaced dotted keys:

- `core.stats.<stat>` — engine-known ability scores (`str`, `dex`, `con`, `int`, `wis`, `cha`)
- `core.resource.<pool>.<aspect>` — resource pool modifiers (`max`, `per_level`, `regen`)
- `core.combat.<property>` — combat modifiers (`ac`, `attack_mod`, `damage_mod`)
- `core.damage.<type>.pct` — global damage type scaling (applies to all abilities with that damage type)
- `core.damage.<type>.crit_pct` — global crit chance by damage type
- `<tree>.<property>` — tree-scoped keys for mechanics specific to one tree

Examples:
- `core.stats.str`
- `core.resource.mana.max`
- `core.resource.mana.regen`
- `core.combat.attack_mod`
- `core.damage.fire.pct`
- `core.damage.frost.pct`
- `core.damage.storm.pct`
- `core.damage.fire.crit_pct`
- `evocation.cast_time_reduce`

## Effect Handlers

Abilities reference an effect handler by name in their `handler` field. The handler
determines what gameplay effect the ability produces. Available handlers:

| Handler       | Scope   | Description |
|---------------|---------|-------------|
| `damage`      | target  | Deals damage to a player or mob target |
| `actor_buff`  | target/self | Applies timed perks to a target player/mob, or self if no target |
| `room_buff`   | room    | Applies timed perks to the caster's current room |
| `zone_buff`   | zone    | Applies timed perks to the caster's current zone |
| `world_buff`  | world   | Applies timed perks to the entire world |

### Buff config fields

All buff handlers (`actor_buff`, `room_buff`, `zone_buff`, `world_buff`) share
the same config fields:

- `"perks"` ([]Perk, required): perks to apply.
- `"duration"` (number, required): number of ticks the buff lasts.
- `"name"` (string, optional): entry name for the timed perk. Defaults to the ability name. Same-name buffs replace rather than stack.

### Spell progression pattern

Buff handlers enable a natural spell progression within a tree where the same
effect scales in scope across tiers:

- **Tier 1**: `actor_buff` — single-target buff (e.g. grant fire resistance to one ally)
- **Tier 2**: `room_buff` — area buff affecting everyone in the room
- **Tier 3/4**: `zone_buff` — zone-wide buff affecting an entire dungeon floor

This creates meaningful choices: the higher-tier version covers more allies but
costs more resources and requires deeper tree investment.

## Design guidance

- **Target ~60 SP of nodes per tree (Tiers 0–4).** Even a player going all-in on one tree should not be able to buy every node. The capstone is a specialization choice that costs breadth within the tree, not a freebie on top of everything.
- Spines should provide infrastructure and identity (reliability, small specialization keys, slots), not large generic power.
- Tier 0 nodes should be flavorful and low impact.
- Each tier should include both shared utility nodes (attractive to any build) and line-specific passives (rewarding commitment to a particular line). The competition between them creates SP pressure.
- Element/line specialization should ramp naturally down the tree.
- Capstones should be the largest jump and should strongly reinforce a line.
