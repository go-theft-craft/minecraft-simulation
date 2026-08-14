# Simulation physics first subproject design

## Status

The user approved this design on 2026-08-14. Implementation requires the
matching implementation plan and an explicit request to execute it.

## Purpose

This design details the first implementation subproject named in the parent
simulation design, which the `headless-minecraft` repository stores at
`docs/superpowers/specs/2026-08-13-minecraft-simulation-design.md`.
The parent design establishes the deterministic kernel, the package layout, the
profile contracts, and the clean-room research boundary. This design does not
revise those decisions. It records four decisions the parent design left open:

- How much physics the first subproject delivers
- Where vanilla behavior ground truth comes from
- Where extracted physics constants live
- The order in which the work lands, and how each stage is verified

The scope is entity movement and collision for the player, dropped items, and
arrows, on Java Edition 1.8.9 and Java Edition 26.1.2, consumed by both the
headless client and the server.

## Decisions

### Fidelity

The subproject implements the parent design's first subproject in full. It
covers three entity families across two profiles, with the in-memory runtime,
replay, canonical digests, and shared conformance fixtures. It does not narrow
to client-side movement or to a single profile.

The 26.1.2 profile is not the 1.8.9 profile with version flags. Swimming,
elytra flight, and the 1.13 collision rework are structurally different code
paths. The two player movement implementations are planned as independent work
with a shared collision core beneath them.

### Ground truth

Vanilla behavior is established from three oracles that produce measurements
rather than code. None of them copies decompiled Java into this repository.

The first oracle extracts constants by running the game as a library. A
dumper program on the classpath of a verified server jar reflects over the
block and entity registries and prints values as JSON. The `mcreference`
command already downloads, verifies, and applies names to those jars, so it
owns the new `dump` subcommand.

The second oracle captures entity traces from the network. For dropped items
and arrows the server is authoritative, so spawning one on a vanilla server and
recording its position and velocity packets yields per-tick trajectories. The
existing `proxy` repository owns that capture mode.

The third oracle uses server correction as the player test. The player is
simulated by the client, so no server trace exists for it. A conforming
implementation instead produces no correction packets when it connects to a
vanilla server and executes scripted input.

Java to Go transpilation of vanilla methods is rejected. The obstacle is
licensing rather than difficulty. Transpiler output is a derivative work, the
parent design forbids committing decompiled methods, and generating it locally
at build time would break reproducible builds for consumers of the module.

### Constants home

Extracted constants join the existing pinned dataset pipeline in
`minecraft-protocol`. The dumper writes `source/java/<version>/physics.json`,
the generator renders `generated/java/<package>/physics.go` beside the existing
`collision_shapes.go`, and `manifest.json` records the file with its digest.

The extracted JSON is committed. Continuous integration then verifies generated
output against pinned source without a JDK and without downloading game
artifacts. Only regeneration requires the jar.

The manifest currently records a PrismarineJS upstream path and license for
every file. Physics constants have Mojang provenance instead.
`THIRD_PARTY_NOTICES.md` gains an entry that states this provenance explicitly
rather than leaving the file under the PrismarineJS attribution.

## Extracted constants

The existing datasets already supply part of the required input. Block
collision shapes are generated into `collision_shapes.go` from
`blockCollisionShapes.json`. Entity width and height come from `entities.json`.
Block bounding box, hardness, and resistance come from `blocks.json`.

The dumper supplies what no dataset records:

- Block slipperiness, including ice, packed ice, slime, and soul sand
- Gravity and drag constants per entity family
- Step height and per-entity motion constants
- The trigonometry lookup table

The trigonometry table deserves its own justification. Every rotation in the
game passes through a lookup table of 65536 float entries that the game builds
during class initialization. Rebuilding that table in Go from the same formula
risks last-place divergence, because Go and Java guarantee no shared bit-exact
result for their sine implementations. Dumping the table adds roughly 256 KB of
data and removes the risk entirely.

The dumper reads the server jar rather than the client jar. The server jar
contains the blocks, entities, and registries the dumper needs, and it does not
load the client's native rendering libraries, so the dumper stays headless. The
1.8.9 dumper must call the game's bootstrap routine explicitly before the
registries populate.

## Trace precision

Java Edition 1.8 transmits entity positions as fixed-point integers in units of
one thirty-second of a block, and entity velocity in units of one eight
thousandth of a block per tick. Captured traces therefore verify a trajectory
to roughly 0.03 blocks rather than exactly.

