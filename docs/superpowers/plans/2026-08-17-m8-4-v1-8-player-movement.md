# M8.4 v1_8 Player Movement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver `movement` and `profile/java/v1_8` for the player, so that a walking, sprinting, jumping, sneaking, falling, and colliding player reproduces Java Edition 1.8.9 trajectories, checked against the game itself rather than against fixtures derived from a reading of it.

**Architecture:** `profile/java/v1_8` owns the constants, the `float32` arithmetic, and the tick phase order; it is the only package here that imports `minecraft-protocol/data`. `movement` owns the rules those phases call, written against `sim` and `collision` and knowing no version. `collision` and `geom` do not change.

**Tech Stack:** Go 1.26.6, `minecraft-protocol` for generated game data, standard library otherwise, Devbox, go-task, `gofumpt` plus `gci`.

## Global Constraints

- Module is `github.com/go-theft-craft/minecraft-simulation`. Go directive is `1.26.6`.
- **Only `profile/java/v1_8` imports a protocol package.** `geom`, `world`, `entity`, `collision`, `sim`, `runtime`, and `movement` must not import `github.com/go-theft-craft/minecraft-protocol/...`. `movement` receives constants through `sim.Profile` and `sim.MotionConstants`.
- **`float32` is confined to the profile.** Every product vanilla computes in `float` is computed in `float32` here and widened once, at the boundary where the game widens it. `movement` and `collision` see `float64` only. This is the rule the sequencing design froze, and M8.2 showed what breaks without it.
- Standard library only beyond the protocol module. No new third-party dependencies.
- Determinism: no wall clock, no global random state, no map iteration in any result.
- No decompiled Java, no Mojang asset, and no source text from the reference workspace is committed. The numeric sequences in this plan are behavioural statements written in prose and Go, not transcribed source.
- Formatting is `gci` then `gofumpt`; run `devbox run -- task fmt` before every commit.
- Commit messages use Conventional Commits. Never add a `Co-Authored-By` or `Claude-Session` trailer.
- Every task ends with `devbox run -- task verify` passing.

## Why this plan gates on the game and not on fixtures

The sequencing design gates M8.4 on recorded fixtures and moves the zero-corrections check to M8.8. It also says plainly what that costs: "If M8.4 passes fixtures but M8.8 draws corrections, the fault is likelier in the fixtures than in the kernel."

M8.2 turned that from a worry into a measurement. Its plan described vanilla's step-up in eight careful prose statements, and two of them were wrong in ways no fixture written from the same prose could have caught: the recorded Y motion of a step, and the width of `stepHeight`. Both were found within minutes of running the game's own bytecode.

So this plan inverts the emphasis. Fixtures are still delivered, because M8.7 and M9 need a portable, protocol-free suite that runs without a JDK. But the fixtures are **generated from the game** through `internal/oracle` rather than hand-derived, and the milestone's real gate is a differential test against `EntityLivingBase.moveEntityWithHeading`. Hand-written fixtures record a belief; generated ones record the game.

This does not remove M8.8's live-server gate. A live server checks things a jar in a harness cannot: that the position we send is the position it expects, at the packet cadence it expects. It stops being the *first* place a wrong constant surfaces.

## The tick this plan reproduces

Recorded from the 1.8.9 server reference workspace during planning. These are behavioural statements, not source. Every value marked `f32` is computed in `float32` and widened only where stated.

**The player's land-movement tick, in order:**

