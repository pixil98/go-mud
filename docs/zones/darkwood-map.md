# Darkwood Zone Map

Diagonal exits are marked with `↗↘↙↖`. Known issues are marked `⚠`.

---

## Overview

```
                     ┌─────────────────────────────────────────────────────────┐
                     │                     NORTH BRANCH                        │
              [WOLF] │                    [RUINS CHAIN]                        │
                │    │                                                          │
  [OVR]─[WCP]─[HNT] ↑                                                         │
            │    │   [NTL]─[DFL]─[CHR]─── Ruins ───────────────────────────── ┘
[EDG]─[CLR]─[TRL]─[DTL]─[CRK]─[AGV]─[STC]─[BHL]─── Barrow
          │    │ ↙   │ ↖    │     │          ↗   ↗
       [OLD] ─── [HDS] [NTL][STR][FLN]              ← Diagonals explained below
             [HNO]   │
               ↘   [HOL]─[BOG]─[BDP]─[BIS]
                    /   │
                [HCV] [GLN]─[WBP]─[DEN]
                        │          │
                      [GLS]─[SLK] [EGG]   [DDP]↓
```

---

## Entry Area

```
           N
           │
    [OVR]──[WCP]──[HNT]
    (dead)   │       │
             │       │
[EDG]───[CLR]│     [TRL]──── east to deep forest ────▶
(←millbrook) │     / │
             │  ↙/   │
          [OLD] /   [HDS]
               /      │
           [HNO]       │
              ↘        │
            [HOL] ◀────┘
```

Rooms:
- `EDG`  darkwood-edge
- `CLR`  darkwood-clearing
- `OLD`  darkwood-old-camp
- `WCP`  darkwood-woodcutter-camp
- `OVR`  darkwood-overgrown-road (dead end, W of WCP)
- `HNT`  darkwood-hunters-path
- `TRL`  darkwood-trail
- `HDS`  darkwood-hollow-descent
- `HNO`  darkwood-hollow-north ⚠ (see notes)

⚠ **hollow-north issue**: `trail` exits SW→`hollow-north`, and `hollow-north`
exits SE→`hollow`. Geometrically, SW of `trail` lands where `old-camp` already
sits (old-camp is S of clearing, and trail is E of clearing). `hollow-north`
is a shortcut rim around the north side of the hollow, but it creates a
confusing diagonal loop alongside the `trail`→`hollow-descent`→`hollow`
cardinal path.

---

## Hollow & Bog

```
              ◀── from trail (N)
                    │
                 [HDS]
                    │
[HCV]───[HOL]───[BOG]───[BDP]───[BIS]
(dead)         (N of BOG → DTL ⚠)

```

Rooms:
- `HOL`  darkwood-hollow
- `HCV`  darkwood-hollow-cave (dead end, bear den, stream)
- `BOG`  darkwood-bog
- `BDP`  darkwood-bog-depths
- `BIS`  darkwood-bog-island (dead end)

⚠ **bog-to-deep-trail**: `bog` exits N→`deep-trail`. In the grid this means
`bog` and `deep-trail` are directly N-S neighbours, but `hollow` is two steps
south of `trail` (via `hollow-descent`→`hollow`), and `bog` is E of `hollow`.
This puts `bog` two rows south of `trail` while `deep-trail` is in the same
row as `trail`. The N exit from `bog` is longer than one step suggests.

---

## Deep Forest

```
                  [RDG] (dead end)
                    │ N
    [WLF]           │
    (dead)        [DTL]───[CRK]───[AGV]───[STC]
       │N          │S       │N      │S       │NE ↗
    [NTL]          │     [STR]   [FLN]       ↗
       ↘ SE   [BOG]      (dead)  (dead) [BHL]──▶ Barrow
       ↘    (← W from HOL)
    [DTL]──[DFL]──[CHR]──▶ Ruins
```

