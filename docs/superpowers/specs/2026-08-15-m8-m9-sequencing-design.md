# M8 and M9 sequencing design

## Status

Approved in outline on 2026-08-15. This document fixes the order of the
remaining M8 stages and subdivides M9. Each milestone still requires its own
implementation plan and an explicit request to execute it.

## Purpose

The parent
[physics-first subproject design](2026-08-14-simulation-physics-first-subproject-design.md)
subdivides M8 into stages and gives each a one-line exit criterion. It was
written before M8.1 ran, and three of its assumptions have since proved wrong.
This document records what changed, states each milestone's real dependency
rather than the coarse one in the master plan, subdivides M9, and freezes the
interfaces that later milestones are written against.

It does not restate the parent design's kernel model, package layout, or
clean-room research boundary. Those stand.

## What M8.1 changed

Three corrections, each discovered by running the work rather than planning it:

**The trace-capture oracle has no owner.** The parent design assigns
`--entity-trace` capture to the existing legacy proxy, budgeted at 400 lines.
That proxy speaks a different protocol family entirely: one-byte packet
identifiers, no VarInt framing, UCS-2 strings, and encryption on one direction
only. It cannot capture Java Edition 1.8.9 entity traces without being
rewritten into a different program. Capture needs a new home, and it is not a
400-line task.

**Motion constants are float32 widened to double.** Eleven of the twelve entity
motion constants are `float` literals that Java widens where they are applied,
so the values are `0.9800000190734863` and its siblings, not the round
decimals. On the ground the horizontal drag is `slipperiness * 0.91F` for
players and `slipperiness * 0.98F` for items, and that product is computed in
`float32` before widening. A kernel that computes it in `float64` will not
match vanilla bit for bit.

**Measured data is version-specific and therefore optional.** Only versions
somebody has run the extraction tool against have physics constants. The
manifest schema, the render plan, and the generated package all treat extracted
data as optional, so a source tree without it still generates.

## Decisions

### Dependencies are stated per milestone

The master plan says everything in M8 after M8.1 is blocked on M4 and M7. That
is too coarse: M8.2 needs neither, and its implementation plan is already
written and committed. Each milestone below states what it actually requires.
Simulation work proceeds in parallel with protocol work whenever its real
dependency allows, because the two touch different repositories.

### Milestone identifiers never change meaning

M8.5's content moves to M9, and its identifier is retired rather than reused.
M8.6, M8.7, and M8.8 keep their numbers. The parent design states that an
`M8.x` identifier means the same thing in every document; renumbering would
silently change what "M8.6" refers to in the master plan, in committed commit
messages, and in this session's history.

### M8 is a player-only slice

Dropped items and arrows leave M8. M8 delivers one deterministic movement slice
for the player across two profiles, which is what its master-plan row claims.
Items and arrows depend on captured traces for their exit criterion, and
capture now depends on infrastructure that does not exist, so keeping them in
M8 would block the whole milestone behind a new repository.

### M8.4 is gated on fixtures, not on a live server

The parent design gates the v1_8 player on a vanilla server sending zero
correction packets. That needs a working headless client (M6) and observed
world state (M7). The movement code itself needs neither. M8.4 therefore
delivers movement verified against recorded fixtures and hand-derived cases,
and the zero-corrections test becomes an explicit gate in M8.8, where the
client adapter exists anyway.

This is the same split M8.1 used for its transcribed constants: build against
what can be checked now, and gate on the authoritative oracle when it exists.

### Capture gets its own repository

Entity-trace capture goes into a new repository speaking Java protocol 47
through `minecraft-protocol`, not into the legacy proxy and not into the
headless client. The legacy proxy cannot speak the protocol. The headless
client could, once M8.8 lands, but capture is a proxy-shaped problem — sitting
between a real client and a real server and recording both directions — and
folding it into the client would couple the oracle to the thing it verifies.

The cost is honest: this is a new repository, not the 400-line subcommand the
parent design budgeted.

## Milestones

Each states its real dependency and one exit criterion.

| Stage | Deliverable | Depends on | Gate |
| --- | --- | --- | --- |
| M8.2 | `geom`, `collision`, minimal world view | — | no tunneling, step-up bounded, zero motion is a fixed point |
| M8.3 | `sim`, `world`, `entity`, `runtime`, in-memory store | M8.2 | an empty tick has a stable digest; a stale store rejects the change set |
| M8.4 | `movement`, `profile/java/v1_8` for the player | M8.3 | fixture conformance for walk, sprint, jump, sneak, fall, and collide |
| M8.5 | *retired* — moved to M9.1 and M9.2 | — | — |
| M8.6 | canonical encoding, result digests, cross-platform matrix | M8.3 for encoding; M8.4 for the matrix | identical digest on Linux, macOS, and Windows, on amd64 and arm64 |
| M8.7 | `profile/java/v26_1` for the player | M8.4, M4 | the M8.4 fixture suite passes on 26.1.2 |
| M8.8 | client prediction and server-authoritative integration | M8.4, M6, M7 | both adapters run one kernel; scripted input draws zero corrections from vanilla 1.8.9 |

