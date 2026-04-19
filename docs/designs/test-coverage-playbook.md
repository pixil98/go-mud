# Test Coverage Playbook

A reusable methodology for raising unit-test coverage one package at a
time. Each coverage push picks a target package and an exit target, then
walks the same four phases.

This document is a **living** reference. The **Coverage Ledger** and the
**Interface Gap Report** below are updated as pushes run — that's how we
keep the picture honest.

## Coverage Ledger

Current statement coverage across the repo. Update the row for a
package whenever its coverage changes, and include the doc change in
the same commit as the tests.

| Package                    | Coverage | Last run    | Notes                     |
|----------------------------|---------:|-------------|---------------------------|
| `internal/storage`         | 83.1%    | 2026-04-15  | —                         |
| `internal/combat`          | 67.6%    | 2026-04-15  | —                         |
| `internal/assets`          | 30.6%    | 2026-04-15  | —                         |
| `internal/game`            | 90.6%    | 2026-04-18  | Phases 1-3 complete            |
| `internal/commands`        | 27.9%    | 2026-04-15  | —                         |
| `cmd/treetool`             |  0.0%    | 2026-04-15  | —                         |
| `cmd/mud`                  | no tests | 2026-04-15  | —                         |
| `cmd/mud/command`          | no tests | 2026-04-15  | —                         |
| `internal`                 | no tests | 2026-04-15  | —                         |
| `internal/display`         | no tests | 2026-04-15  | —                         |
| `internal/gametest`        | no tests | 2026-04-15  | test helper package       |
| `internal/listener`        | no tests | 2026-04-15  | network layer             |
| `internal/messaging`       | no tests | 2026-04-15  | NATS layer                |
| `internal/player`          | no tests | 2026-04-15  | login/session flows       |

Maintenance rules:

- **Running the playbook on a package** → update that row with the new
  percentage, today's date, and a short note (e.g. "Phase 2 complete").
- **Landing a feature that adds or removes tests** → update the affected
  row even if it wasn't a formal playbook run.
- **Row date > 1 month old** → re-run `go test -cover` and refresh the
  number. Drift of more than 5 percentage points is a signal to open a
  targeted playbook run on that package.

To get current numbers:

```bash
go test -coverprofile=/tmp/<pkg>_cov.out ./<package>
go tool cover -func=/tmp/<pkg>_cov.out | tail -1
```

## The Playbook

Each coverage push has:

- **Target package** — the single package under focus.
- **Coverage goal** — typically 70%, 80%, or 90%.
- **Stop points** — phase boundaries where the user can review and bail.

### Core Rule: Hard-to-Test Is a Signal

When a function is hard to test, the first question is *why*. The
answer is usually one of:

