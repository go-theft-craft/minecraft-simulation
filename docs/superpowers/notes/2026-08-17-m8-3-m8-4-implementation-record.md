# M8.3 and M8.4: what was implemented, what diverged, what the data said

Written 2026-08-17 at the point M8.4 Task 7 landed, and extended the same day
when Tasks 8, 9, and 10 completed the milestone. It records the state of the
work, every deviation from the two plans, and the findings that came out of
measurement rather than reading. It is not a plan: the plans are
`2026-08-17-m8-3-kernel-contracts.md` and
`2026-08-17-m8-4-v1-8-player-movement.md`, and this note says where the code
differs from them and why.

## State

**M8.3 is complete.** All eleven tasks, eleven commits, `9a572e6`..`e545ad5`.
Both of its exit criteria are tested: an empty tick digests identically across
runs and is pinned in `sim/kernel_test.go`, and a change set based at revision N
is refused by a store at revision N+1 with nothing written
(`runtime/runner_test.go`).

**M8.4 is complete.** `movement` holds the rules of the 1.8.9 land tick,
`profile/java/v1_8` supplies the constants, the sine table, the block table, and
the phase order, and both are checked against the game rather than asserted.
Commits `d7add26`, `286349a`, `16ffc19`, `e8b96ff`, `d8af4c0`, `844557e`,
`3029754`, `525b381`, `28963f4`.

Both gates pass. The differential test runs six scenarios over eight randomly
obstructed rooms each, a hundred ticks apiece — 4,800 ticks — and compares
position, motion, and the collision flags against
`EntityLivingBase.onLivingUpdate` bit for bit at every tick. The committed
fixtures replay the same six scenarios with no JDK, no jar, and no prepared
workspace, and every expectation in them is the jar's answer rather than ours.

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
   every phase after it reads them. Task 8 confirmed both orderings.
7. **The tick has twelve phases, not eleven.** `v1_8.motion-threshold` runs
   between the countdown and the jump. The plan's tick had eleven steps and did
   not describe this one; the oracle did.
8. **`sim` gained `BlockNames`**, an optional interface for resolving a block
   name to a handle. Task 9 needed it: a fixture describes its world by name,
   because a handle is meaningless outside the profile that minted it, and no
   rule ever looks a block up by name so the lookup does not belong on
   `sim.Profile`.
9. **A fixture describes its world as a filled region plus named boxes**, not the
   plan's list of named cells. A floor written cell by cell is four hundred
   entries of stone, and the plan asks for a regeneration to leave a diff
   somebody can read.

## What Task 8 settled

The three readings it was pointed at all held. Gravity and the two drags do run
after the move, in that order; sneak scaling does precede the per-tick input
decay; and `Cos` does add its quarter turn as a float before the truncation —
2,400 ticks of walking, turning past zero and through negative yaws, agree.

The Java risk did not materialise. A minimal `EntityLivingBase` subclass works
and `EntityPlayerMP` was not needed: `getAIMoveSpeed` on a non-player living
entity returns a plain field, `setAIMoveSpeed` sets it, and the only abstract
methods left are the four equipment accessors. Nearby-entity pushing and the
fall-state callback are overridden away, because the stub world has no chunk
provider to find a second entity with and fall damage is not movement.

Driving `onLivingUpdate` rather than assembling the tick from `jump` and
`moveEntityWithHeading` was the decision that made the gate worth having: the
jump counter is private to `EntityLivingBase`, so the only honest way to check a
counter is to let the game keep it, and the motion threshold below would not have
been found by a harness that only called the two methods the plan named.

### What it found

Four rules were wrong, and each had passed every test written from prose:

1. **The player's box is not 0.6 by 1.8.** The game halves a float width and adds
   a float height to a double position, so the body reaches
   `0.30000001192092896` from its centre and stands `1.7999999523162842` tall.
   Caught on the tick the body was created, before a rule had run.
2. **Both drags are double products.** The constants are floats and the motion is
   a double, so Java widens the constant to meet it. Our rules narrowed the
   motion first, which is a different number, and it was wrong for a body doing
   nothing but standing on a floor.
3. **The heading converts degrees in two float steps.** `yaw * pi_f / 180`, not
   `yaw * (pi/180)_f`. The jump impulse three rules away really does use the
   pre-divided constant, so both forms are in the game and they disagree: at some
   yaws they read neighbouring entries of the sine table. Found as a body that
   drifted four millionths of a block per tick along one axis while matching
   exactly along the other.
4. **The tick discards any component of motion below `0.005`, before the jump.**
   No plan described this rule. Without it a body walking at any angle other than
   square on diverges within four ticks. It is a twelfth phase,
   `v1_8.motion-threshold`.

### What it could not reach

Two things, both recorded rather than fixed:

- **The vertical clamp is not a rule of the tick.** The game calls the landing
  behaviour of the block under the body's feet, whose default zeroes the vertical
  motion — which is what we implement — and whose slime override negates it,
  bouncing the body. There is no per-block landing hook here, so slime is kept
  out of the differential worlds and its slipperiness is unchecked against the
  game. The hook belongs with M9's block behaviours.
- **The sneak scale is a client transform.** The 1.8.9 client scales its own
  input on the way out, as `(float)(axis * 0.3)`, and the server entity the
  harness drives never sees an unscaled axis. It is folded into the profile at
  that width and pinned by a unit test with an axis searched for because it
  separates the two widths — for the -1, 0, and 1 a keyboard produces they are
  identical, so nothing measurable could have decided it either way.

Two smaller things worth carrying forward. Sprinting is not a rule of the tick
either: it raises the movement-speed attribute by 30% and the air factor by the
same fraction, both in `EntityPlayer.onLivingUpdate` and both *after* the
movement runs, so the values a tick multiplies by are the previous tick's.
Whatever fills `Locomotion` owns that. And the sneak edge-guard that stops a
crouching player walking off a ledge is gated on `instanceof EntityPlayer`, so
neither the harness's stub nor our collision applies it; a real player at a real
server will, which is M8.8's problem to notice.

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