1. **Jump countdown.** A counter decrements toward zero at the top of the tick.
2. **Jump.** If jump input is held, the body is on the ground, and the counter is zero: vertical motion is set to `0.42` (`f32`), a sprinting body additionally receives `-sin(yaw) * 0.2` on X and `+cos(yaw) * 0.2` on Z (`f32`, sin from the table), and the counter is set to ten. If jump input is not held, the counter is set to zero.
3. **Input decay.** Strafe and forward input are each multiplied by `0.98` (`f32`) *before* they are used.
4. **Friction.** `0.91` (`f32`) when airborne. When on the ground, the slipperiness of the block below multiplied by `0.91` (`f32`). The block below is at `floor(posX)`, `floor(box.MinY) - 1`, `floor(posZ)`.
5. **Acceleration.** `0.16277136 / friction³` (`f32`).
6. **Speed.** On the ground, the movement-speed attribute multiplied by the acceleration (`f32`). Airborne, a jump movement factor whose default is `0.02` (`f32`).
7. **Apply input.** Given strafe `s`, forward `f`, and speed `v`: if `s² + f² < 1e-4` nothing happens. Otherwise a scale `v / max(1, sqrt(s² + f²))` is applied to both inputs, and then `motionX += s·cos(yaw) − f·sin(yaw)` and `motionZ += f·cos(yaw) + s·sin(yaw)`. Every term is `f32`, each sum is formed in `f32`, and only the final term is widened and added to the `float64` motion.
8. **Move.** The collision resolve M8.2 delivered, with the motion as it now stands.
9. **Gravity.** Vertical motion decreases by `0.08`. This one is a `double` literal, not a widened `float`, which is why `physics.json` records it as exactly `0.08` while its neighbours are not round.
10. **Vertical drag.** Vertical motion is multiplied by `0.98` (`f32`).
11. **Horizontal drag.** Horizontal motion is multiplied by the friction from step 4, recomputed from the *pre-move* position. The recomputation is why a player who walks off ice keeps ice friction for the tick that leaves it.

**Two lookup rules the trigonometry depends on:**

- Sine is a table read, not a computation: `table[int(angle · 10430.378) & 65535]`, with the multiply in `f32` and the conversion truncating toward zero. Cosine reads the same table at `int(angle · 10430.378 + 16384) & 65535`.
- The truncation is toward zero for negative angles, and the mask is applied to the resulting signed integer. Go's `int32` conversion and `&` behave identically on two's complement, so this ports directly — but it must be written deliberately, because rounding instead of truncating changes the index for roughly half of all angles.

**Ladders, water, and lava are out of scope.** They are separate branches of the same method, and the scope of this milestone is the land tick. The phase list must leave room for them rather than pretend they do not exist.

## Design decisions this plan settles

### The profile is a fixed phase list, and the order is the profile's

`sim.Profile.Phases()` returns the eleven steps above as named phases with namespaced identifiers, in that order. The order is data, not control flow, so M8.7 can build a different order for 26.1.2 without rewriting a rule, and a custom profile can insert a phase at a named boundary as the parent design requires.

Phases that read no state and write no state, like the jump countdown, still exist as phases. Collapsing them into their neighbours would make the phase list stop describing the tick.

### Per-entity movement state does not fit in `entity.State`

The tick needs a jump counter, strafe and forward input, yaw, sprint and sneak flags, a jump movement factor, and a movement-speed attribute. None of that is geometry, and M8.3 deliberately kept `entity.State` to geometry and motion so it would stay comparable.

This plan adds `movement.Input` (what the controller asked for this tick, arriving as a `sim.Command`) and `movement.Locomotion` (what persists between ticks: the jump counter, the flags, the speed). `Locomotion` lives in the store as a second entity-keyed map, reached through a new view, which means M8.3's `sim.Op` gains one kind. That is a contract change to a milestone already closed, so it is Task 1 and it is done deliberately rather than smuggled in.

### Angles are `float32` state

Yaw is a `float` in the game and it indexes the sine table, so storing it as `float64` and narrowing per read would be a second chance to be wrong. `Locomotion.Yaw` is `float32`. It is the second `float32` in the module after the table itself, and it is in the profile's own state type, not in `geom`.

### The oracle generates the fixtures

`internal/oracle` gains a movement harness that drives the game's own `moveEntityWithHeading` on a minimal living entity, over the same world stubs the M8.2 harness uses. A generator writes multi-tick trajectories to a fixture file. `mctest` replays fixtures with no JDK and no jar.

A generated fixture records the game's answer, so a fixture and the differential test cannot disagree about what vanilla does — only about whether our code still matches it. That is the property that makes fixtures worth shipping to M8.7.

## What this plan deliberately does not decide