1. **Missing interface at the consumer.** The function calls a concrete
   collaborator where an interface belongs (per CLAUDE.md: "interfaces
   should live where they are consumed, not where they are satisfied").
   Tests would need a fake of that interface, but there's nothing to
   implement against. **Action:** add the interface at the consumer
   site, switch the production code to accept the interface, write the
   fake in the test file. This is not a production refactor we avoid —
   it's a seam the codebase was already missing.
2. **Cross-package struct construction.** A test needs to build a
   non-trivial struct from another package to set up the call. Per
   CLAUDE.md, that's also a signal the consumer should be taking an
   interface, not the concrete struct.
3. **Global state or package-level side effects.** The function reads
   from a package-level variable or calls a global (`time.Now`,
   `rand.IntN`, a package-level singleton). Small globals are handled
   by the clock/RNG seams in Phase 2. Larger globals become interface
   gap items.
4. **Deep dependency chains.** The function is a single line that
   triggers a graph of collaborators. Handled by Phase 3 fixture
   builders.

Treat every Phase 2/3 blocker as a candidate **Interface Gap**. Every
gap gets one of two dispositions:

- **fix-in-flight** — small, obvious seams that land inside the phase
  that discovered them. The default preference.
- **deferred with explicit sign-off** — the user has looked at it and
  agreed that rearchitecting right now isn't worth the cost. Rationale
  and sign-off date go on the Interface Gap Report; blocked tests land
  in a follow-up plan tied to the eventual refactor.

There is no silent "accepted-lossy" escape hatch. If a function is hard
to test and we can't think of a clean refactor, that's a conversation,
not a default.

### Core Rule: Test Intent, Not Implementation

Tests must assert what a function **should** do, not what the current
code happens to do. If the two diverge, we've found a bug — flag it,
don't codify it.

In practice:

- **Derive expectations from the function's contract** — its name, doc
  comment, type signature, and how callers use it. Not from stepping
  through the current body and writing asserts that match every branch.
- **When intent is ambiguous, ask.** If a doc comment is missing,
  vague, or contradicts the function name, or if the current code has
  multiple plausible interpretations, stop and ask. Don't guess.
- **When the current code looks wrong, surface it.** If a test against
  the *intended* behavior would fail against the current code, do not
  weaken the test to match. Report the discrepancy — it's either a bug
  to fix or a misunderstanding to clarify.
- **Tests should survive refactors that preserve behavior.** If a test
  locks in an implementation detail (private field layout, exact call
  ordering where order doesn't matter, exact error message strings),
  it will churn on every cleanup. Assert on the observable contract.

When in doubt, ask rather than assume. A handful of clarifying
questions up front is cheaper than a suite of tests that cement the
wrong behavior.

### Phase 0 — Audit existing tests (no coverage delta)

Before adding new tests, walk every existing `*_test.go` in the target
package and flag issues. We don't want to pile new tests on a shaky
foundation.

Review criteria:

1. **Table-driven style** (per CLAUDE.md) — uses `map[string]struct{...}`
   with descriptive case names. Loop via `for name, tc := range tests`.
   Flag single-case `t.Run` blocks or non-table tests that should
   convert.
2. **Behavior, not implementation** — does the test assert visible
   state changes and return values, or private fields, exact call
   ordering that doesn't matter, and exact error strings when a type
   check would do? Flag implementation-leaking assertions.
3. **Intent-driven, not code-mirroring** — does the test assert what
   the function *should* do (from its name, doc comment, and callers),
   or does it simply mirror the current body's branches? Flag any test
   that would pass regardless of whether the function has a bug. If
   existing tests codify behavior that contradicts the function's
   apparent intent, surface it for user review.
4. **Helper reuse** — tests use package-level helpers instead of
   open-coding setup. Flag duplication a helper would absorb.
5. **Case naming** — keys describe the *behavior under test* ("solo
   caster heals only self"), not the *input* ("nil target"). Flag
   cryptic or numeric-suffixed names.
6. **Assertion specificity** — a failing test should tell you *what*
   is wrong. Flag bare `if err != nil` with no message, or
   `if got != want` without printing both.
7. **Setup clutter** — long fixture blocks obscuring the case. Flag
   where `setup func(f *fixture)` on the table row or a shared builder
   helps.
8. **Dead tests** — tests that always pass regardless of the code under
   test. Flag for deletion.
9. **Coverage-only tests** — tests that move a number without asserting
   meaningful behavior. Flag for rewrite.
10. **Forbidden patterns** — type assertions in tests when an interface
    would do. Flag and replace.
11. **Naming conventions** — `Id` not `ID`; comments describe
    behavior, not "satisfies interface".

**Deliverable:** a list of concrete fix-ups grouped by file. Mechanical
cleanups (rename case keys, extract helpers, tighten assertions) get
applied in place. Anything that would meaningfully change what a test
asserts is called out for user approval first.

**Exit:** every existing test file reviewed, mechanical fix-ups
applied. Coverage number roughly unchanged; test *quality* is now the
baseline for Phases 1–3.

### Phase 1 — Easy wins (no new infrastructure)

Add tests for pure logic and state-mutation functions that need no
mocks or new helpers beyond what already exists.

Qualifies:

- Simple accessors and setters with branching worth verifying.
- Pure math, formatting, parsing, lookup functions.
- State-mutation on already-constructed instances (add/remove,
  set/clear, count/find).
- Helpers on a single struct that don't reach collaborators.

Does NOT qualify (defer to Phase 2 or 3):

- Anything needing a mock commander, clock, RNG, storer, or publisher.
- Anything that walks full world state or ticks the game loop.
- Anything that sends on a channel a fake reader must drain.

**Rule:** no new interfaces, no production refactors, no new mocks.
Extend existing helpers only when the extension is mechanical.

**Exit:** every qualifying function is covered. Record the delta and
the list of what's now green vs still 0%.

### Phase 2 — Seams and small mocks

Tackle functions testable once a small in-package test double exists.
Add doubles to `helpers_test.go`. Typical doubles:

- **Fake commander / handler** — records ability/command invocations.
- **Fake clock** — function variable replacing `time.Now` at package
  level (e.g. `var now = time.Now`).
- **Fake RNG** — function variable replacing `rand.IntN` at package
  level.
- **Fake publisher / subscriber** — records emitted events.
- **Fake storer** — in-memory `storage.Storer[T]`.

**Production concession:** clock and RNG seams are the only production
changes allowed in Phase 2, and only if genuinely needed. They're
one-line `var` declarations with no behavior change. Anything larger
goes on the Interface Gap Report.

Qualifies:

- Tick/update functions calling into a commander or handler.
- Functions branching on `time.Now()`.
- Functions using package-level RNG.
- Functions emitting via a publisher interface.

**Exit:** all tick and seam-testable functions covered. Record delta,
list new mocks, list items pushed to Phase 3.

### Phase 3 — Heavy fixtures

Functions with deep dependencies on world state, storage persistence,
or network layers. Approach: build the minimum possible fixture.

Typical heavy fixtures:

- `newTestWorld` — real `WorldState` with one zone, one or two rooms,
  a fake subscriber, and a fake storer.
- `newTestSession` — simulates a login/message pump without real TCP.

Qualifies:

- World-level operations (`NewWorldState`, `AddPlayer`, `Tick`).
- Persistence round-trips.
- Cross-package integration at the level of a single package's public
  API.

**Remaining uncovered code** by end of Phase 3 — each handled
explicitly, not silently accepted:

- **Structural formatters** (a 100-line stat-sheet formatter). Test
  structural shape, not exact text. If even structural tests are
  brittle, surface as an interface gap ("formatter should be broken
  into composable pieces").
- **Defensive branches** (double nil guards, unreachable error paths).
  If they really can't fail, they're dead code — propose deletion.
- **Anything else** — Interface Gap Report with a proposed refactor;
  user decides fix-in-flight vs deferred.

**Exit:** coverage goal hit, or a short list of explicit user-
signed-off deferrals. No silent gaps.

## Per-Phase Reporting

At the end of every phase, report:

1. **Coverage delta** — before → after, overall and per-file if useful.
2. **Files touched** — new tests, extended tests, production seams.
3. **Blockers surfaced** — anything pushed to a later phase and why.
4. **0%-coverage list** — remaining functions at 0% and their
   disposition.
5. **Interface Gap Report entries** — any new gaps surfaced this phase.
6. **Continue / stop / revise recommendation** — user decides.
7. **Ledger update** — edit the row for the target package and commit
   alongside the test changes.

## Verification (every phase)

```bash
go build ./...                                       # whole repo compiles
go test ./<target>/... -count=1                      # target tests pass
go test -coverprofile=/tmp/<pkg>_cov.out ./<target>  # get coverage
go tool cover -func=/tmp/<pkg>_cov.out | tail -1     # confirm total
go test ./... -count=1                               # full suite green
go tool cover -func=/tmp/<pkg>_cov.out \
  | awk '$3 == "0.0%"'                               # review 0% items
```

## Global Out of Scope

- **Large production refactors inside a coverage phase.** Small seams
  (clock/RNG vars, new consumer-side interfaces with obvious shape)
  are in scope when they unblock tests. Larger refactors go on the
  Interface Gap Report for a dedicated follow-up.
- **Integration / end-to-end tests** spanning multiple packages. Each
  push stays inside one target package.
- **100% coverage.** 90% is the usual target; the last mile pushes
  tests toward implementation coupling. Gaps above 90% get the same
  fix-in-flight / deferred / wontfix treatment.
- **Rewriting tests that pass Phase 0 criteria** just to match a new
  aesthetic — mechanical cleanups only.

## Interface Gap Report

Populated as coverage pushes surface places where a missing interface
blocks testing. Each entry:

- **Site** — file and function.
- **What the code wants** — the concrete collaborator it reaches for.
- **Proposed interface** — the minimal surface the consumer actually
  needs (methods only).
- **Blocked tests** — which tests landed behind this gap.
- **Disposition** — `fix-in-flight`, `deferred`, or `wontfix`.

Format:

```
### <file>:<function>
- Wants: <InterfaceName> { <method signatures> }
- Currently reaches for: <concrete call chain>
- Blocked tests: <list>
- Disposition: <fix-in-flight | deferred (sign-off date, reason, follow-up) | wontfix (reason)>
```

Rules:

1. **Never silently work around a gap.** A weakened or skipped test
   must also land on this list.
2. **Every `deferred` entry has a user sign-off date and reason.**
   "We decided not to rearchitect" is a decision, not a default — it
   requires a conversation, and the outcome lands here.
3. **`fix-in-flight` is the default preference.** Only escalate to
   `deferred` when the scope would blow up the current plan.

### (no entries yet)

New entries get added below this line as coverage pushes surface them.

---

## First Application: `internal/game` @ ~90%

The playbook's first run. Exploration snapshot from 2026-04-15:

**Already well covered:**

- `perkcache.go` — 100%
- `threat.go` — 100%
- `experience.go` — ~90%

**Zero-test files:**

- `world.go`, `zone.go`, `driver.go`, `dictionary.go`, `death.go`,
  `publisher.go`; major functions in `group.go` also uncovered.

**Thinly covered files:**

- `actor.go` (~30%), `character.go` (~27%), `mobile.go` (~25%),
  `room.go` (~17%), `object.go` (~20%), `inventory.go` /
  `equipment.go` (~20%).

**Existing test helpers:**

- `internal/game/helpers_test.go` — `newTestObj`
- `internal/game/actor_test.go` — `newTestCI`, `newTestMI`
- `internal/gametest/actormock.go` — `BaseActor`

### Phase 0 — Audit

Files to review:

- `actor_test.go`, `character_test.go`, `mobile_test.go`,
  `room_test.go`, `object_test.go`, `inventory_test.go`,
  `equipment_test.go`, `threat_test.go`, `perkcache_test.go`,
  `experience_test.go`, `helpers_test.go`, plus any others present.

### Phase 1 — Easy wins (~30% → ~55%)

New test files:

- `group_test.go` — `GroupLeader`, `WalkGroup`, `AreAllies`,
  `IsPlayerSide`, `GroupPublishTarget.ForEachPlayer`. Solo, flat
  group, nested sub-group, mixed player+mob group, non-grouped
  follower.
- `dictionary_test.go` — lookup hit/miss.

Extended test files:

- `actor_test.go` — accessors (`Room`, `IsAlive`, `Level`,
  `Inventory`, `Equipment`, `ThreatEnemies`, `IsCharacter`),
  resource ops (overheal clamp, drain), grant add/remove,
  `HasGrant`/`GrantArgs`.
- `character_test.go` — `Name`, `Asset`, `CombatTargetId`,
  `EffectiveStats` (base + modifier), `Gain` XP path.
- `mobile_test.go` — creation, perk inheritance, `Drain`, `IsAlive`.
- `room_test.go` — add/remove player/mob/object, `GetMob`,
  `FindMobs`, `FindObject`,
  `ForEachActor`/`ForEachMob`/`ForEachPlayer`, occupant counting.
- `inventory_test.go` / `equipment_test.go` —
  add/remove/find/count/iterate, equip slot collision, unequip to
  inventory.
- `object_test.go` — `newCorpse` drains inventory+equipment into a
  container, decay end-state consistency.

### Phase 2 — Tick systems and mocks (~55% → ~75%)

Helpers in `helpers_test.go`:

- `fakeCommander` — satisfies the commander interface used by
  `ActorInstance.autoUseTick`,
  `MobileInstance.tryWander`/`tryScavenge`.
- `fakeClock` — overrides `now` in `zone.go`.
- `fakeRand` — overrides `randIntN` in `mobile.go`.

Production seams (the only production changes):

- `internal/game/mobile.go` — `var randIntN = rand.IntN`.
- `internal/game/zone.go` — `var now = time.Now`.

Targets:

- `actor_test.go` — `autoUseTick` (ready/cooldown/no-op).
- `character_test.go` — `CharacterInstance.Tick`, `StatSections`
  (structural asserts only).
- `mobile_test.go` — `MobileInstance.Tick`, `tryWander` (exits,
  `stay_zone`, null exits), `tryScavenge`.
- `room_test.go` — `Reset` (respawn on empty, preserve on occupied,
  object respawn, exit wiring), `Describe`.
- `zone_test.go` (new) — `ZoneInstance.Reset` via `fakeClock`.

### Phase 3 — World wiring (~75% → ~90%)

Helpers:

- `fakeSubscriber` — records published events.
- `fakeStorer` — in-memory `storage.Storer[*assets.Character]`.
- `newTestWorld` — stitches a minimal `WorldState`.

New test files:

- `world_test.go` — `NewWorldState`, `AddPlayer`/`RemovePlayer`,
  `Tick`, `GetRoom`/`GetPlayer`/`GetZone`, `ForEachPlayer`.
- `driver_test.go` — start/stop/tick dispatch.
- `publisher_test.go` — only if concrete logic exists beyond
  interfaces.
- `death_test.go` — `processDeath`: mob corpse drop, player
  `OnDeath` routing.

Extended test files:

- `character_test.go` — `SaveCharacter` round-trip via `fakeStorer`.

End-of-phase items for decision (not silently accepted):

- `StatSections` — 118-line formatter. Proposal: structural asserts
  on section count + required labels. If brittle, surface as a gap
  ("formatter should be broken into composable pieces").
- Defensive nil guards / unreachable branches — propose deletion;
  for the rest, surface on the gap report for a decision.
