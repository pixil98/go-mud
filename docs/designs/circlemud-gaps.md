# CircleMUD Compatibility Gaps

Gaps identified while reviewing CircleMUD asset specs against our current asset types.
The goal is not a perfect 1:1 mapping — just enough to support a sample world with similar play experience.

## Import Status

1878 rooms across 29 zones imported into `circlemud/rooms/` and `circlemud/zones/`.
569 mobiles imported into `circlemud/mobiles/`.
678 objects imported into `circlemud/objects/`.
Alternate config at `config.circlemud.json`.

Zone reset commands (M/O/G/E/P/D) translated into declarative spawns:
- M commands → `Room.MobSpawns` (723 rooms)
- O commands → `Room.ObjSpawns` with nested contents from P commands (138 rooms)
- G/E commands → `Mobile.Inventory` and `Mobile.Equipment` (first-wins per slot to handle mobs placed with different loadouts)

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

## Mobiles — DONE

### Import — done
569 mobs imported via `tools/import_circlemud_mobs.py`.
Zone reset M/G/E commands translated into `Room.MobSpawns`, `Mobile.Inventory`, and `Mobile.Equipment`.
Equipment uses first-wins per slot when the same mob is placed with different gear.
Unmapped properties (alignment, gold, gender, skipped flags) preserved in `circlemud_unused`.

### Behavior flags (action bitvector) — done
Mapped to `Mobile.Flags`:

| CircleMUD Flag | Our flag | Status |
|---|---|---|
| SENTINEL | `sentinel` | Done |
| SCAVENGER | `scavenger` | Done |
| AWARE | `aware` | Done |
| AGGRESSIVE | `aggressive` | Done |
| STAY_ZONE | `stay_zone` | Done |
| WIMPY | `wimpy` | Done |
| MEMORY | `memory` | Done |
| HELPER | `helper` | Done |
| AGGR_EVIL/GOOD/NEUTRAL | — | Deferred (needs alignment system) |

### Affection flags — done
Mapped to perk grants:

| CircleMUD Flag | Our grant key | Status |
|---|---|---|
| INVISIBLE | `invisible` | Done |
| DETECT_INVIS | `detect_invis` | Done |
| SENSE_LIFE | `sense_life` | Done |
| WATERWALK | `waterwalk` | Done |
| SANCTUARY | modifier `core.defense.all.absorb.pct 50` | Done |
| INFRAVISION | `infravision` | Done |
| SNEAK | `sneak` | Done |
| HIDE | `hide` | Done |
| NOCHARM | `nocharm` | Done |
| NOSUMMON | `nosummon` | Done |
| NOSLEEP | `nosleep` | Done |
| NOBASH | `nobash` | Done |
| NOBLIND | `noblind` | Done |

### Other mob gaps
- **BaseStats on mobs** — add `BaseStats map[StatKey]int` to `Mobile`
- **Gold drops** — deferred until currency system (preserved in `circlemud_unused`)
- **Alignment** — deferred until alignment system (preserved in `circlemud_unused`)
- **Attack type flavor** — deferred

---

## Objects — DONE

### Import — done
678 objects imported via `tools/import_circlemud_objs.py`.
Zone reset O/P commands translated into `Room.ObjSpawns` with nested contents.
Alias-matching extra descriptions promoted to `detailed_desc`; non-overlapping kept as `extra_descs`.
Unmapped properties (cost, rent, weight, skipped effects) preserved in `circlemud_unused`.
Containers with locks referencing missing key objects stored as keyless with original key vnum in `circlemud_unused`.

### Object type mapping — done

| CircleMUD Type | Our mapping | Status |
|---|---|---|
| WEAPON | `attack` grant perk with damage dice | Done |
| ARMOR | `core.combat.ac.flat` modifier perk | Done |
| LIGHT | `light` grant perk | Done (finite burn time deferred) |
| CONTAINER | `container` flag + closure with lock/key | Done |
| KEY | No special properties | Done |
| TREASURE/OTHER/TRASH | No special properties | Done |
| Spell items (SCROLL, WAND, STAFF, POTION) | Type + values in `circlemud_unused` | Deferred until spell system |
| FOOD/DRINKCON/FOUNTAIN | Type + values in `circlemud_unused` | Deferred until hunger/thirst |
| BOAT | Type in `circlemud_unused` | Deferred until water traversal |

### Object flags — done

| Flag | Description | Status |
|---|---|---|
| `invisible` | Object starts invisible | Done |
| `no_drop` | Cursed — cannot be dropped | Done |
| `no_sell` | Shopkeepers will not buy or sell | Done |
| `wearable` | Derived from wear bitvector slots | Done |
| `immobile` | No WEAR_TAKE in source data | Done |

### Affect fields — done
Mapped to modifier perks: STR, DEX, INT, WIS, CON, CHA → `core.stats.*`;
HIT → `core.resource.hp.max`; AC → `core.combat.ac.flat`;
HITROLL → `core.combat.attack.flat`; DAMROLL → `core.damage.all.flat`.

### Other object gaps
- **Cosmetic aura** — GLOW, HUM, BLESS preserved in `circlemud_unused` effects
- **Light sources** — `light` grant done; finite burn time in `circlemud_unused`
- **Boat** — deferred until water traversal
- **Spell delivery items** — deferred until spell system
- **Food/drink** — deferred until hunger/thirst
- **Item cost** — deferred until currency/shops (preserved in `circlemud_unused`)
- **Alignment restrictions** — deferred until alignment (preserved in `circlemud_unused`)

---

## Zones — DONE

29 zones imported with name, reset mode, and lifespan.

### Zone reset commands — done

| Command | Description | Our equivalent | Status |
|---|---|---|---|
| M — load mob to room | `Room.MobSpawns` | Done |
| O — load object to room | `Room.ObjSpawns` | Done |
| G — give object to mob | `Mobile.Inventory` | Done |
| E — equip object on mob | `Mobile.Equipment` | Done |
| P — put object in object | `ObjectSpawn.Contents` | Done |
| D — set door state | `Exit.Closure` | Done |
| R — remove object | Skip | Decaying objects handle this |

### Spawn max-existing limits — deferred
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
