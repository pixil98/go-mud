# CircleMUD Compatibility Gaps

Gaps identified while reviewing CircleMUD asset specs against our current asset types.
The goal is not a perfect 1:1 mapping — just enough to support a sample world with similar play experience.

---

## Rooms

### Room flags
These map to grant perks on `Room.Perks`. No schema change needed — just new well-known grant keys and runtime behavior for each.

| CircleMUD Flag | Description |
|----------------|-------------|
| DARK | Room is dark; characters need a light source or infravision to see |
| DEATH | Death trap; character dies on entry (no XP loss) |
| NOMOB | Mobs cannot enter |
| INDOORS | Room is indoors |
| PEACEFUL | Violence not allowed |
| SOUNDPROOF | Shouts and broadcasts not heard in room |
| NOTRACK | Track ability cannot path through |
| NOMAGIC | All magic fails in room |
| TUNNEL | Only one person allowed at a time |
| PRIVATE | Cannot teleport/goto if two or more people present |
| GODROOM | Staff-only room |

### Sector type (water, forest, city, etc.)
Without movement points, sector type is mostly cosmetic. The one case that matters: `water_noswim` — traversal requires a solution (boat item, waterwalk ability). We'd model this as a perk on the room (e.g. `room.water`) that the player or their equipment must satisfy.

**Decision needed:** Do we add a `Sector string` field, or just model water/special terrain as room perks entirely?

### Extra descriptions
Rooms (and objects) can have keyword→text pairs that players access via `look at <keyword>`. Useful for flavor and puzzles.

**Decision needed:** How do extra descs fit into the targeting system? Simple fallback when `look <keyword>` finds no entity, or something else?

### Exit description
`look north` can show a description without the player actually entering the room. Currently `Exit` has no description field — straightforward to add.

### Pickproof doors
A lock that can only be opened with a key, not picked. Add a `Pickproof bool` to `Closure` (or `Lock`).

### Spawn max-existing limits
A world-wide cap on live instances of a given mob/object type at the time of zone reset. `MobSpawn`/`ObjSpawn` have no such field. Useful for rare/unique mobs.

---

## Mobiles

### Behavior flags (action bitvector)
These map to grant perks on `Mobile.Perks`. No schema change needed.

| CircleMUD Flag | Description |
|----------------|-------------|
| SENTINEL | Doesn't wander |
| SCAVENGER | Picks up valuables from the ground |
| AWARE | Cannot be backstabbed |
| AGGRESSIVE | Attacks all visible players |
| STAY_ZONE | Won't wander outside its zone |
| WIMPY | Flees at ~20% HP |
| AGGR_EVIL | Attacks evil-aligned players |
| AGGR_GOOD | Attacks good-aligned players |
| AGGR_NEUTRAL | Attacks neutral-aligned players |
| MEMORY | Remembers and retaliates against attackers |
| HELPER | Assists other mobs being attacked in same room |
| NOCHARM | Immune to charm |
| NOSUMMON | Immune to summoning |
| NOSLEEP | Immune to sleep spells |
| NOBASH | Cannot be bashed |
| NOBLIND | Immune to blind |

### Affection flags
Also map to grant perks on `Mobile.Perks`.

| CircleMUD Flag | Description | Notes |
|----------------|-------------|-------|
| INVISIBLE | Starts invisible | |
| DETECT_INVIS | Can see invisible characters/objects | |
| SENSE_LIFE | Senses hidden life | |
| WATERWALK | Can traverse no-swim water | |
| SANCTUARY | Takes half damage | Implemented as `core.defense.all.absorb.pct 50` |
| INFRAVISION | Sees in dark rooms | |
| SNEAK | Movement doesn't announce to room | |
| HIDE | Hidden; only visible with sense life | |
| PROTECT_EVIL | Damage reduction vs evil attackers | |
| PROTECT_GOOD | Damage reduction vs good attackers | |
| NOTRACK | Cannot be tracked | |

### Bare hand damage
Covered by the `attack` grant perk with a dice expression arg (e.g. `"2d6"`). No gap.

### HP as dice expression
CircleMUD lets builders set mob HP as `xdy+z`. We skip this — level-based HP derivation is sufficient.

### Base stats on mobs
CircleMUD E-spec allows setting STR, DEX, CON, INT, WIS, CHA directly on a mob. Currently `BaseStats` lives only on `assets.Character`. Adding `BaseStats map[StatKey]int` to `assets.Mobile` would allow builders to set explicit base stats on mobs, with unset stats falling back to level-based defaults.

### Gold drops
Mobs carry gold and drop it on death. No currency system exists yet.

**Decision needed:** How is gold represented? Options:
- A `Gold int` field on `Mobile` (and `ActorInstance`), materialized as a lootable money object on death
- A money `ObjectSpawn` in the mob's inventory (uses existing inventory system)

### Alignment
CircleMUD has numeric alignment (-1000 to 1000: evil/neutral/good), used by AGGR_EVIL/GOOD/NEUTRAL flags and item restrictions. Deferred for now — can add a simple `Alignment string` (`"good"`, `"neutral"`, `"evil"`) to `Mobile` and `Character` when needed.