- The `mctest` runner beyond replaying movement fixtures. Later mechanics extend it.
- Water, lava, ladders, vines, webs, slime blocks, elytra, and flight.
- Sprint state transitions driven by hunger or collision, and food. The plan takes sprinting as an input flag.
- Potion effects on jump and speed. The phase list leaves the hook; the rules arrive with M9.

## File structure

```text
minecraft-simulation/
  movement/input.go              Input command, Locomotion state
  movement/input_test.go
  movement/view.go               LocomotionView, and an in-memory implementation
  movement/friction.go           Friction, acceleration, and speed rules
  movement/friction_test.go
  movement/heading.go            Applying input to motion through a sine table
  movement/heading_test.go
  movement/gravity.go            Gravity and the two drags
  movement/gravity_test.go
  movement/step.go               The collision step, and its result to state
  movement/step_test.go
  movement/jump.go               The jump countdown and the jump impulse
  movement/jump_test.go
  movement/trig.go               Table-backed sine and cosine
  movement/trig_test.go
  profile/java/v1_8/profile.go   New, ID, Slipperiness, Motion, Shape, Phases
  profile/java/v1_8/profile_test.go
  profile/java/v1_8/phases.go    The eleven phases, in order
  profile/java/v1_8/phases_test.go
  profile/java/v1_8/blocks.go    data.Blocks and data.CollisionShapes to handles
  profile/java/v1_8/blocks_test.go
  mctest/fixture.go              Fixture format, load and save
  mctest/fixture_test.go
  mctest/replay.go               Replaying a fixture against a profile
  mctest/replay_test.go
  mctest/testdata/*.json         Generated fixtures, committed
  internal/oracle/java/MovementOracle.java
  internal/oracle/movement_test.go
  internal/oracle/generate_test.go   Writes mctest/testdata, behind a flag
```

---

## Task 1: Extend the M8.3 contracts for per-entity locomotion

**Files:**
- Modify: `sim/change.go`, `sim/digest.go`, `sim/profile.go`, `sim/result.go`
- Create: `movement/input.go`, `movement/view.go`
- Test: `sim/digest_test.go`, `movement/input_test.go`

**Interfaces:**
- Produces:
  - `movement.Locomotion struct { JumpTicks int32; Yaw, Pitch float32; Sprinting, Sneaking, Jumping bool; MoveSpeed, JumpFactor float32 }`
  - `movement.Input struct { Strafe, Forward float32; Yaw, Pitch float32; Jump, Sprint, Sneak bool }` implementing `sim.Command`
  - `movement.LocomotionView interface { Locomotion(entity.ID) (Locomotion, bool) }`
  - `movement.Bodies` in-memory implementation with `Clone`
  - `sim.OpSetLocomotion` and the `Op.Locomotion` field
  - `sim.TickInput.Locomotion` of type `movement.LocomotionView`
  - `sim.TickState.Locomotion(entity.ID) (movement.Locomotion, bool)` and `SetLocomotion(entity.ID, movement.Locomotion)`

This is a change to contracts M8.3 closed, so it comes first and it is explicit. `sim` importing `movement` is the right direction: `movement` is a rule package below the profile and above the kernel's data types, and the alternative — a `map[string]any` bag on `entity.State` — would cost comparability and canonical encoding, which are the two properties the digest depends on.

If that import direction proves wrong when the code exists, the fix is to move `Locomotion` into `entity` as a second state struct, not to weaken the encoding. Record which was chosen in the commit message.

- [ ] **Step 1: Write the failing tests**

