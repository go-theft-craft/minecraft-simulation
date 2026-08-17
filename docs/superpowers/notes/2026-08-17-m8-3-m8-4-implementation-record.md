# M8.3 and M8.4: what was implemented, what diverged, what the data said

Written 2026-08-17, at the point M8.4 Task 7 landed and Task 8 had not started.
It records the state of the work, every deviation from the two plans, and the
findings that came out of measurement rather than reading. It is not a plan: the
plans are `2026-08-17-m8-3-kernel-contracts.md` and
`2026-08-17-m8-4-v1-8-player-movement.md`, and this note says where the code
differs from them and why.

## State

**M8.3 is complete.** All eleven tasks, eleven commits, `9a572e6`..`e545ad5`.
Both of its exit criteria are tested: an empty tick digests identically across
runs and is pinned in `sim/kernel_test.go`, and a change set based at revision N
is refused by a store at revision N+1 with nothing written
(`runtime/runner_test.go`).

**M8.4 is at Task 7 of 10.** `movement` holds the eleven rules of the 1.8.9 land
tick and `profile/java/v1_8` supplies the constants, the sine table, the block
table, and the phase order. Commits `d7add26`, `286349a`, `16ffc19`, `e8b96ff`,
`d8af4c0`, `844557e`, `3029754`.

Not started: **Task 8**, the movement oracle, which is the milestone's real gate;
**Task 9**, fixtures generated from the game plus the `mctest` replay runner; and
**Task 10**, the milestone record. Until Task 8 runs, every constant and every
order in the tick is *asserted* rather than checked against the game. That is the
gap this note exists to keep visible.

`devbox run -- task verify` passes, M8.2's oracle included.

**Adjacent work, in other repositories.** M8.8's Task 2 (the `adapter` seam) is in
this module at `8f7ee51`. M8.8's Task 1 (the client's outbound action path) is in
`headless-minecraft` at `e0a65d6`, and has since been extended there with a
respawn action by other work. M8.8's Tasks 3 to 6 remain blocked on M8.4.

## Findings

Four things the code told us that no plan said. The first two came from asking
the dataset, the third from running the tick, the fourth from a packet capture.

### The dataset stores block slipperiness unwidened, and step height widened

`physics.json` records a block's slipperiness as the round decimal — `0.6`,
`0.98` — while the game's field is a `float`, so the value the game computes with
is `float32(0.6)`, which widens back to `0.6000000238418579`. The same dataset
records an entity's step height *already* widened, which is the form M8.2's
oracle established.

So the two are stored in opposite conventions, and narrowing at the block table's
boundary is not a lossy convenience: it is what recovers the width the game uses.
`profile/java/v1_8/blocks_test.go` pins the asymmetry, so a later dataset that
starts storing widened slipperiness is noticed there rather than as a trajectory
that drifts.

### Ground acceleration is almost exactly one, by construction

`0.16277136 / friction³` with the default ground friction of `0.6 × 0.91` gives
`0.9999998`. The game's numerator is the cube of that friction, so an ordinary
block accelerates a body at its movement-speed attribute and every other surface
is expressed relative to it.

Worth recording because the first pin written for this was `0.09807128`, which is
the *speed* — the attribute times the acceleration — and not the acceleration. The
formula was right and the expectation was wrong by an order of magnitude, which a
pinned literal would have enshrined either way.

### A standing player is not a static state

Gravity is applied after the move, so a body handed zero motion is airborne for
its first tick and is clamped by the floor on every tick after it. A standing
vanilla player falls about a sixteenth of a block and is stopped, twenty times a
second, and `OnGround` follows from the vertical clamp rather than from resting
contact.

The first version of `TestAStandingPlayerStaysOnTheFloor` asserted `OnGround`
after one tick and failed. The rule was right; the test was describing a game that
does not exist. It now runs five ticks and asserts the settled behaviour.

### The 1.8.9 movement packet cadence, measured

Recorded in `headless-minecraft`'s `version/action.go` and repeated here because
it is a fact about the game rather than about that module. Derived from a capture
of an unmodified 1.8.9 client against an unmodified offline-mode server, 3,703
serverbound movement packets from one five-minute session:

