# World Design: The Reaches of Aldenmere

## Overview

The world is built outward from **Millbrook**, a modest market town serving as the
starting area for new players. Zones chain together geographically rather than all
branching from a single hub. Rooms at the edge of one zone serve as the natural
transition into the next — there are no dedicated "travel corridor" zones.

---

## Zone Design Principles

### Size
- **Town zones**: 20–35 rooms. Compact enough to feel like a real place.
- **Adventure zones**: 40–80+ rooms. Large enough to feel like a world to explore.
  Difficulty should escalate naturally from the entry point inward — easier areas
  near zone edges, harder areas near the center or boss rooms.

### Connectivity
- Zones chain outward from Millbrook.
- Every zone should have at least one entry and one exit to an adjacent zone.
- Rooms at a zone's boundary should feel like the edge of that zone, not a void.
- Dead-end boss rooms are expected. Dead-end zones should be rare.

### Exits
- Use all eight compass directions (N/S/E/W + NE/NW/SE/SW) plus up/down.
- Cross-zone exits require both `zone_id` and `room_id` in the exit spec.
- Doors and locks should have narrative justification (a gate, a sealed tomb, etc.)

### Atmosphere
- Each zone should have a consistent aesthetic players feel immediately.
- Write room descriptions in present tense, second person ("You stand...").
- Describe what players can *sense*: sight, sound, smell, temperature.
- `description` is 4–6 sentences. Cover what players see first, secondary details,
  sounds and smells, and a sense of who or what has been here before them.

### Reset Mode
| Mode       | Behavior                                            | Use for                        |
|------------|-----------------------------------------------------|--------------------------------|
| `lifespan` | Resets on timer even if players are present         | Towns, grinding zones          |
| `empty`    | Resets on timer only when no players are present    | Dungeons, exploration zones    |
| `never`    | Never resets (admin only)                           | Unique/scripted areas          |

- **Towns**: `lifespan` with a long interval (e.g., `"6h"`). NPCs and shop
  containers respawn predictably.
- **Exploration zones**: `empty` with a moderate interval (e.g., `"30m"`).
  Players don't lose progress mid-dungeon.
- **Grinding zones**: `lifespan` with a shorter interval. Enemies respawn around players.

---

## Room Design Principles

### Structure
Every room needs:
- `name`: Short (2–5 words). Used in navigation output.
- `description`: 4–6 sentence scene-setter. Present tense, second person.
- At minimum one way in and one way out.
- `zone_id`: The zone this room belongs to.

### Objects in Rooms
Items should feel placed with intention. Two approaches:

1. **Described in the room**: Use the object's `long_desc` to explain its
   position (e.g., "A woodcutter's axe leans against a nearby stump, its
   blade notched from use."). Spawn it directly in the room. Good for lone
   items where a container would feel forced.

2. **Inside a flavorful container**: Place them in open containers like a
   weapon rack in a smithy, a display case in a shop, a crate in a warehouse.
   Open containers: `"flags": ["container", "immobile"]`, no `closure`.
   Container contents go in the room's `object_spawns`:
   `{"object_id": "rack", "contents": [{"object_id": "sword"}]}`

**Locked containers** (chests, strongboxes) are reward mechanics — they only
make sense when a key exists and is gated behind effort:
- The key is dropped by a boss mob in that zone.
- The key is purchasable from a vendor (for high-value shop containers).
- The key is hidden elsewhere in the world as a discovery reward.

Do not add locked containers without a corresponding key placement. Do not
add keys without a corresponding locked container to open.

### Mobile Spawns
- `mobile_spawns` is a list of mob IDs; each spawns one instance on reset.
- Don't overload rooms — 1–3 mobs per room is plenty.
- Town NPCs give the zone life even when the zone is peaceful.

### Perks on Rooms
- Rooms inherit perks from their zone automatically via the perk chain.
- Add `perks` on a room to override zone behavior for just that room.
- `{ "type": "grant", "key": "peaceful" }` prevents combat in that room.

---

## Mob Design Principles

### Combat System
Combat uses `d20 + attack_mod ≥ target AC` to hit, then `dice_roll + damage_mod`
for damage. Hit chances for level-appropriate fights land around 55–70%.

### Stats by Level
| Level | HP      | AC | Attack Mod | Damage Mod | Typical Attack Die |
|-------|---------|----|------------|------------|--------------------|
| 1     | 6–10    | 8  | 0          | 0          | 1d4                |
| 2     | 12–18   | 9  | +1         | 0          | 1d4                |
| 3     | 20–30   | 10 | +2         | +1         | 1d4                |
| 4     | 28–40   | 11 | +3         | +1         | 1d6                |
| 5     | 40–55   | 12 | +4         | +2         | 1d6                |
| 6     | 55–70   | 13 | +5         | +3         | 1d8                |
| 7     | 70–90   | 14 | +6         | +3         | 1d8                |
| 8     | 90–110  | 15 | +7         | +4         | 1d8                |
| 9     | 120–150 | 15 | +8         | +4         | 1d8                |
| 10    | 145–160 | 16 | +9         | +5         | 1d8                |

Attack dice are flexible — use whatever fits the creature. Multiple `attack` grant
perks give multiple independent attack rolls per round (each checked against AC
separately). Bosses often use this for a distinctive multi-attack feel.

All combat mobs need:
- `{ "type": "grant", "key": "autoattack" }` — mob attacks automatically each round
- At least one `{ "type": "grant", "key": "attack", "arg": "1d6" }` — attack dice

### Aliases
Aliases must be **single words**. The target system matches one input token exactly.
Players type `kill wolf` or `kill leader`, never `kill bandit leader`.

Use distinctive single words:
- `["wolf", "grey", "mangy"]` for a grey mangy wolf
- `["leader", "bandit", "scarred"]` for the scarred bandit leader

### Difficulty Escalation Within Zones
Scale difficulty from zone entry inward:
- Entry rooms: 2–3 levels below zone cap (weak scouts, young creatures)
- Mid-zone: zone cap − 1
- Boss areas: zone cap or cap + 2

---

## Object Design Principles

### Wearable Items
- `"flags": ["wearable"]` and a `wear_slots` array.
- Slots: `wield`, `off_hand`, `hold`, `body`, `head`, `neck`, `arms`, `hands`,
  `wrist`, `waist`, `legs`, `feet`, `finger`, `about`, `light`.
- Weapons grant attack: `{ "type": "grant", "key": "attack", "arg": "1d6" }`.
- Armor modifies AC: `{ "type": "modifier", "key": "core.combat.ac", "value": 4 }`.

### Loot Placement
Mobs cannot currently drop items. All loot must be placed in the world via
`object_spawns` (directly in a room, inside a container, or inside a locked
container requiring a key). Design loot placement before building the zone —
knowing where the rewards are shapes what the exploration feels like.

---

## World Map

```
                     [Northern Wilds]  (future)
                            |
                      [King's Road]    (future)
                            |
           [River Dale] -- [MILLBROOK] -- [THE DARKWOOD] -- [Forest Depth] (future)
           (future)            |
                         [The Thornway] (future)
```

### Current Zones

| Zone         | ID         | Rooms | Level Range | Reset        | Design doc                       |
|--------------|------------|-------|-------------|--------------|----------------------------------|
| Millbrook    | millbrook  | 30    | n/a         | lifespan, 6h | [zones/millbrook.md](zones/millbrook.md) |
| The Darkwood | darkwood   | 43    | 2–10        | empty, 30m   | [zones/darkwood.md](zones/darkwood.md)   |
