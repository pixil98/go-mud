# Scoped Perks and Symmetric Bypass

## Status

**Partially implemented.** The room-flag pattern is in place; the broader
unification is a direction, not a commitment. This document captures the
shape so future systems (visibility, water-breathing, alignment protection,
emitter auras) land coherently rather than reinventing the same primitive.

## Context

The codebase has a recurring pattern: an entity holds a property that
restricts something, and another actor holds a grant that lifts the
restriction. The first instance was dark/darkvision, then nomagic and
water joined.

We unified the bypass side under a single grant key:

```json
{ "type": "grant", "key": "ignore_room_flag", "arg": "dark" }
```

The check at the consequence site is `room.Restricts(actor, RoomFlagDark)`,
which is `room has flag X AND actor lacks ignore_room_flag:X`. See
[internal/game/room.go](../../internal/game/room.go) for the helper and
[internal/assets/room.go](../../internal/assets/room.go) for the flag enum.

## The pattern is more general than rooms

The same shape appears across scopes:

| Restriction (entity holds) | Bypass (actor holds)              | Scope    |
|----------------------------|-----------------------------------|----------|
| `dark` flag                | `ignore_room_flag:dark`           | room     |
| `nomagic` flag             | `ignore_room_flag:nomagic`        | room     |
| `water` flag               | `ignore_room_flag:water`          | room     |
| `invisible` perk on B      | `detect_invis` on observer A      | actor↔actor |
| `hide` perk on B           | `sense_life` on observer A        | actor↔actor |
| `protect_evil` on defender | (alignment-side flag on attacker) | actor↔actor |

The actor↔actor cases are pairwise rather than unilateral, but the data
shape is identical: a property on the target side, a bypass on the other
side. The current `ignore_room_flag` grant key bakes "room" into the name
because that's the only scope today; once a second scope appears the
generalization becomes useful.

## Direction: scoped perks with symmetric bypass

The proposal is to treat *every* such property as a perk on whichever
entity owns it, and express bypass with one of:

- `ignore_room_flag:<key>` — actor ignores a room-side property
- `ignore_actor_perk:<key>` — observer ignores a target-actor's property
- (future) `ignore_zone_flag:<key>`, `ignore_item_flag:<key>`, etc.

Or, once we have ≥2 scopes, collapse to a single key with a structured arg:
`ignore:<scope>:<key>`. We don't have to decide between these until the
second scope ships; what matters is that the *shape* is consistent.

### Why perks instead of bespoke flags

Today, room flags and actor perks are different code paths:
`Room.Flags []RoomFlag` (literal slice check) vs
`PerkCache.HasGrant(key, arg)` (sourced, timed, aggregated). A unified
model means one runtime check site per scope, plus all the perk
machinery (timed effects, multiple sources, aggregation) becomes
available to room-side properties for free.

Concrete wins:

- **Timed room properties.** A "darkness" spell that adds dark to a room
  for 30 ticks is just a timed perk on the room — same machinery as the
  existing `TimedPerkCache` already used by buffs.
- **Lifted properties** (the "null race" idea). An actor or item with an
  emitter perk can contribute to its room's PerkCache while present. The
  room doesn't need to know what's in it; the cache aggregates.
- **Asset-defined properties.** New restrictions don't need engine code:
  asset authors define a perk key, give actors an `ignore_room_flag` grant
  for it, and any consequence handler that consults the cache picks it up.

### Authoring stays terse

Flags-as-perks would balloon the JSON:

```json
"flags": ["dark", "nomagic"]
```

becomes

```json
"perks": [
  {"type": "flag", "key": "dark"},
  {"type": "flag", "key": "nomagic"}
]
```

The compromise: **flags remain authoring shorthand**, but at instance
load time each flag expands into a permanent entry in the entity's
PerkCache. Runtime checks go through the cache (`room.HasPerk("dark")`
or similar), not `HasFlag`. JSON stays terse; runtime is unified.

## Lift mechanism (future)

Today, actor/item perks live on the actor/item. The "null race that
negates magic in their room" use case requires a *lift*: a perk
declared on the emitter contributes to the containing room's PerkCache
while the emitter is present.

Sketch:

- A perk declares `"lift": "room"` (or `zone`, etc.).
- When the emitter enters a room, the room's PerkCache adds a new
  source bound to the emitter.
- When the emitter leaves (or is destroyed), the source is removed.
- The room's `Restricts` check sees the perk just like a static flag.

This is roughly the inverse of the existing zone→actor perk inheritance
(zone perks bubble down into actor PerkCaches). Both directions become
the same mechanism: PerkCache aggregates from named sources and emits
the union.

## Pairwise checks (visibility, alignment)

Room flags are unilateral: the room either has the property or it
doesn't. Visibility is pairwise: A sees B based on B's invisibility and
A's detect_invis. The same perk model handles this — the check just
consults two caches:

```
canSee(A, B) =
    !B.HasPerk("invisible") || A.HasPerk("ignore_actor_perk:invisible")
```

The bypass is on the observer, the property is on the target. Any
visibility-style mechanic (sense_life vs hide, see_invis vs invis,
truesight vs glamour) follows the same shape.

## Migration plan

Not a single sweep — opportunistic, when new systems land:

1. **Now (done).** Room flags + `ignore_room_flag:<key>` bypass. Restricts
   helper. Flag is a typed string enum on Room.

2. **When invis/detect_invis is implemented.** Add an actor-perk shape
   parallel to room flags: `ignore_actor_perk:<key>` bypass on the
   observer. Implement `canSee(A, B)` using the symmetric primitive.
   This forces a decision on whether to introduce a unified
   `ignore:<scope>:<key>` form or stay with per-scope keys.

3. **When timed room perks are needed** (e.g. darkness spell). Extend
   `RoomInstance.Perks` to admit timed entries (the
   [TimedPerkCache](../../internal/game/perkcache.go) already supports
   this on actors). At that point, formalize "flag → permanent perk in
   the cache at load time" so all checks go through one path.

4. **When the emitter/lift use case lands** (null race, aura items).
   Add `lift: "room"` semantics to perks and the source-attach/detach
   plumbing on PerkCache.

Steps 2–4 are independent — each new system pays for the unification
incrementally. We don't refactor existing code without a concrete user.

## Open questions

- **Bypass key naming.** `ignore_room_flag:<key>` works for one scope.
  Adding `ignore_actor_perk:<key>` is fine but starts a family of keys.
  Worth collapsing to `ignore:<scope>:<key>` when we add the second
  scope; not worth doing it preemptively.

- **Flag → perk expansion.** When does this happen — at asset load
  (everything is a perk in memory, flags are authoring sugar) or at
  check time (HasFlag forwards to PerkCache lookup)? Load-time is
  cleaner but requires touching the asset → instance pipeline. Check-
  time is a smaller change but keeps two storage paths alive. Defer
  the choice until step 3 forces it.

- **Pairwise checks vs unilateral.** Room flags are O(1) (one cache
  lookup). Visibility is O(observers × targets) per resolution. The
  perk model unifies the shape but the cost profile differs; not a
  blocker but worth noting when the visibility system lands.

- **Asset-side perk validation.** Today room flags are validated against
  a known set (`validRoomFlags`). If properties become arbitrary perks,
  do we validate keys per scope, or accept any string? Some validation
  is healthy (typos in asset JSON should fail loudly).