```
moved   = squared distance from the last reported position > 9.0e-4,
          or twenty ticks have passed since one was reported
rotated = the yaw or the pitch differs from the last reported, exactly

moved and rotated -> position_look    moved   -> position
rotated           -> look             neither -> flying
```

The rule agrees with the capture on 3,700 of 3,702 transitions. The two it misses
are at the login boundary. The forced update lands on the twenty-first packet
after the last one that carried a position, which is what a counter incremented
before its comparison and reset on any position-carrying packet produces: pinning
it at twenty instead scored 95.7%, and the 150 disagreements were all the same
events mispredicted one packet early.

This matters to M8.4 because it removes cadence from the list of things a M8.8
correction could mean. It does not remove *trajectory* — that is Task 8's job.

## Deviations from the plans

Recorded because a later reader will otherwise take the plans as a description of
the code. Each is in a commit message too.

### M8.3

1. **`sim` imports `movement`, and `Locomotion` lives there.** The M8.4 plan
   offered `entity` as the alternative home. `movement` won because the rules
   that read the state live there and `entity` is deliberately geometry alone.
   The plan permits either and asks for the choice to be recorded.
2. **`Runner.Step` takes the tick's commands.** The plan's `Step(ctx)` left no way
   for a per-connection command queue to reach the tick, which is exactly what
   M8.8's server driver needs.
3. **`TickState.Commands()` exists.** The plan's accessor list omitted it, and a
   phase cannot answer a command it cannot read.
4. **`ProfileID.validate` landed with Task 1 rather than Task 9**, since the
   linter rejects an unused unexported function.
5. **The empty-tick digest needed no re-pinning when locomotion was added.** The
   plan expected one. The locomotion encoding sits inside the operation loop, and
   an empty tick has no operations, so its bytes did not change. `tagLocomotion`
   was appended to the tag list, never inserted.
6. **`world.Blocks` and `movement.Bodies` gained `Clone`**, which `runtime.Memory`
   needs to fork a snapshot.

### M8.4

1. **`movement.Input` carries an `Entity`.** The plan's `Input` named no subject,
   which leaves a tick simulating several bodies unable to tell whose intent it
   holds.
2. **`Phases()` returns a fresh list per call.** The phases of one list share a
   per-tick scratch structure, so a list belongs to the one kernel that took it.
   Two kernels built from one profile therefore keep their own, and a test
   interleaves three of them and requires identical digests.
3. **`New` takes `*data.Set`, not a `data.Bundle`.** The plan said to adjust to
   whatever `minecraft-protocol` really exposes.
4. **The sine table comes from the dataset.** `data.Physics.SinTable` already
   carries all 65,536 entries, so nothing needed dumping and no Mojang asset is
   committed here.
5. **Soul sand's slipperiness is the default.** The plan's Task 6 expected it to
   differ; it slows a player through a different mechanism, which this milestone
   does not implement. The test asserts the default and says why.
6. **The input-decay phase applies the sneak factor before the 0.98 decay**, and
   the jump-countdown phase also adopts the tick's input flags and facing, because
   every phase after it reads them. Both orderings are Task 8's to confirm.

## What Task 8 has to settle

The three places where the implementation is a considered reading rather than a
measurement:

- **Gravity and the two drags run after the move**, in that order.
- **Sneak scaling precedes the per-tick input decay.**
- **`Cos` adds its quarter turn as a float before the truncation**, not to the
  truncated index. The two agree for a positive angle and disagree for a negative
  one, so only a negative-yaw case separates them.

The known risk on the Java side: M8.2's harness subclasses `Entity`, and a whole
movement tick needs `EntityLivingBase`, which is abstract and reads its movement
speed from an attribute map rather than a field. The plan's fallback is
`EntityPlayerMP`, which is concrete but needs more setup. Try the subclass first
and record which worked.

## What needs a person

Nothing in M8.4. M8.8's vanilla lane needs labelled scenario recordings — walk,
sprint, jump, sneak, fall, collide — driven through the relay by a real 1.8.9
client, which means somebody at a keyboard. The capture that exists is
unstructured play: enough to settle the cadence, which it did, and not enough to
compare six scenarios against.

Committing capture-derived fixtures would additionally need a scrubbing step,
because a recording holds usernames, UUIDs, and chat. M8.4's own fixtures come
from the jar through the oracle and are plain numbers, so they carry none of that
and can be committed as the plan says.