That precision is sufficient for the errors this subproject must catch. A wrong
drag constant, a wrong gravity value, a wrong collision axis order, or a missing
environmental response all diverge far beyond one thirty-second of a block
within a few ticks. Last-place drift is not detectable this way.

If bit-exact item and arrow fixtures become necessary, the upgrade path reuses
the dumper technique. Running the server jar in process, stepping an entity, and
printing raw double values produces exact traces. This design does not build
that harness. It notes that the dumper infrastructure reduces it to a small
addition rather than a new project.

## Repository responsibilities

Four repositories change. The dependency direction from the parent design is
preserved: core simulation packages import no protocol package, and profile
packages import only `minecraft-protocol/data`.

```text
minecraft-reference    mcreference dump --versions 1.8.9,26.1.2 --side server
        |                  writes physics.json
        v
minecraft-protocol     source/java/1.8/physics.json     committed and pinned
        |              generated/java/v1_8/physics.go    generated and verified
        v
minecraft-simulation   geom -> collision -> sim, world, entity
        |              movement, item, projectile
        |              profile/java/v1_8 and profile/java/v26_1
        v
  headless-minecraft (prediction)        server (authoritative)
        ^
  proxy --entity-trace  produces item and arrow golden traces
```

## Effort and difficulty

Sizes are rough implementation lines. Tests approximately double each entry.
Difficulty reflects subtlety rather than volume.

| Component | Repository | Impl | Difficulty |
| --- | --- | ---: | --- |
| `physics.json` generation and manifest entry | protocol | 300 | Low |
| `mcreference dump` | reference | 800 | Medium |
| `proxy --entity-trace` | proxy | 400 | Low to medium |
| `geom` | simulation | 800 | Low |
| `sim` contracts | simulation | 1200 | Medium |
| `world` views | simulation | 500 | Medium |
| `entity` bodies | simulation | 400 | Low |
| `collision` | simulation | 1000 | High |
| `movement` and v1_8 player | simulation | 900 | High |
| v26_1 player | simulation | 1200 | Very high |
| Dropped item, both profiles | simulation | 300 | Medium |
| Arrow, both profiles | simulation | 500 | Medium to high |
| `runtime` | simulation | 500 | Medium |
| `replay` and digests | simulation | 400 | Medium |
| `mctest` fixture runner | simulation | 600 | Medium |

The total is approximately 9,800 implementation lines, 9,000 test lines, and
1,500 lines of fixtures, across four repositories.

The distribution matters more than the total. Roughly sixty percent is
mechanical work of low or medium difficulty. Risk concentrates in four entries:
`collision`, the two player movement profiles, and the dumper.

The largest single risk is the 26.1.2 player movement. It shares little
structure with 1.8.9, and the version is recent enough that its behavior can
still change. Schedule slip is most likely to appear there.

## Milestones

These milestones subdivide **M8** in the cross-repository master plan, which
`headless-minecraft` stores at `MASTER_PLAN.md`. They use the `M8.x` namespace
so that a milestone identifier means the same thing in both documents.

Milestones are ordered by risk retired rather than by layer. Each states one
exit criterion.

### M8.1: ground-truth pipeline

Deliver `mcreference dump`, `physics.json` for 1.8.9, the generated
`physics.go`, the manifest entry, and the notices entry.

Exit: `v1_8.Physics()` returns slipperiness, gravity, drag, step heights, and
the trigonometry table, and `task generate:check` passes without a JDK.

This milestone is first because it is independent, low risk, and validates the
riskiest tooling assumption, which is that the server jar reflects cleanly in a
headless process. A failure surfaces before any physics depends on it.

### M8.2: geometry and collision core

Deliver `geom`, `collision`, and a minimal world view. No entity types and no
profiles.

Exit: property tests establish that swept motion never tunnels through a solid
block, that step-up respects its bound, and that zero motion is a fixed point.

This is the highest-difficulty component with no dependencies, and everything
else rests on it.

### M8.3: kernel contracts

Deliver `sim`, `world`, `entity`, `runtime`, and the in-memory store. A tick
that performs no work, deterministically, with change sets and revision checks.

Exit: an empty tick produces a stable digest and a change set that a store at a
newer revision rejects.

This follows M8.2 deliberately. What collision returns, including contacts and
step results, should shape the tick contracts rather than the reverse.

### M8.4: v1_8 player

Deliver `movement` and `profile/java/v1_8` for the player only.

Exit: scripted walking, sprinting, jumping, and sneaking against a vanilla
1.8.9 server produce no correction packets.

This is the milestone to optimize the schedule around. It is the first point at
which an entity moves, and the first vertical slice through all four
repositories.

### M8.5: traces, items, and arrows