Rooms:
- `DTL`  darkwood-deep-trail
- `RDG`  darkwood-ridge (dead end, N of DTL)
- `CRK`  darkwood-creek
- `STR`  darkwood-stream-source (dead end, N of CRK)
- `AGV`  darkwood-ancient-grove
- `FLN`  darkwood-fallen-giant (dead end, S of AGV)
- `STC`  darkwood-stone-circle
- `NTL`  darkwood-north-trail
- `WLF`  darkwood-wolf-territory (dead end)
- `DFL`  darkwood-deadfall
- `CHR`  darkwood-charcoal-mound

⚠ **NW diagonal (deep-trail→north-trail)**: `deep-trail` exits NW→`north-trail`.
The reverse is SE. `deep-trail` also exits N→`ridge`, so NW is intentionally a
different branch. This diagonal is probably fine but could be replaced with a
cardinal W or N connection if preferred.

⚠ **NE chain (ancient-grove→stone-circle→barrow-hill)**: Two back-to-back NE
diagonals. `ancient-grove` NE→`stone-circle` NE→`barrow-hill`. Could be
straightened to cardinal (e.g., N then E, or just E).

---

## Barrow

```
              ◀── NE from stone-circle
                    │
               [BHL]───[BEN]───[BHA]───[BTM]
               ↙SW              │N     (dead)
           [AGV] ⚠           [BSD]
                              (dead)
```

Rooms:
- `BHL`  darkwood-barrow-hill
- `BEN`  darkwood-barrow-entrance
- `BHA`  darkwood-barrow-hall
- `BTM`  darkwood-barrow-tomb (boss: barrow-lord)
- `BSD`  darkwood-barrow-side (barrow-blade loot)

⚠ **barrow-hill SW bug**: `barrow-hill` exits SW→`ancient-grove`, bypassing
`stone-circle`. It should exit SW→`stone-circle` to be consistent with the
path `ancient-grove`→`stone-circle`→`barrow-hill`.

---

## Ruins

```
     ◀── S from charcoal-mound
              │
          [RPA]
              │
          [RAR]
              │
    [RCP]──[RSQ]──[RHL]
    (dead)         │ ↓ down
                 [RUC]
                 (dead)
```

Rooms:
- `RPA`  darkwood-ruins-path
- `RAR`  darkwood-ruins-arch
- `RSQ`  darkwood-ruins-square
- `RCP`  darkwood-ruins-chapel (dead end, ward-amulet loot)
- `RHL`  darkwood-ruins-hall
- `RUC`  darkwood-ruins-undercroft (boss: stone-golem)

No diagonal exits in this sub-area.

---

## Spider Area

```
    ◀── S from creek
          │
       [GLN]───[WBP]───[DEN]
          │               │ N          │ ↓ down
       [GLS]──[SLK]   [EGG]         [DDP]
       (dead) (dead)   (dead)        (boss: spider-queen)
```

Rooms:
- `GLN`  darkwood-glen
- `WBP`  darkwood-web-path
- `DEN`  darkwood-den
- `GLS`  darkwood-glen-south
- `SLK`  darkwood-silk-grove (dead end)
- `EGG`  darkwood-egg-chamber (dead end)
- `DDP`  darkwood-den-depths (boss: spider-queen, venom-ring loot)

No diagonal exits in this sub-area.

---

## Issue Summary

| # | Room             | Exit       | Destination     | Issue                                     |
|---|------------------|------------|-----------------|-------------------------------------------|
| 1 | barrow-hill      | SW         | ancient-grove   | ⚠ Should be SW→stone-circle               |
| 2 | trail            | SW         | hollow-north    | ⚠ Geometrically overlaps old-camp position |
| 3 | hollow           | NW         | hollow-north    | ⚠ Paired with issue #2                    |
| 4 | deep-trail       | NW         | north-trail     | Diagonal; could be W or N                 |
| 5 | ancient-grove    | NE         | stone-circle    | First of two chained NE diagonals         |
| 6 | stone-circle     | NE         | barrow-hill     | Second of two chained NE diagonals        |
| 7 | bog              | N          | deep-trail      | ⚠ Geometrically 2 rows away, not 1        |
