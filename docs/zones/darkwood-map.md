# Darkwood Zone Map

---

## Overview

```
  [OVR]─[WCP]─[HNT]
            │    │
[EDG]─[CLR]─[TRL]─[DTL]─[CRK]─[AGV]─[STC]─[BHL]─── Barrow
          │    │  ↖  │ ↖    │     │        │    │
       [OLD] [HDS] [NTL][STR] [FLN] [WOLF] │    └── east to barrow
            ↙   │         │
         [HNO] [HOL]─[BOG]─[BDP]─[BIS]
                 /   │
             [HCV] [GLN]─[WBP]─[DEN]
                     │          │
                   [GLS]─[SLK] [EGG]   [DDP]↓

              ↖ = NW diagonal (deep-trail→north-trail)
              ↖ = NW diagonal (trail←hollow-north via west on descent)
```

---

## Entry Area

```
    [OVR]──[WCP]──[HNT]
    (dead)   │       │
             │       │
[EDG]───[CLR]──────[TRL]──── east ────▶
             │       │
          [OLD]   [HDS]──[HNO]
                    │       │
                  [HOL]◀────┘
```

Rooms:
- `EDG`  darkwood-edge (← millbrook-east-gate)
- `CLR`  darkwood-clearing
- `OLD`  darkwood-old-camp (dead end)
- `WCP`  darkwood-woodcutter-camp
- `OVR`  darkwood-overgrown-road (dead end)
- `HNT`  darkwood-hunters-path
- `TRL`  darkwood-trail
- `HDS`  darkwood-hollow-descent
- `HNO`  darkwood-hollow-north (western rim bypass)

**hollow-north** is a western detour off `hollow-descent`. From `hollow-descent`
go W to reach the rim, then S to drop into the hollow — bypassing the main
descent path.

---

## Hollow & Bog

```
         ◀── N from trail
               │
            [HDS]──[HNO]
               │       │
[HCV]───────[HOL]   (S)│
(dead, bear)   │       │
               └───────┘
               │
[HCV]──[HOL]──[BOG]──[BDP]──[BIS]
               │             (dead)
            (stream
            from HCV)
```

Simplified:

```
[HDS]──[HNO]
  │       │ S
[HOL]◀───┘
  │ W         │ E
[HCV]       [BOG]──[BDP]──[BIS]
(dead)                    (dead)
```

Rooms:
- `HOL`  darkwood-hollow
- `HCV`  darkwood-hollow-cave (dead end, bear den + stream)
- `BOG`  darkwood-bog (accessible only from hollow going E)
- `BDP`  darkwood-bog-depths
- `BIS`  darkwood-bog-island (dead end)

Bog is accessible from the hollow going east; it is not connected to the deep
trail. The hollow drains eastward into the bog, which drains further east into
the bog depths.

---

## Deep Forest

```
              [RDG] (dead end)
                │ N
              [DTL]──[CRK]──[AGV]──[STC]──[BHL]──▶ Barrow
             ↗ NW      │N     │S     │E      │E
          [NTL]       [STR] [FLN] (from    (from
            │N        (dead) (dead) AGV)    STC)
          [WLF]
          (dead)
            │E
          [DFL]──[CHR]──▶ Ruins (S)
```

Rooms:
- `DTL`  darkwood-deep-trail
- `RDG`  darkwood-ridge (dead end, N of DTL)
- `CRK`  darkwood-creek
- `STR`  darkwood-stream-source (dead end, N of CRK)
- `AGV`  darkwood-ancient-grove
- `FLN`  darkwood-fallen-giant (dead end, S of AGV)
- `STC`  darkwood-stone-circle (E of AGV)
- `NTL`  darkwood-north-trail (NW of DTL — one remaining diagonal)
- `WLF`  darkwood-wolf-territory (dead end, N of NTL)
- `DFL`  darkwood-deadfall (E of NTL)
- `CHR`  darkwood-charcoal-mound

The `deep-trail`→`north-trail` NW diagonal is the only remaining diagonal in
the zone. It is intentionally distinct from the N→`ridge` cardinal exit.

---

## Barrow

```
[AGV]──[STC]──[BHL]──[BEN]──[BHA]──[BTM]
  N→    E→    W←  E→         │N     (dead)
(grove) (circle) (circle)  [BSD]
                            (dead, barrow-blade)
```

Rooms:
- `BHL`  darkwood-barrow-hill (W→stone-circle, E→barrow-entrance)
- `BEN`  darkwood-barrow-entrance
- `BHA`  darkwood-barrow-hall
- `BTM`  darkwood-barrow-tomb (boss: barrow-lord)
- `BSD`  darkwood-barrow-side (barrow-blade loot)

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
    (dead)          │ ↓ down
                  [RUC]
                  (boss: stone-golem)
```

Rooms:
- `RPA`  darkwood-ruins-path
- `RAR`  darkwood-ruins-arch
- `RSQ`  darkwood-ruins-square
- `RCP`  darkwood-ruins-chapel (dead end, ward-amulet loot)
- `RHL`  darkwood-ruins-hall
- `RUC`  darkwood-ruins-undercroft (boss: stone-golem)

No diagonal exits.

---

## Spider Area

```
    ◀── S from creek
          │
       [GLN]───[WBP]───[DEN]
          │               │ N     │ ↓ down
       [GLS]──[SLK]    [EGG]    [DDP]
       (dead) (dead)   (dead)   (boss: spider-queen, venom-ring)
```

Rooms:
- `GLN`  darkwood-glen
- `WBP`  darkwood-web-path
- `DEN`  darkwood-den
- `GLS`  darkwood-glen-south
- `SLK`  darkwood-silk-grove (dead end)
- `EGG`  darkwood-egg-chamber (dead end)
- `DDP`  darkwood-den-depths (boss: spider-queen, venom-ring loot)

No diagonal exits.
