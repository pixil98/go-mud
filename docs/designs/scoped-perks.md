# Scoped Perks and Symmetric Bypass

## Status

**Foundation in place.** Room flags are perks, the bypass key is unified,
and the audience is encoded by key-prefix convention. The remaining steps
(visibility, timed room perks, emitter lift) are independent features that
plug into the same primitive.

## Context

The codebase has a recurring pattern: an entity holds a property that
restricts something, and another actor holds a grant that lifts the
restriction. The first instance was dark/darkvision, then nomagic, water,
peaceful, and the rest of the room flags joined.

Properties are plain grant perks; the bypass uses a single grant key
`ignore_restriction` whose arg names the property being ignored:

```json
// Room (or zone, world) declares the property
{ "type": "grant", "key": "room_dark" }

// Actor (race, spell, equipment) declares the bypass
{ "type": "grant", "key": "ignore_restriction", "arg": "room_dark" }
```

The check at the consequence site is `room.Restricts(actor, RoomFlagDark)`,
which is `room resolves room_dark in its PerkCache AND actor lacks
ignore_restriction:room_dark`. See [internal/game/room.go](../../internal/game/room.go)
for the helper and [internal/assets/room.go](../../internal/assets/room.go)
for the well-known room flag keys.

## The pattern is more general than rooms

The same shape appears across scopes:

| Restriction (entity holds) | Bypass (actor holds)               | Scope        |
|----------------------------|------------------------------------|--------------|
| `room_dark`                | `ignore_restriction:room_dark`     | room         |
| `room_nomagic`             | `ignore_restriction:room_nomagic`  | room         |
| `room_water`               | `ignore_restriction:room_water`    | room         |
| `room_peaceful`            | `ignore_restriction:room_peaceful` | room         |
| `invisible` on B           | `detect_invis` on observer A       | actor↔actor  |
| `hide` on B                | `sense_life` on observer A         | actor↔actor  |

The actor↔actor cases are pairwise rather than unilateral, but the data
shape is identical: a property on the target side, a bypass on the other
side, both expressed as grant perks.

## Why perks instead of bespoke flags

Before this work, room flags lived in a typed `[]RoomFlag` slice with
its own `HasFlag` check, separate from the perk machinery. Unifying gives
us one runtime path for both, and all the perk machinery becomes
available to room-side properties for free:

- **Timed room properties.** A "darkness" spell that adds dark to a room
  for 30 ticks is just a timed perk on the room — same machinery as the
  existing `TimedPerkCache` already used by buffs. No new code needed.
- **Inherited properties.** A zone declares `room_water` once and every
  room in the zone resolves it via the existing PerkCache subscription
  chain (room subscribes to zone, zone to world).
- **Lifted properties** (future "null race"). An actor or item with an
  emitter perk could contribute to its room's PerkCache while present.
- **Asset-defined keys.** New restrictions don't need engine code: asset
  authors define a perk key, give actors `ignore_restriction:<key>` to
  bypass, and any consequence handler that consults the cache picks it up.

## Audience by key prefix

Rather than a separate `target` field on `Perk`, the audience is encoded
in the key's prefix:

- `room_<name>` — room property (consumed by room or by anything checking
  the room's perks: visibility, movement, combat init, etc.)
- `<name>` (no prefix) — actor property (the implied default; existing
  actor-side keys like `peaceful` were renamed `room_peaceful` only when
  they're properly room properties).

This keeps the data shape minimal — perks are still `{type, key, value, arg}`
— and makes audience visible at a glance in JSON. It also keeps keys
unique across scopes by construction, so no consumer ever cross-matches.

The runtime never inspects the prefix; consumers query by full key. The
prefix is a convention, not a mechanism.

## Inheritance

PerkCache subscriptions form a chain:

- World holds top-level perks.
- Each zone subscribes to the world.
- Each room subscribes to its zone.
- Each character subscribes to their current room (re-attached on move).

A zone declaring `room_dark` makes every room in the zone resolve it
(rooms inherit via the zone source). A world declaring some
hypothetical `actor_*` perk would propagate via room → zone → world to
every actor anywhere.

Subscriptions don't filter — a room's cache contains everything its zone
declared, including `actor_*`-prefixed keys destined for occupants.
Consumers query by key; entries of the wrong audience sit harmlessly
because nobody at that level asks for them.

Mobiles do **not** subscribe to their room's cache today — they hold their
own perks but don't inherit the room/zone chain. That's a deliberate gap.
Wiring it would let consequence-site checks move from
`room.Restricts(actor, flag)` onto the actor (the actor would resolve the
room's flags directly via inheritance), but it requires a filter
mechanism: a player casting a room-wide AC buff shouldn't accidentally
buff the mob they're fighting. The `target`/audience metadata we
deliberately deferred is what that filter would key off; revisit when
the use case lands.

## Pairwise checks (visibility, alignment)

Room flags are unilateral: the room either resolves the property or
doesn't. Visibility is pairwise: A sees B based on B's invisibility and
A's detect_invis. The same perk model handles this — the check just
consults two caches:

```
canSee(A, B) =
    !B.HasGrant("invisible", "") || A.HasGrant("ignore_restriction", "invisible")
```

(If we later want a separate bypass family for actor↔actor checks
distinct from `ignore_restriction`, we can introduce `ignore_actor_perk`.
For now `ignore_restriction` works for both — it's just an actor-held
permission to ignore some other named perk.)

## Migration plan

1. **Done.** Room flags expressed as grant perks with `room_*`-prefixed
   keys. Single `ignore_restriction` bypass grant. `room.Restricts(actor,
   RoomFlagX)` helper queries the room's PerkCache. `Room.HasFlag` and
   `Room.Flags` removed; flags array dropped from asset JSON.

2. **Visibility (when invis/detect_invis ships).** Implement
   `canSee(A, B)` using the symmetric grant + `ignore_restriction`
   primitive. Forces a call on whether actor↔actor bypasses share the
   `ignore_restriction` key or grow their own family.

3. **Timed room perks (when a darkness/light spell ships).** No
   structural work — flags are already perks; timed entries work via
   the existing `TimedPerkCache` machinery. Wire the spell effect to
   add a timed perk to the room.

4. **Mobile perk inheritance.** When mobs need to inherit room/zone
   perks (room flags affecting mob behavior, zone-wide buffs reaching
   mobs, etc.), subscribe each mob's PerkCache to its room. Once that's
   in place, restriction checks can move from
   `room.Restricts(actor, flag)` onto the actor directly — the actor's
   own cache will already resolve the room flags via inheritance.

5. **Lift mechanism (when the null-race / aura-item use case lands).**
   Add `lift: "room"` semantics to perks and the source-attach/detach
   plumbing on PerkCache so emitters contribute to their container's
   cache while present.

Steps 2–5 are independent and pay for themselves when a concrete feature
needs them. We don't refactor without a user.

## Open questions

- **Bypass key naming for actor↔actor.** `ignore_restriction:<key>`
  works for room properties today. When invis/detect_invis lands, we
  decide whether the same key serves actor↔actor or a separate family
  (`ignore_actor_perk:<key>`) reads better. Defer until then.

- **Pairwise checks vs unilateral.** Room flags are O(1) per actor (one
  cache lookup). Visibility is O(observers × targets) per resolution.
  The perk model unifies the shape but the cost profile differs; not a
  blocker but worth noting when the visibility system lands.

- **Validation of well-known keys.** Today no validation enforces that
  e.g. `room_dark` is the only `dark`-style key (vs `darkness`, `dim`,
  etc.). Asset authors can drift. If drift starts happening, add a
  registry of well-known keys per audience and warn on unknown ones.
