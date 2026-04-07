# CircleMUD Compatibility Gaps

Gaps identified while reviewing CircleMUD asset specs against our current asset types.
The goal is not a perfect 1:1 mapping — just enough to support a sample world with similar play experience.

## Import Status

1878 rooms across 29 zones imported into `circlemud/rooms/` and `circlemud/zones/`.
46 stub key objects in `circlemud/objects/`. Alternate config at `config.circlemud.json`.

**Still needed to make the import playable:**
- Import mobiles and wire them into room `mobile_spawns`
- Import objects (weapons, armor, containers, etc.) and wire into room `object_spawns` and mobile `inventory`/`equipment`
- Zone reset commands (M/O/G/E/P/D) define which mobs/objects load where — these need to be translated into our declarative `MobSpawns`, `ObjSpawns`, `Mobile.Inventory`, and `Mobile.Equipment`

---

## Rooms — DONE

### Room flags — done
Implemented as `Room.Flags []string` with typed enum, mirroring the `ObjectFlag` pattern.

| CircleMUD Flag | Our representation | Status |
|---|---|---|
| DARK | perk `dark` (propagates to occupants) | Done — visibility check in look/move handlers |
| DEATH | flag `death` | Done — data only, no respawn system yet |
| NOMOB | flag `nomob` | Done — data only, no mob wandering yet |
| PEACEFUL | perk `peaceful` | Already existed |
| NOMAGIC | perk `nomagic` | Done — data only, no spell system yet |
| TUNNEL | flag `single_occupant` | Done — enforced in move handler |
| INDOORS | — | Dropped (cosmetic, no system to drive) |
| SOUNDPROOF | — | Dropped (no shout system) |
| NOTRACK | — | Dropped (no tracking system) |
| PRIVATE | — | Dropped (no teleport/goto) |
| GODROOM | — | Dropped (no privilege system) |

### Light and darkness — done (basic)
- `dark` perk on room propagates to occupants
- `infravision` personal grant counters darkness for the holder
- `light` equipment grant — look/move handlers scan all actors in room; if anyone has it, darkness is negated for everyone
- See TODO file for remaining light work (finite duration, intrinsic-light objects, utility light spell)

### Extra descriptions — done
`ExtraDesc` type with `Keywords []string` and `Description string` on both `Room` and `Object`.
Look handler falls back to extra desc search when standard target resolution doesn't match.
Uses `AllowUnresolved` on the look command's target spec.

### Exit description — done
`Description string` on `Exit`. Look handler shows it when targeting an exit direction.

### Pickproof doors — done
`Pickproof bool` on `Lock`. Data only — no pick command exists yet.

### Sector type — deferred
Cosmetic without movement points. Water traversal (the one meaningful case) deferred to when water system exists.

### Spawn max-existing limits — deferred
Zone-level concern, not a room property. Deferred until zone reset improvements.

---

## Mobiles — TODO

### Import
CircleMUD mob data exists at `_output/mob/` in the parser repo. Need to:
1. Import mob definitions into `circlemud/mobiles/`
2. Translate zone reset M commands into `Room.MobSpawns`
3. Translate zone reset G/E commands into `Mobile.Inventory` and `Mobile.Equipment`

### Behavior flags (action bitvector)
These are mob flags (like room flags — properties of the mob, not effects on others).

| CircleMUD Flag | Description | Notes |
|---|---|---|
| SENTINEL | Doesn't wander | Need mob wandering system |
| SCAVENGER | Picks up valuables from the ground | |
| AWARE | Cannot be backstabbed | |
| AGGRESSIVE | Attacks all visible players | |
| STAY_ZONE | Won't wander outside its zone | |
| WIMPY | Flees at ~20% HP | |
| AGGR_EVIL/GOOD/NEUTRAL | Attacks aligned players | Needs alignment system |
| MEMORY | Remembers and retaliates | |
| HELPER | Assists other mobs in same room | |

### Affection flags
These are perks (effects on the mob itself, composable from multiple sources).

| CircleMUD Flag | Our grant key | Notes |
|---|---|---|
| INVISIBLE | `invisible` | |
| DETECT_INVIS | `detect_invis` | |
| SENSE_LIFE | `sense_life` | |
| WATERWALK | `waterwalk` | |
| SANCTUARY | modifier `core.defense.all.absorb.pct 50` | |
| INFRAVISION | `infravision` | Already defined |
| SNEAK | `sneak` | |
| HIDE | `hide` | |
| NOCHARM | `nocharm` | |
| NOSUMMON | `nosummon` | |
| NOSLEEP | `nosleep` | |
| NOBASH | `nobash` | |
| NOBLIND | `noblind` | |

### Other mob gaps
- **BaseStats on mobs** — add `BaseStats map[StatKey]int` to `Mobile`
- **Gold drops** — deferred until currency system
- **Alignment** — deferred until alignment system
- **Attack type flavor** — deferred

---

## Objects — TODO

### Import
CircleMUD object data exists at `_output/obj/` in the parser repo. Need to:
1. Import object definitions into `circlemud/objects/` (replacing current key stubs)
2. Map CircleMUD object types to our flags/perks (weapon, armor, container, etc.)
3. Translate zone reset O/P commands into `Room.ObjSpawns` with nested contents

### Object flags
Extend existing `Flags []string` with new values:

| Flag | Description |
|---|---|
| `invisible` | Object starts invisible |
| `no_drop` | Cursed — cannot be dropped |
| `no_sell` | Shopkeepers will not buy or sell |

### Other object gaps
- **Cosmetic aura** — `Aura string` field (GLOW, HUM, BLESS)
- **Light sources** — `light` grant on equipment (basic version done); finite burn time not yet supported
- **Boat** — object flag for water traversal
- **Spell delivery items** — deferred until spell system
- **Food/drink** — deferred until hunger/thirst
- **Item cost** — deferred until currency/shops
- **Alignment restrictions** — deferred until alignment
- **Affect fields** — mostly covered by perks; HP max modifier needs a well-known key

---

## Zones — partially done

29 zones imported with name, reset mode, and lifespan.

### Zone reset commands — TODO
The zone files contain imperative reset commands that define the world state:

| Command | Description | Our equivalent | Status |
|---|---|---|---|
| M — load mob to room | `Room.MobSpawns` | Need to translate from zone data |
| O — load object to room | `Room.ObjSpawns` | Need to translate from zone data |
| G — give object to mob | `Mobile.Inventory` | Need to translate from zone data |
| E — equip object on mob | `Mobile.Equipment` | Need to translate from zone data |
| P — put object in object | `ObjectSpawn.Contents` | Need to translate from zone data |
| D — set door state | `Exit.Closure` | Already handled by room import |
| R — remove object | Skip | Decaying objects handle this |

### Spawn max-existing limits
Zone reset commands include a `max` field (max live instances when spawning). Our spawn system has no equivalent. Deferred.

---

## Shops — deferred

Shops are a new asset type. Fully deferred until currency system exists.

---

## Future Systems

Systems that multiple gaps depend on:

| System | Needed by |
|---|---|
| Currency | Gold drops, item cost, shops |
| Alignment | AGGR flags, item restrictions, shop restrictions |
| Spell system | Scroll/wand/staff/potion, mob spell abilities, nomagic enforcement |
| Hunger/thirst | Food and drink objects |
| Water traversal | Sector types, boat flag, waterwalk |
| Saving throws | Save modifiers on objects, spell effects |