The critical path is M8.2 → M8.3 → M8.4 → M8.8. M8.6's canonical encoding can
land alongside M8.3; only its cross-platform matrix waits for M8.4 to produce
something worth hashing. M8.7 waits on M4 for 26.1 data.

M9 follows M8.8 and subdivides by mechanic, because the packages and the
conformance fixtures are already organized that way and each mechanic is
independently verifiable.

| Stage | Deliverable | Gate |
| --- | --- | --- |
| M9.1 | entity-trace capture in a new protocol-47 proxy repository | a captured trace replays deterministically from its recording |
| M9.2 | dropped item and arrow rules, both profiles | captured traces replay within one thirty-second of a block |
| M9.3 | movement scenarios | correction, teleport, and disconnect mid-action behave as vanilla |
| M9.4 | digging and block breaking | break times match vanilla for tool, block, and effect combinations |
| M9.5 | building and placement | placement legality and resulting block state match vanilla |
| M9.6 | attack, damage, knockback | reach validation, cooldown timing, damage, and death match vanilla |
| M9.7 | containers and inventory | window open and close, slot synchronization, and rejected moves match vanilla |
| M9.8 | crafting | recipe matching and result stacks match vanilla, including the 2x2 grid |

M9's stages get one-paragraph resolution here. Each earns a detailed plan when
it becomes next, for the same reason M8.3's contracts are not specified in this
document: the information needed to write them does not exist yet.

## Frozen interfaces

These are the seams later milestones are written against. Everything behind
them is each milestone's own decision.

### The kernel

```go
type Kernel interface {
	Step(ctx context.Context, input TickInput) (TickResult, error)
}
```

`TickInput` carries the profile, the snapshot revision and simulation tick,
immutable views, a simulation scope, ordered commands, explicit serialized
random state, and deterministic work limits. `TickResult` carries the input
revision and tick, an ordered change set, ordered domain and presentation
events, command outcomes, updated random state, the data dependencies read,
a completeness result, and a canonical digest.

M8.3 fixes the field names and types. What is frozen here is that `Step` is
the only entry point, that it is pure with respect to its input, and that a
result carries its own digest rather than the caller computing one.

### Incompleteness

A view distinguishes known-absent from unknown. When a rule in scope needs
unknown data, the tick is incomplete: the result names the missing regions and
carries no applicable change set and no events.

`collision.Resolve` already implements this shape — it returns the cells it
could not resolve and applies no motion. M8.3 must adopt the same rule at the
tick level rather than inventing a second convention.

```go
type Completeness struct {
	Complete bool
	Missing  []Dependency
}
```

### Change sets and revisions

A change set records the revision it was computed against. A store applies it
only if the store still holds that revision. This is what lets a client apply
predicted changes to a forked snapshot and discard them after a correction.

```go
type ChangeSet struct {
	BaseRevision Revision
	Ops          []Op
}
```

A change set is fully applicable or not applicable. There is no partial apply.

### Profiles are protocol-free to the kernel

The kernel sees a profile through an interface that names no protocol type.
The concrete `profile/java/v1_8` package imports `minecraft-protocol/data` and
adapts it; `sim`, `world`, `entity`, and `collision` import no protocol package
at all.

```go
type Profile interface {
	ID() string
	Slipperiness(block BlockRef) float64
	Motion(family EntityFamily) MotionConstants
}
```

`BlockRef`, `EntityFamily`, and `MotionConstants` are named here but defined
later: M8.3 fixes the first two with the entity and world contracts, and M8.4
fixes the third with the movement rules. What is frozen is that the kernel
reaches physics through a profile, never through a protocol package.

This is the seam that keeps M8.1's generated `data.Physics` from leaking into
the kernel. It also means the float32 rule above belongs to the profile: the
profile computes `slipperiness * 0.91F` in `float32` and hands the kernel a
widened `float64`.

### What is deliberately left open

Every internal type and algorithm; the tick phase list and its order, which
each profile owns; the storage interface behind `runtime`; the fixture file
format; and the digest algorithm. M8.3 and M8.6 decide those.

## Risks

**The capture repository does not exist.** M9.1 is a new repository, and the
parent design's estimate for it assumed an existing proxy could be extended.
Re-estimate before scheduling M9.

**The determinism matrix needs runners.** M8.6 gates on identical digests
across three operating systems and two architectures. Whether that CI capacity
exists is unverified, and the gate is meaningless without it. Confirm before
M8.6 starts, not during.

**M4 and M5 are in flight in `minecraft-protocol`.** M8.7 depends on M4's 26.1
data. Simulation work stays in `minecraft-simulation` to avoid contending for
the same files.

**Bit-exactness is unproven past M8.1.** The float32 finding says vanilla's
arithmetic is reproducible in principle, but nothing has yet demonstrated a
matching trajectory over many ticks. M8.4's fixture gate is the first real
test, and it is a weaker signal than the live gate it replaces. If M8.4 passes
fixtures but M8.8 draws corrections, the fault is likelier in the fixtures than
in the kernel.

## Definition of done

This document is complete when a reader can answer, for any remaining M8 or M9
stage: what it delivers, what must exist first, how it is judged finished, and
which interfaces it may not change. It is superseded stage by stage as each
implementation plan is written.