### Load/default position
Sleeping, sitting, resting, standing at spawn and after combat. Skipped — not a priority.

### Attack types (combat message flavor)
CircleMUD mobs have a bare-hand attack type (hit, bite, slash, claw, etc.) controlling combat message wording. Skipped for now — see Possible Future Work.

---

## Objects

### Extra descriptions
Objects (and rooms) can have keyword→text pairs accessible via `look at <keyword>`. A note object is just an object with one extra desc entry. Same `ExtraDescs []ExtraDesc` solution as rooms — covers both features.

### Cosmetic aura
CircleMUD has GLOW, HUM, and BLESS flags as cosmetic markers. Rather than individual flags, add a single `Aura string` field to `Object` — a short description appended when examining the object (e.g. "It glows faintly.", "It hums softly.").

### Object flags
Extend the existing `Flags []string` on `Object` with new values. These are properties of the object itself, not effects granted to the wearer.

| Flag | Description |
|------|-------------|
| `invisible` | Object starts invisible |
| `no_drop` | Cursed — cannot be dropped |
| `no_sell` | Shopkeepers will not buy or sell |

The `magic` flag (prevents enchanting) is deferred until a spell/enchantment system exists.

### Light sources
Objects can be light sources with a finite duration (hours remaining, or -1 for eternal). A carried or equipped light source should negate `room.dark`. This interacts with the darkness/light system.

**Decision needed:** How does a light source interact with `room.dark`? Does it grant something to the room, to the actor, or is it checked directly by the darkness system?

### Boat
Enables traversal of no-swim water rooms. Probably an object flag (`boat`) that the movement/room system checks when entering a `room.water` room.

### Spell delivery items (scroll, wand, staff, potion)
Require a spell system and charge tracking on object instances. Deferred.

### Food and drink containers
Require a hunger/thirst system. Deferred.

### Item cost
Gold coin value used by shopkeepers. Deferred until shops/currency are implemented.

### Alignment-restricted items (ANTI_GOOD/EVIL/NEUTRAL)
Items that can only be used by certain alignments. Deferred until alignment system is added.

### Affect fields (stat mods when equipped)
Mostly covered by modifier perks (STR, DEX, AC, hitroll, damroll, etc.). Gaps:
- `core.resource.hp.max` modifier — probably just needs a well-known perk key, no schema change
- MANA modifier — deferred with mana system
- MOVE modifier — deferred with movement system
- Saving throw modifiers — deferred with saving throw system

---

## Zones

CircleMUD zones use imperative reset commands (M/O/G/E/P/D/R) to describe spawns and state. Our declarative approach covers almost all of it:

| CircleMUD Command | Our equivalent |
|-------------------|----------------|
| M — load mob to room | `Room.MobSpawns` |
| O — load object to room | `Room.ObjSpawns` |
| G — give object to mob inventory | `Mobile.Inventory` |
| E — equip mob with object | `Mobile.Equipment` |
| P — put object inside object | `ObjectSpawn.Contents` |
| D — set door state on reset | `Exit.Closure` declared state |
| R — remove object from room | Skip; decaying objects (`Lifetime > 0`) handle this naturally |

The only gap is **spawn max-existing limits**, already noted under Rooms.

---

## Shops

Shops are a new asset type with no current equivalent. A shop ties a shopkeeper mob to a set of rooms where it operates, a catalog of always-stocked items, and rules for buying/selling.

Fully deferred until the currency system exists.

**Key design question:** Are catalog items separate from the shopkeeper's inventory, or does "always stocked" mean the shop restocks the mob's inventory on zone reset?

Other open questions when the time comes:
- Buy types: simple list of object type names, or complex boolean keyword expressions like CircleMUD?
- Open hours: time-gated shops require a game time system
- With-who restrictions: depend on alignment and class systems

---

## Future Systems

Systems that multiple gaps depend on. Worth designing before implementing the gaps that need them.

### Currency
Needed by: mob gold drops, item cost, shops.
Gold should probably be an int on `ActorInstance` (not an inventory object), materialized as a lootable money object on mob death.

### Alignment
Needed by: AGGR_EVIL/GOOD/NEUTRAL mob flags, ANTI_alignment item flags, shop restrictions.
Simple `Alignment string` (`"good"` / `"neutral"` / `"evil"`) on `Mobile` and `Character`.

### Light and darkness
Needed by: `room.dark`, INFRAVISION, light source objects.
A room with `room.dark` is unlit. Light sources (carried/equipped objects) and infravision negate darkness. Affects visibility and targeting in dark rooms.

### Spell system
Needed by: scroll, wand, staff, potion object types; spell-based mob abilities.

### Hunger and thirst
Needed by: food and drink container object types.

### Movement points and water traversal
Needed by: sector types, boat flag, WATERWALK grant, `room.water`.
Even without full movement points, water traversal (requiring boat or waterwalk) is a useful standalone feature.

### Saving throws
Needed by: saving throw modifier affect fields on objects, various spell effects.

---

## Possible Future Work

- **Attack type flavor** — `AttackType string` on `Mobile`/weapon `Object` for varied combat messages (bites, slashes, claws, etc.)
