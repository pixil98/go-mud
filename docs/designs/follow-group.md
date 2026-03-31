# Follow and Group System Design

## Context

Follow, group, and moveFollowers are tightly coupled to `*game.CharacterInstance`. Mobs can't follow or be group members, which blocks the summoner class archetype. The goal is to make these systems operate on interfaces with pointer-based follow trees so both players and mobs can participate.

## Core Model: Pointer-Based Follow Tree

Each actor stores a pointer to who they follow (parent) and a map of pointers to who follows them (children). The tree is walked directly via pointers — no ID-based lookups, no room scanning.

```
         [Leader]
        /        \
  [Player A]   [Summoned Mob]
      |
  [Player B]
```

### Why Pointers Instead of IDs

- Tree traversal is direct — no room scanning to find followers
- No lookup adapters or registries needed
- `moveFollowers` walks `leader.GetFollowers()` instead of iterating every actor in the room
- Loop detection walks up via `GetFollowing()` pointers — no `FollowPlayerLookup`
- Subtree pruning is natural: if a follower isn't in the room, skip the entire branch

### Follow vs Group

Following is unilateral — anyone can follow anyone without consent. Group membership requires consent: the leader explicitly adds members. You must be following to join a group, but following alone does not grant group access.

Groups remain an explicit data structure (`Group` struct with a member map), not derived from the follow tree. This preserves the consent model while allowing the follow tree to handle movement cascading independently.

## Data Model

### `FollowTarget` Interface (`internal/game/actor.go`)

Defined in the `game` package (where it's stored/consumed). Both `CharacterInstance` and `MobileInstance` satisfy it.

```go
type FollowTarget interface {
    Id() string
    Name() string
    Notify(msg string)
    IsInCombat() bool
    Location() (string, string)
    Move(from, to *RoomInstance)
    GetFollowing() FollowTarget
    SetFollowing(FollowTarget)
    GetFollowers() []FollowTarget
    AddFollower(FollowTarget)
    RemoveFollower(id string)
}
```

Note: `game` can't import `shared` (circular dependency), so this is a separate interface from `shared.Actor`. Both CI and MI satisfy both interfaces independently.

### Fields on `ActorInstance`

```go
following FollowTarget            // who this actor follows (nil = not following)
followers map[string]FollowTarget // actors following this actor, keyed by Id()
```

Lock-aware accessors live on `CharacterInstance` and `MobileInstance` (each has its own mutex). `SetFollowing(target)` manages reverse links automatically: adds self to new leader's followers, removes self from old leader's followers.

### `GroupMember` Interface (`internal/game/group.go`)

Minimal interface — only methods actually called on group members:

```go
type GroupMember interface {
    Id() string
    Name() string
    Notify(msg string)
    Resource(name string) (int, int)
    GetFollowing() FollowTarget
    SetFollowing(FollowTarget)
    GetGroup() *Group
    SetGroup(*Group)
}
```

`Group.members` becomes `map[string]GroupMember`. `ForEachPlayer` type-asserts `*CharacterInstance` to yield only players (preserves NATS publishing). `ForEachMember` iterates all members including mobs.

## Movement Cascading

`moveFollowers` walks the leader's follower tree directly. No room scanning.

```go
func moveFollowers(leader game.FollowTarget, fromRoom, toRoom *RoomInstance, direction string) {
    for _, fl := range leader.GetFollowers() {
        // Prune: follower not in same room — skip entire subtree.
        _, followerRoom := fl.Location()
        if followerRoom != fromRoom.Room.Id() {
            continue
        }
        // Prune: follower in combat — skip subtree (their followers won't move either).
        if fl.IsInCombat() {
            fl.Notify(fmt.Sprintf("%s leaves %s without you.", leader.Name(), direction))
            continue
        }
        fl.Move(fromRoom, toRoom)
        fl.Notify(fmt.Sprintf("You follow %s.\n%s", leader.Name(), toRoom.Describe(fl.Name())))
        moveFollowers(fl, fromRoom, toRoom, direction)
    }
}
```

Subtree pruning: if a follower isn't in the same room or is in combat, the entire branch below them is skipped. Their followers would only move if they do.

## Loop Detection

Walk up the tree via `GetFollowing()` pointers. No lookup adapter needed.

```go
func wouldCreateLoop(follower, leader FollowTarget) bool {
    current := leader
    for i := 0; i < 100; i++ {
        next := current.GetFollowing()
        if next == nil { return false }
        if next.Id() == follower.Id() { return true }
        current = next
    }
    return true // safety: 100+ links treated as loop
}
```

## Notifications

Follow notifications use `Notify()` directly on the stored pointer instead of routing through NATS `Publisher.Publish(SinglePlayer(...))`. Since `MobileInstance.Notify()` is a no-op, no `IsCharacter()` check is needed — mobs silently ignore notifications.

## Cleanup on Disconnect/Despawn

When a player disconnects or a mob despawns:
1. Call `SetFollowing(nil)` — removes reverse link from old leader's followers
2. Iterate `GetFollowers()` and call `follower.SetFollowing(nil)` for each — orphaned followers stop following

## MobileInstance Changes

- Add `GetFollowing`, `SetFollowing`, `GetFollowers`, `AddFollower`, `RemoveFollower` methods
- Add `Move(from, to *RoomInstance)`: calls `from.RemoveMob(mi.Id())` + `to.AddMob(mi)`
- Change `GetGroup()` from hardcoded `nil` to delegate to `ActorInstance`
- Add `SetGroup(*Group)` method

## Handler Interfaces

Each handler defines a narrow interface with only the methods it calls (existing pattern):

- **FollowActor**: `Id`, `Name`, `Notify`, `GetFollowing`, `SetFollowing` (+ `FollowTarget` for the target)
- **MoveActor**: `Id`, `Name`, `Notify`, `Location`, `IsInCombat`, `Move` (+ tree access for followers)
- **GroupActor**: `Id`, `Name`, `Notify`, `GetFollowing`, `SetFollowing`, `GetGroup`, `SetGroup`, `Resource`

## Summon Mob Effect

New `summon_mob` ability effect (modeled after `spawn_obj`):

- Config: `mobile_id` (required) — the mob asset ID to summon
- At runtime: spawns a `MobileInstance`, places it in the caster's room, calls `mi.SetFollowing(caster)` to make it follow the caster
- The mob then moves with the caster automatically via `moveFollowers`

## Subgroup Considerations (Deferred)

When a summoner is in a group, their summoned mobs form a "subgroup" under them. Open questions for the summoning implementation:
- Display: should `group` show summoned mobs indented under their summoner?
- Targeting: can other group members heal the summoner's mobs?
- Lifecycle: when a summoner leaves the group, do their mobs leave too?

The structural work here ensures mobs *can* be group members. The policy of how they're managed is separate.