Extend `TestDigestNoticesEveryField` with a `locomotion` case that changes `Op.Locomotion.JumpTicks` and requires the digest to change. Add `movement/input_test.go` covering: `Input.CommandKind() == "movement.input"`; `Locomotion` is comparable; `Bodies` returns what it was given, reports a missing entity, and clones without aliasing.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ ./movement/`

- [ ] **Step 3: Write `movement/input.go` and `movement/view.go`**

- [ ] **Step 4: Extend `sim`**

Add the operation kind, the field, the view on `TickInput`, the two `TickState` methods, and the encoding. The encoder gains a `tagLocomotion` appended to the tag list, never inserted into it, because inserting would renumber every existing tag and change every digest ever recorded.

- [ ] **Step 5: Re-pin the empty-tick digest**

Adding a field to the encoding changes the empty tick's digest. Run `TestEmptyTickDigestIsPinned`, read the new value, and update the constant **in this commit**, so the change is attributable to this task rather than discovered three tasks later.

- [ ] **Step 6: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [ ] **Step 7: Commit**

```bash
git add sim/ movement/
git commit -m "feat(sim): carry per-entity locomotion state through the tick"
```

---

## Task 2: Table-backed sine and cosine

**Files:**
- Create: `movement/trig.go`
- Test: `movement/trig_test.go`

**Interfaces:**
- Produces:
  - `type Table struct { ... }`
  - `func NewTable(entries []float32) (Table, error)`
  - `func (t Table) Sin(angle float32) float32`
  - `func (t Table) Cos(angle float32) float32`

`NewTable` requires exactly 65536 entries and rejects anything else, because a short table would silently wrap and produce plausible wrong angles.

`Sin` reads `entries[int32(angle * 10430.378) & 65535]`. Two details decide correctness and both are easy to get wrong: the multiply is `float32`, and the conversion truncates toward zero rather than rounding. `Cos` adds `16384` inside the `float32` multiply's result before converting.

- [ ] **Step 1: Write the failing test**

Cover:
- `NewTable` rejects a table of the wrong length and accepts 65536 entries.
- `Sin(0)` is the table's first entry and `Cos(0)` is entry 16384, so the index arithmetic is pinned rather than merely plausible.
- A negative angle truncates toward zero: build a table whose entries are their own indices as floats, then assert the index chosen for a small negative angle is the one truncation gives and not the one rounding or flooring would give. Name the expected index as a literal.
- `Sin` over a full turn stays within `[-1, 1]` and `Sin(angle)` and `Sin(angle + 2π)` agree for a spread of angles, which is what the mask is for.
- The table is not mutated by a read, and `NewTable` copies its input.

- [ ] **Step 2: Run the test to verify it fails**

- [ ] **Step 3: Write the implementation**

```go
// Sin returns the game's sine: a table read, not a computation.
//
// The game builds a 65536-entry table at class initialization and never calls a
// sine function at runtime, so reproducing the formula in Go would risk
// last-place divergence for no benefit. The table is dumped and pinned instead.
//
// Two details are load-bearing. The multiply is float32, and the conversion
// truncates toward zero rather than rounding, which for a negative angle picks a
// different entry than rounding would. The mask then wraps the signed index,
// which is what makes the table periodic.
func (t Table) Sin(angle float32) float32 {
	return t.entries[int32(angle*10430.378)&65535]
}
```

- [ ] **Step 4 to 6: Verify, check, and commit**

```bash
git add movement/trig.go movement/trig_test.go
git commit -m "feat(movement): add table-backed sine and cosine"
```

---

## Task 3: Friction, acceleration, and speed

**Files:**
- Create: `movement/friction.go`
- Test: `movement/friction_test.go`

**Interfaces:**
- Produces:
  - `func GroundFrictionBlock(box geom.AABB, pos geom.Vec3) geom.BlockPos`
  - `func Friction(slipperiness float32, onGround bool) float32`
  - `func Acceleration(friction float32) float32`
  - `func Speed(friction float32, onGround bool, moveSpeed, jumpFactor float32) float32`

Every signature is `float32` in and `float32` out. That is the discipline: the widening happens once, where the motion is updated, and nowhere else.

`GroundFrictionBlock` returns the cell whose slipperiness applies: `floor(posX)`, `floor(box.MinY) - 1`, `floor(posZ)`. It takes the box and the position separately because the game uses the box for the vertical coordinate and the position for the horizontal ones, and a function that took only one of them would have to guess.

- [ ] **Step 1: Write the failing test**

Cover:
- `Friction` is exactly `0.91` widened from `float32` when airborne, whatever the slipperiness.
- On the ground with default slipperiness `0.6`, `Friction` is the `float32` product and **not** the `float64` one. Assert against `float32(0.6) * float32(0.91)` computed in the test, and add a second assertion that the result differs from `float64(0.6) * float64(0.91)`. That second assertion is the point: it fails if anyone widens too early.
- `Acceleration` for the default ground friction, pinned as a literal read from the first run.
- `Speed` returns the jump factor unchanged when airborne, and the product when grounded.
- `GroundFrictionBlock` for a body flush at `y = 0` returns `y = -1`, and for a body at `y = 0.5` also returns `y = -1`, since the floor of `0.5` is `0`.

- [ ] **Step 2 to 6: Fail, implement, verify, check, commit**

```bash
git add movement/friction.go movement/friction_test.go
git commit -m "feat(movement): add ground friction, acceleration, and speed"
```

---

## Task 4: Applying input to motion

**Files:**
- Create: `movement/heading.go`
- Test: `movement/heading_test.go`

**Interfaces:**
- Produces: `func ApplyHeading(table Table, motion geom.Vec3, strafe, forward, speed, yaw float32) geom.Vec3`

This is step 7 of the tick and the single most float-sensitive function in the milestone. The order of operations is fixed:

1. `magnitude = strafe² + forward²` in `float32`. If it is below `1e-4`, return the motion unchanged.
2. `magnitude = sqrt(magnitude)` as a `float32` narrowing of a `float64` square root, which is what the game does and what makes it portable: IEEE square root is exactly rounded.
3. If `magnitude < 1`, it becomes `1`.
4. `scale = speed / magnitude`, then `strafe *= scale` and `forward *= scale`, all `float32`.
5. `sin = table.Sin(yaw · π / 180)` and `cos = table.Cos(yaw · π / 180)`, with the conversion factor in `float32`.
6. `motion.X += float64(strafe·cos − forward·sin)` and `motion.Z += float64(forward·cos + strafe·sin)`. Each bracket is formed in `float32` and widened once.

- [ ] **Step 1: Write the failing test**

Cover:
- Zero input returns the motion unchanged, including the case where both inputs are tiny but non-zero and their squares fall below the threshold.
- Input at exactly the threshold: assert which side of `1e-4` is inclusive, matching the game's `<`.
- Facing zero yaw with forward input moves along one axis and leaves the other alone, which pins the sign convention.
- Diagonal input is normalized: full strafe and full forward do not travel `sqrt(2)` times as fast.
- Input below unit magnitude is *not* normalized up, which is the `max(1, …)` clamp.
- The widening happens once: compute the expected value in the test with explicit `float32` brackets and compare bit for bit, and add a case whose `float64`-throughout answer differs, so the test fails if the discipline is broken.
- Yaw of `360` and yaw of `0` agree, exercising the table mask.

- [ ] **Step 2 to 6: Fail, implement, verify, check, commit**

```bash
git add movement/heading.go movement/heading_test.go
git commit -m "feat(movement): apply strafe and forward input to motion"
```

---

## Task 5: Gravity, drags, jumping, and the collision step

**Files:**
- Create: `movement/gravity.go`, `movement/jump.go`, `movement/step.go`
- Test: one test file each

**Interfaces:**
- Produces:
  - `func ApplyGravity(motion geom.Vec3, gravity float64) geom.Vec3`
  - `func ApplyVerticalDrag(motion geom.Vec3, drag float32) geom.Vec3`
  - `func ApplyHorizontalDrag(motion geom.Vec3, friction float32) geom.Vec3`
  - `func Countdown(state Locomotion) Locomotion`
  - `func Jump(table Table, state Locomotion, motion geom.Vec3, upwards float32) (Locomotion, geom.Vec3, bool)`
  - `func Step(view world.View, state entity.State, limit int) (collision.Result, error)`

`ApplyGravity` takes a `float64` because gravity is a `double` literal in the game. Its neighbours take `float32` because theirs are not. The asymmetry looks like an inconsistency and is the opposite: it is the reason `physics.json` records `0.08` exactly and `0.9800000190734863` for the drag beside it.

`Jump` reports whether it fired, so the phase can set the counter without duplicating the conditions. The sprint impulse uses the table, not a computed sine.

`Step` is a thin adapter over `collision.Resolve`: it builds a `collision.Move` from the body, passes the candidate budget through, and returns the result untouched. It must not adjust the result. M8.2 established that `collision.Result` reports what the game reports, including a step's Y motion being the settle alone, and a phase that "fixed" that would reintroduce the bug the oracle caught.

- [ ] **Step 1: Write the failing tests**

Cover for gravity and drags: each function touches only the components it should; the vertical drag result is bit-identical to the `float32` product widened; and gravity applied to a `float64` motion is a plain subtraction, so a test that expects `0.08` to be widened from `float32` fails.

Cover for jump: it fires only when the body is on the ground, the jump flag is set, and the counter is zero; it sets the counter to ten; a sprinting jump adds the horizontal impulse and a walking jump does not; releasing the jump flag zeroes the counter.

Cover for step: a free move passes through unchanged; an unknown region surfaces as a non-empty `Unknown` in the returned result rather than an error; the candidate budget reaches `collision`, which a zero-budget case with a huge motion proves by returning `collision.ErrCandidateLimit`.

- [ ] **Step 2 to 6: Fail, implement, verify, check, commit**

```bash
git add movement/gravity.go movement/jump.go movement/step.go movement/*_test.go
git commit -m "feat(movement): add gravity, drags, jumping, and the collision step"
```

---

## Task 6: The v1_8 block table

**Files:**
- Create: `profile/java/v1_8/blocks.go`
- Test: `profile/java/v1_8/blocks_test.go`

**Interfaces:**
- Produces:
  - `type blockTable struct { ... }`
  - `func newBlockTable(bundle data.Bundle) (blockTable, error)` — adjust to the real constructor `minecraft-protocol` exposes
  - `func (t blockTable) shape(ref world.BlockRef) (geom.Shape, bool)`
  - `func (t blockTable) slipperiness(ref world.BlockRef) float32`
  - `func (t blockTable) ref(name string, meta uint8) (world.BlockRef, bool)`

This is where `data.CollisionShapes`, `data.Blocks`, and `data.Physics` become `world.BlockRef` handles. A handle is an index into this table, so the profile that minted it is the only thing that can read it, exactly as `world.BlockRef` promises.

Slipperiness is stored as `float32`, because the game's field is a `float` and the friction product must be computed at that width. `data.Physics.Slipperiness` returns `float64`; narrow once here, at the boundary, and assert in a test that the narrowing round-trips for every block in the dataset. If any value does not round-trip, that is a finding about `physics.json` and it must be reported rather than papered over.

`ref` exists for tests and fixtures, which name blocks rather than handles.

- [ ] **Step 1: Write the failing test**

Cover: the table resolves stone to a full cube and air to an empty shape; slipperiness of ice, packed ice, slime, and soul sand differ from the default, so the dataset is genuinely wired up and not defaulting silently; an unknown handle reports `false` rather than a zero shape; and every slipperiness in the dataset narrows to `float32` and back without changing value.

- [ ] **Step 2 to 6: Fail, implement, verify, check, commit**

```bash
git add profile/java/v1_8/blocks.go profile/java/v1_8/blocks_test.go
git commit -m "feat(profile): map 1.8.9 block data to simulation handles"
```

---

## Task 7: The v1_8 profile and its phase list

**Files:**
- Create: `profile/java/v1_8/profile.go`, `profile/java/v1_8/phases.go`
- Test: `profile/java/v1_8/profile_test.go`, `profile/java/v1_8/phases_test.go`

**Interfaces:**
- Produces:
  - `func New(bundle data.Bundle) (sim.Profile, error)`
  - Eleven unexported phase types with the identifiers below

Phase identifiers, in order, matching the eleven steps of the tick:

```text
v1_8.jump-countdown
v1_8.jump
v1_8.input-decay
v1_8.friction
v1_8.acceleration
v1_8.apply-input
v1_8.move
v1_8.gravity
v1_8.vertical-drag
v1_8.horizontal-drag
v1_8.commit
```

`friction` and `acceleration` are separate identifiers even though one feeds the other, because a custom profile that wants different friction should not have to reimplement acceleration to get it.

`commit` writes the body and the locomotion state through `sim.TickState`. Everything before it carries per-tick values in a scratch structure the phases share, so that a tick that turns out to be incomplete leaves nothing behind — the kernel drops operations, and there were none to drop until `commit` ran.

The friction the horizontal drag uses is the one `v1_8.friction` computed from the pre-move position. The phase list is what makes that guaranteed rather than incidental: `move` runs between them and cannot change what `friction` already recorded.

- [ ] **Step 1: Write the failing test**

Cover:
- `New` returns a profile whose identity is `java/1.8.9@1` and whose phase list has the eleven identifiers above, in that order, asserted as a literal slice. A reordering must fail this test loudly, because reordering is exactly the kind of change that quietly breaks trajectories.
- `New` rejects a bundle with no physics.
- `Motion(entity.FamilyPlayer)` returns the four constants from `physics.json`, with `StepHeight` equal to `float64(float32(0.6))`. Assert the exact value: this is the constant M8.2's oracle caught, and a test that accepts `0.6` would let it regress.
- `Slipperiness` of an unknown handle returns the default.
- Running one tick of a standing player on a stone floor through `sim` and `runtime` leaves the body on the floor and reports it on the ground.

- [ ] **Step 2 to 6: Fail, implement, verify, check, commit**

```bash
git add profile/java/v1_8/
git commit -m "feat(profile): add the Java 1.8.9 player profile and its tick phases"
```

---

## Task 8: The movement oracle

**Files:**
- Create: `internal/oracle/java/MovementOracle.java`
- Create: `internal/oracle/movement_test.go`

**Interfaces:**
- Produces no Go API. The harness protocol mirrors `MoveOracle`: `C` clears the world, `B x y z kind` places a block, and a new `T` command runs one whole movement tick and prints the resulting position, motion, and flags.

This is the milestone's real gate. It drives the game's own `moveEntityWithHeading` and the jump logic on a minimal `EntityLivingBase` subclass, using the same world stub `MoveOracle` uses, and compares whole trajectories against ours.

The Java work is larger than `MoveOracle`'s because `EntityLivingBase` has more abstract methods and reads attributes for its movement speed. The overrides needed are: the three `Entity` abstract methods, the equipment accessors, `getTotalArmorValue`, and whatever the compiler names when it refuses the subclass. Movement speed is set through the attribute map rather than a field, and the harness must set it explicitly rather than relying on a default that a living entity without AI may not have.

**If the subclass proves impractical**, the fallback is to drive `EntityPlayerMP`, which is concrete. It needs a `GameProfile`, a `MinecraftServer`, and an interaction manager, which is more setup but no guesswork. Try the minimal subclass first and record which one worked and why in the commit message. Do not fall back to transcribing the tick into Java: that is a derivative work of the kind the parent design rejects, and it would test our reading rather than the game.

- [ ] **Step 1: Write the harness and a smoke test**

Run one tick of a player standing on a stone floor with no input, print the result, and compare it by hand against the tick above: motion should be `0.08` of gravity, dragged, and clamped by the floor to zero, leaving the body where it was and on the ground.

- [ ] **Step 2: Write the differential test**

For each of the six scenarios the exit criterion names — walk, sprint, jump, sneak, fall, collide — and for a spread of random worlds and yaws:

1. Build the world in `world.Blocks` and in the harness.
2. Run 100 ticks through `sim` with the v1_8 profile, and 100 ticks through the harness.
3. Compare position, motion, and the collision flags **every tick**, bit for bit.

Comparing every tick rather than the endpoint is what makes a failure debuggable: the first differing tick names the phase, where an endpoint comparison names only the scenario.

Expect this test to fail on first run. That is its purpose. Each failure is a finding about the tick above; fix the rule, and record findings worth keeping in the README, as M8.2 did.

- [ ] **Step 3: Run it, fix what it finds, and record**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./internal/oracle/ -run TestMovement -v`

- [ ] **Step 4 to 6: Verify, check, and commit**

```bash
git add internal/oracle/
git commit -m "test(oracle): check a whole movement tick against a real 1.8.9 server"
```

---

## Task 9: Fixtures generated from the game, and the replay runner

**Files:**
- Create: `mctest/fixture.go`, `mctest/replay.go`, `mctest/testdata/*.json`
- Create: `internal/oracle/generate_test.go`
- Test: `mctest/fixture_test.go`, `mctest/replay_test.go`

**Interfaces:**
- Produces:
  - `type Fixture struct { Profile sim.ProfileID; Blocks []Block; Body Body; Ticks []Tick }`
  - `func Load(path string) (Fixture, error)`, `func (f Fixture) Save(path string) error`
  - `func Replay(profile sim.Profile, fixture Fixture) error`

A fixture records the profile identity, the world as named blocks with metadata rather than handles, the initial body, and per-tick input with the expected position, motion, and flags. Blocks are named because a handle is meaningless outside the profile that minted it, and a fixture must survive the table being renumbered.

`internal/oracle/generate_test.go` writes the fixtures from the harness, behind an explicit flag so an ordinary test run never rewrites committed expectations. Regenerating is a deliberate act with a visible diff.

`Replay` needs no JDK and no jar. That is what makes the suite portable to M8.7 and to CI.

- [ ] **Step 1: Write the fixture format and its round-trip test**

Cover: save then load returns an equal fixture; a fixture naming an unknown block fails to replay with an error that names the block; a fixture whose profile identity does not match the profile fails before simulating, per the parent design.

- [ ] **Step 2: Generate the fixtures**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./internal/oracle/ -run TestGenerateFixtures -args -write-fixtures`

- [ ] **Step 3: Replay them without a JDK**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && PATH=/usr/bin devbox run -- go test ./mctest/ -v`

Expected: PASS, with the six scenarios replaying from committed data. Confirm the oracle tests skip under this `PATH` rather than fail, which proves the suite is genuinely portable.

- [ ] **Step 4 to 6: Verify, check, and commit**

```bash
git add mctest/ internal/oracle/generate_test.go
git commit -m "test(mctest): generate movement fixtures from the game and replay them"
```

---

## Task 10: The milestone record

**Files:**
- Modify: `README.md`, `CHANGELOG.md`

- [ ] **Step 1: Record the packages and the findings**

Extend the package table with `movement`, `profile/java/v1_8`, and `mctest`. Add a section stating the `float32` discipline and where the widening happens, because it is the rule most likely to be broken by a later contributor who sees `float32` in a signature and assumes it is a mistake.

Record every divergence Task 8 found, in the same form M8.2 used: what we believed, what the game does, and how it was found.

- [ ] **Step 2: Changelog, verify, commit**

```bash
git add README.md CHANGELOG.md
git commit -m "docs: record M8.4 and what the movement oracle found"
```

---

## Definition of done

- `movement` implements the eleven steps of the 1.8.9 land tick, and imports no protocol package.
- `profile/java/v1_8` supplies the constants, the sine table, the block table, and the phase order, and is the only package here importing `minecraft-protocol/data`.
- Every product the game computes in `float` is computed in `float32`, widened once where the game widens it, and a test for each such product fails if it is widened early.
- `Motion(entity.FamilyPlayer).StepHeight` is `float64(float32(0.6))`, asserted exactly.
- **The differential test passes:** walk, sprint, jump, sneak, fall, and collide agree with the game's own `moveEntityWithHeading` every tick, bit for bit, over random worlds and yaws.
- **Fixture conformance passes:** the same six scenarios replay from committed, game-generated fixtures with no JDK and no jar present.
- `devbox run -- task verify` passes, and the M8.2 oracle tests still pass untouched.

## Follow-on

M8.6 hashes these results across platforms. The `float32` products are where a platform difference would show, and this milestone is where they all appear, so M8.6's matrix is testing this code more than it is testing the encoder.

M8.7 reuses the fixture format and the harness technique for 26.1.2, and needs the dumper extended first. Its plan says so.

M8.8 keeps the live-server gate. What this milestone changes is that a correction from a real server will point at the adapter or the packet cadence rather than at a constant, because the constants have already been checked against the game.
