# Skill Trees

This document defines how skill/spell progression trees are authored in JSON assets.

## Overview

The game uses **trees** to represent progression. Players spend:

- **Minor Points (SP)**: earned each level
- **Major Points (BP)**: earned every 5 levels (5, 10, 15, …)

Trees are designed to encourage specialization without hard classes.

A tree contains three node lists:

- **Spine**: ordered “tier gates” purchased with BP (vertical chain)
- **Nodes**: non-spine nodes purchased with SP (actives and passives)
- **Capstones**: mutually exclusive endgame packages purchased with 2 BP

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
- Spine I: requires **any 1–2** Tier 0 nodes in that tree
- Spine II: requires Spine I + **any 2** Tier I nodes
- Spine III: requires Spine II + **any 2** Tier II nodes
- Spine IV: requires Spine III + **any 2** Tier III nodes

Exact node lists and counts are tree-author decisions.

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
- Each capstone requires the **final spine node**.
- A character may own **at most one capstone per tree**.
- Capstones cost **2 BP**.

Capstones should be **packages**, typically:
- 1 `unlock_ability` (signature active)
- + 2–5 passive perks (`key_mod` / `tag`) that strongly reinforce a specific line identity

## Perks

Perks are granted by nodes and races. Keep perk types limited; prefer `key_mod` over inventing new perk handlers.

### Supported perk types

#### `unlock_ability`
Grants an ability by ID (spells and skills share the same namespace).
```json
{ "type": "unlock_ability", "id": "firebolt" }
```

#### `stat_mod`
Modifies a core, engine-known stat (e.g., `str`, `dex`, `int`, `wis`, `cha`).
```json
{ "type": "stat_mod", "stat": "int", "value": 1 }
```

#### `key_mod`
Modifies an asset-defined numeric key. The engine stores and sums these keys but does not interpret them by default; other assets (abilities, items) can read them.
```json
{ "type": "key_mod", "key": "evocation.storm.chain_jumps_add", "value": 1 }
```

#### `tag`
Grants a keyword flag (for rules or UI).
```json
{ "type": "tag", "tag": "darkvision" }
```

### Key naming convention (recommended)
Use namespaced dotted keys:

- `<tree>.<line>.<property>_<add|pct>`

Examples:
- `evocation.fire.damage_pct`
- `evocation.fire.burn_pct`
- `evocation.frost.slow_pct`
- `evocation.storm.disrupt_add`
- `evocation.force.evasion_ignore_pct`

## Design guidance

- Spines should provide infrastructure and identity (reliability, small specialization keys, slots), not large generic power.
- Tier 0 nodes should be flavorful and low impact.
- Element/line specialization should ramp naturally down the tree.
- Capstones should be the largest jump and should strongly reinforce a line.