Deliver `proxy --entity-trace`, then dropped item and arrow rules on the 1.8.9
profile.

Exit: captured traces replay within one thirty-second of a block.

Capture lands here rather than in M8.1 because items and arrows are the only
subjects it verifies.

### M8.6: replay and determinism

Deliver canonical encoding, result digests, and the cross-platform matrix.

Exit: an identical digest on Linux, macOS, and Windows, on amd64 and arm64.

### M8.7: v26_1 profile

Extend the dumper, add `physics.json` for 26.1.2, and implement the player,
dropped item, and arrow rules for that profile.

Exit: the same conformance suite passes on 26.1.2.

This lands last, when all scaffolding is proven, because it is the work most
likely to slip.

### M8.8: consumer integration

Deliver client prediction and reconciliation in `headless-minecraft`, and
authoritative simulation and input validation in `server`.

## Consumer readiness

`headless-minecraft` cannot consume this work when M8.4 completes. Its own
roadmap places it at P0. It cannot connect to a server, and prediction requires
its P1 lifecycle and authentication work and its P2 observed world state.

M8.4 nonetheless requires a client that can connect to a vanilla server. The
mitigation is a test-harness client inside the simulation test suite, built on
the generated 1.8 codecs that `minecraft-protocol` already provides. A vanilla
server in offline mode requires no encryption, and setting
`network-compression-threshold` to `-1` in `server.properties` avoids the
compression gap that the protocol README documents. The remaining surface is
handshake, login start, login success, and play, which the existing codecs
cover.

The `server` repository is a convenient counterparty for smoke-testing the
loop, but it is not vanilla and therefore carries no authority as a conformance
oracle. Only a verified Mojang jar closes M8.4.

## Testing

Five tiers correspond to the oracles. The division that governs repository
policy is which tiers can run in public continuous integration.

| Tier | Subject | Oracle | Public CI |
| --- | --- | --- | --- |
| 1 | Unit and property tests for geometry, collision, and numeric conversion | None required | Yes |
| 2 | Generated constants match pinned source | `generate:check` | Yes, without a JDK |
| 3 | Golden trace conformance for items and arrows | Captured traces | Yes, fixtures committed |
| 4 | Player conformance | Vanilla server | No, gated |
| 5 | Determinism and cross-platform digests | Self | Yes, matrix |

Tier 4 requires a game jar, so it cannot run in public continuous integration
by default. It is tagged, gated, and run locally and on a scheduled job with
credentials. Every other tier passes on each push with no game artifacts
present on the runner. That separation is what keeps the repositories
publishable.

Tier 1 carries more weight here than it usually would, because tier 4 runs
infrequently. Property tests are the fast feedback loop for the
highest-difficulty component: swept motion never tunnels, resolution is
order-independent wherever vanilla behavior is order-independent, and a body at
rest remains at rest.

Every fixture records its provenance. A fixture states whether it was captured
or authored, the game version and jar digest it came from, and the tool version
that produced it. Captured and authored fixtures fail for different reasons. A
captured fixture turning red means the implementation drifted. An authored
fixture turning red may mean the fixture itself was wrong. That distinction
cannot be recovered later if it was not recorded at the time.

Audit mode from the parent design is inexpensive and included. Running a tick
twice from identical input and comparing digests catches map iteration order,
wall-clock reads, and result-affecting goroutines. Those sources otherwise
surface as an unreproducible cross-platform failure in tier 5.

## Acceptance criteria

The subproject is complete when all of these statements are true:

- The dumper produces `physics.json` for both versions from verified server
  jars, and continuous integration verifies the generated Go without a JDK.
- Core simulation packages import no protocol package, and profile packages
  import only `minecraft-protocol/data`.
- The player, dropped item, and arrow families simulate on both the 1.8.9 and
  26.1.2 profiles.
- Scripted player input produces no correction packets from a vanilla server on
  either version.
- Captured item and arrow traces replay within one thirty-second of a block.
- A complete input produces one canonical digest on every supported operating
  system and architecture.
- No decompiled method, obfuscated class, or game asset appears in any public
  repository.
- Every committed fixture records its provenance, version, and generating tool.

## Deferred work

This design does not cover fluid propagation, explosions, primed TNT, vehicles,
redstone, pistons, random block ticks, mob artificial intelligence, or combat.
The parent design orders that work in its later subprojects. Each requires its
own implementation plan and conformance fixtures.

Bit-exact item and arrow fixtures are deferred. The captured traces in tier 3
are sufficient for this subproject, and the in-process harness that would
supersede them is a small addition once the dumper exists.
