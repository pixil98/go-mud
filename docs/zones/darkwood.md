# Zone: The Darkwood

**ID**: `darkwood` | **Reset**: empty, 30m | **Rooms**: 43 | **Level range**: 2–10

## Goals

The Darkwood is the first place players can get hurt. The tone should shift
noticeably the moment they leave Millbrook — quieter, denser, less safe. It's
a full exploration zone with distinct sub-areas, multiple bosses, and enough
size that players can get genuinely lost.

The zone rewards thoroughness: players who push deeper find harder enemies and
better loot. Players who only skim the edges can still make progress.

## Design Principles

- Difficulty escalates naturally from west (entry) to east and inward.
- Three distinct boss encounters, each with a different feel:
  the barrow-lord (undead, classic dungeon), the stone-golem (ancient ruins),
  and the spider-queen (monster lair, fast multi-attack).
- A coherent geography: the stream that starts at stream-source feeds the creek,
  flows underground, and surfaces in the bear's cave before draining into the bog.
- No shortcuts to bosses — players earn the deeper areas.

## Areas

| Area          | Rooms | Level | Notes                                              |
|---------------|-------|-------|----------------------------------------------------|
| Entry         | 7     | 2–4   | edge, clearing, old-camp, woodcutter-camp, trail, hunters-path, overgrown-road |
| Hollow        | 5     | 4–6   | descent, hollow, hollow-north, hollow-cave (bear den + stream) |
| Bog           | 3     | 4–5   | bog, bog-depths, bog-island                        |
| Deep forest   | 6     | 5–7   | deep-trail, ridge, creek, ancient-grove, fallen-giant, stream-source |
| North branch  | 4     | 6–7   | north-trail, wolf-territory, deadfall, charcoal-mound |
| Stone circle  | 1     | 7     | Transition between deep forest and barrow          |
| Barrow        | 5     | 7–9   | barrow-hill, entrance, hall, side (barrow-blade), tomb (barrow-lord) |
| Ruins         | 6     | 8–9   | path, arch, square, chapel (ward-amulet), hall, undercroft (stone-golem) |
| Spider        | 7     | 8–10  | glen, glen-south, web-path, den, egg-chamber, silk-grove, den-depths (spider-queen) |

## Mobs

| Mob           | Level | HP  | AC | Notes                                    |
|---------------|-------|-----|----|------------------------------------------|
| deer          | 1     | 8   | 8  | Passive, no autoattack                   |
| wolf          | 3     | 25  | 10 | Pack animal, appears in groups           |
| bandit        | 3     | 22  | 10 | Entry area; wields a rusted knife        |
| boar          | 4     | 35  | 11 | Hollow area                              |
| giant-frog    | 4     | 32  | 10 | Bog                                      |
| dire-wolf     | 6     | 60  | 13 | Deep forest                              |
| cave-bear     | 6     | 65  | 13 | Elite; hollow-cave bear den              |
| skeleton      | 7     | 78  | 14 | Barrow (animated dead)                   |
| ruin-shade    | 8     | 95  | 15 | Ruins area                               |
| spider        | 8     | 95  | 15 | Spider area and egg-chamber              |
| barrow-lord   | 9     | 140 | 16 | Boss; barrow-tomb; 1d8 + 1d4            |
| stone-golem   | 9     | 145 | 16 | Boss; ruins-undercroft; 1d8 + 1d4       |
| spider-queen  | 10    | 155 | 16 | Boss; den-depths; 4× 1d3 (fast flurry)  |

## Loot

All loot is currently placed in the world — mobs do not drop items.

| Item                    | Location                        | Effect               |
|-------------------------|---------------------------------|----------------------|
| darkwood-woodcutter-axe | woodcutter-camp (ground)        | 1d8, wield           |
| darkwood-barrow-blade   | barrow-side (bone-pile)         | 1d8 + damage_mod +2  |
| darkwood-ward-amulet    | ruins-chapel (offering bowl)    | +2 AC, neck          |
| darkwood-venom-ring     | den-depths (cocoon)             | +2 AC, finger        |

## TODO

- **Mob loot drops**: Bosses should drop items on death. The barrow-lord and
  spider-queen in particular feel like they should drop something notable. Once
  the drop system exists, consider adding a key to each boss that opens a locked
  chest in their area.
- **Bandit key**: The rusted knife is on the bandit, but eventually bandits could
  carry a key to a hidden stash box somewhere in the entry area.
- **Locked chest in barrow-side**: The bone-pile currently spawns the barrow-blade
  openly. Once boss drops exist, this could become a locked chest opened by a key
  dropped by the barrow-lord.
- **Exit to Forest Depth**: The east edge of the Darkwood has no exit yet. A future
  zone to the east would connect here.
