# M8.7 v26_1 Player Movement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver `profile/java/v26_1` for the player, so that the M8.4 scenario suite passes on Java Edition 26.1.2, and extend the ground-truth pipeline far enough to supply that version's constants.

**Status:** Complete, 2026-08-18. Every task is done and the gate is the game:
4,800 ticks of the six scenarios agree with a real 26.1.2 server bit for bit.
Task 1's answer is in
`docs/superpowers/notes/2026-08-17-v26-1-oracle-feasibility.md`. A jar-backed
oracle is possible and cheaper than 1.8.9's, because Mojang ships this version
unobfuscated — so this milestone takes the strong branch below and the gate is
the game. Three things the note changes: the collision variant is a prerequisite
task rather than a contingency, the oracle's world stub is its largest piece
rather than an afterthought, and four quantities differ from 1.8.9 in formula
rather than in width, so nothing may be ported by analogy.

**Architecture:** The same shape as M8.4 and deliberately not the same code. `profile/java/v26_1` owns 26.1.2's constants, its `float32` arithmetic, and its own phase order. Rules that genuinely coincide with 1.8.9's move into `movement` as shared functions; rules that differ get their own. `collision`, `geom`, `sim`, and `runtime` do not change.

**Tech Stack:** Go 1.26.6, `minecraft-reference` for extraction, `minecraft-protocol` for generated data, Devbox, go-task.

## This plan spans three repositories, and starts with what nobody knows yet

M8.4's plan could state the tick it reproduced, line by line, because a deobfuscated 1.8.9 server sat in the reference workspace and could be read and executed. Almost none of that is true here yet, and pretending otherwise would produce a plan whose tasks are guesses.

What is known:

- **The dumper refuses this version.** `minecraft-reference` gates extraction on the version string and reports that only 1.8.9 is implemented. Its Java dumper source names 1.8.9 identifiers throughout, so extending it is a new dumper rather than a new argument.
- **There is no deobfuscated 26.1.2 jar in the workspace.** `prepare` leaves an original and an executable jar for this version, and decompiled sources, but not the named jar the 1.8.9 oracle compiles against. Whether a named jar can be produced, and by which path, is the first thing to find out, because both the extraction and the oracle depend on it.
- **The movement code is structurally different.** The parent design says so and rates this the largest single risk in the whole subproject: "It shares little structure with 1.8.9, and the version is recent enough that its behavior can still change." Swimming, elytra flight, and the 1.13 collision rework are different code paths, not flags.
- **The constants pipeline is already version-parameterized.** M8.1 left extracted data optional in the manifest schema and the render plan, so a second version generates without touching the first. That part is ready.

So Tasks 1 and 2 are research with committed artifacts as output, and the plan states what it will do with either answer. Everything after them is contingent on what they find, and the plan says where.

## Global Constraints

- Modules touched: `minecraft-reference` (the dumper), `minecraft-protocol` (the dataset and generated package), `minecraft-simulation` (the profile).
- **Only `profile/java/v26_1` imports a protocol package**, and it imports only `minecraft-protocol/data`.
- **`float32` stays in the profile.** 26.1.2's widths must be established independently. Do not assume they match 1.8.9's: a constant that is a `double` in one version may be a `float` in the other, and M8.2 showed what assuming costs.
- No decompiled Java, no Mojang asset, and no source text from the reference workspace is committed, in any of the three repositories.
- The 1.8.9 profile, its fixtures, and its recordings must not change. If a shared rule needs a different shape for 26.1.2, the 1.8.9 behaviour is what proves the refactor was safe, and its committed digests are the check.
- Formatting and commit conventions as in the other plans. No `Co-Authored-By` or `Claude-Session` trailer.

## Design decisions this plan settles

### Shared rules are extracted only after both versions exist

The temptation is to design a shared movement abstraction now and implement both profiles against it. That is backwards. M8.4's rules were written against one version; which of them 26.1.2 shares is a finding, not a premise.

So: implement 26.1.2's rules in its own package first, duplicating whatever looks similar, and extract shared functions into `movement` only where the two are demonstrably identical — same constants, same widths, same order. The 1.8.9 recordings from M8.6 are the safety net for every extraction, because an extraction that changes a 1.8.9 digest is a broken extraction and the matrix says so immediately.

Duplication that survives to the end of this milestone is a finding worth recording, not a failure. Two versions that genuinely differ should read as two implementations.

### The scenario suite is the contract, not the rules

The exit criterion is that the M8.4 suite passes on 26.1.2, which is a statement about scenarios — walk, sprint, jump, sneak, fall, collide — and not about expected numbers. The expected numbers are this version's, generated from this version's jar.

`mctest` therefore gains nothing here beyond a second set of fixtures, provided M8.4 built the format properly: named blocks, a profile identity, per-tick inputs and expectations. If it did not, fixing that is part of this milestone rather than a workaround inside it.

### Scope is the player on land, again

Swimming and elytra are 26.1.2's own mechanics and they are not in the M8.4 suite. Adding them here would mean this milestone gates on scenarios the 1.8.9 profile has no counterpart for, which is a different milestone. The phase list must leave room for them, and the plan records them as out of scope rather than silently absent.

## File structure

```text
minecraft-reference/
  internal/reference/physics/dumper_source_26_1.go   A second dumper program
  internal/reference/physics/dumper.go               Version selection
  internal/reference/physics/dumper_test.go

minecraft-protocol/
  source/java/26.1/physics.json      Committed, digest-pinned
  generated/java/v26_1/physics.go    Generated
  manifest.json                      A second physics entry
  THIRD_PARTY_NOTICES.md             Mojang provenance for the new file

minecraft-simulation/
  profile/java/v26_1/profile.go
  profile/java/v26_1/phases.go
  profile/java/v26_1/blocks.go
  profile/java/v26_1/*_test.go
  movement/shared.go                 Only what both versions provably share
  mctest/testdata/26_1/*.json        Generated fixtures
  replay/testdata/26_1/*.json        Recordings, joining the M8.6 matrix
  internal/oracle/java/MovementOracle26.java
  internal/oracle/movement26_test.go
```

---

## Task 1: Find out whether a 26.1.2 oracle is possible

**Files:**
- Create: `docs/superpowers/notes/2026-08-17-v26-1-oracle-feasibility.md`

**Interfaces:** produces no code. It produces the decision the rest of the plan branches on.

Answer four questions and commit the answers with the commands that produced them:

1. **Can a named 26.1.2 server jar be produced?** Mojang publishes official mappings for modern versions. Determine whether `mcreference prepare` already fetches them, whether it applies them, and if not, what it would take. Record the exact artifact paths that exist today.
2. **Does the movement code reflect cleanly in a headless process?** The 1.8.9 oracle needed an explicit bootstrap call before registries populated. Establish the equivalent for this version, and whether a modern server jar can be initialized far enough to move an entity without starting a world.
3. **Is the entity movement method reachable the same way?** The 1.8.9 harness drove a minimal living-entity subclass. Determine whether the modern hierarchy allows the same, or whether a concrete player type is needed, or whether neither works.
4. **Which constants are `float` and which are `double`?** For gravity, the drags, the friction product, the step height, and the input scale. This is the M8.2 lesson applied before writing code rather than after.

- [x] **Step 1: Answer all four, with evidence**

- [x] **Step 2: State the consequence for this milestone**

Write, explicitly, one of:

- **A jar-backed oracle is possible.** Then this milestone follows M8.4's shape exactly: differential test first, generated fixtures second, and the gate is the game.
- **A jar-backed oracle is not possible.** Then say why, and the milestone's gate degrades to hand-derived fixtures plus M8.8's live server against a 26.1.2 server. Say plainly in the notes and in the master plan that this is the weaker gate M8.4 avoided, and that a constant wrong in the fifteenth decimal place will not be caught until a real client connects.

Do not start Task 3 until this is written down. A milestone whose verification strategy is decided halfway through is a milestone that will be verified by whatever was convenient.

- [x] **Step 3: Commit**

```bash
git add docs/superpowers/notes/2026-08-17-v26-1-oracle-feasibility.md
git commit -m "docs(notes): record whether a 26.1.2 movement oracle is possible"
```

---

## Task 2: Extend the dumper to 26.1.2

**Repository:** `minecraft-reference`

**Files:**
- Create: `internal/reference/physics/dumper_source_26_1.go`
- Modify: `internal/reference/physics/dumper.go`
- Test: `internal/reference/physics/dumper_test.go`

**Interfaces:**
- Produces: version selection replacing the current single-version rejection, and a second embedded dumper program.

The existing dumper rejects every version but 1.8.9 with an explicit error, which is the right shape: it means adding a version is adding a case, and a version nobody has written a dumper for still fails loudly rather than producing an empty dataset.

What the new dumper must produce, matching the 1.8.9 schema so the generator needs no new shape: block slipperiness with its default, the per-family motion constants, and the trigonometry table if this version still uses one. **If it does not use a lookup table**, that is a schema question — record the table as absent and let the profile compute, and say so in the notes from Task 1, because a profile computing a sine is exactly the last-place-divergence risk the table was dumped to avoid.

The constants must be captured at their real width. The 1.8.9 dumper reached most values reflectively and needed twelve transcribed by hand because they are literals inside method bodies. Expect the same division here, and expect the split to fall differently.

- [x] **Step 1: Write the failing test**

Cover: `Dump` for an unsupported version still fails with a message naming the version; `Dump` for 26.1.2 without a prepared jar fails with a clear error rather than a panic; and with a prepared jar, the output validates against the schema and contains a non-empty slipperiness index and motion constants for the player.

- [x] **Step 2: Write the dumper and run it**

- [x] **Step 3: Verify each transcribed constant twice**

Any value a reflective dumper cannot reach is transcribed, and M8.1's rule applies: confirm it from two independent readings before committing it. Record both in the reference notes.

- [x] **Step 4 to 6: Verify, check, commit**

```bash
cd ../minecraft-reference
git add internal/reference/physics/
git commit -m "feat(physics): add a 26.1.2 physics dumper"
```

**Done.** `minecraft-reference` commits `b7b6003` and `443d99d`. The dumper runs
against the prepared jar and produces 1,168 blocks, the 65,536-entry
trigonometry table, and a default slipperiness of 0.6. Constants and findings
are in that repository's `reference/notes/physics-motion-26.1.2.md`, confirmed
twice as M8.1 requires — once from decompiled source and once from `javap`.

Three things Task 3 inherits:

- **The trigonometry table is bit for bit identical to 1.8.9's.** The dataset
  still carries its own copy, because a shared table would be a claim about
  every future version rather than a measurement of this one, but the profile
  can expect the same numbers. What changed is the index: a `double` multiplier
  through a `long` rather than a `float` multiplier through an `int`.
- **Only five blocks differ from the default slipperiness**, and the one 1.8.9
  calls `slime` is `slime_block` here.
- **The workspace has no `compatibility.json` for 26.1.2**, because it was
  prepared before `mcreference` began writing one. `Dump` reads that report to
  choose between `named.jar` and `executable.jar`, so generating the dataset
  needs `task reference:prepare` re-run for this version first. The dumper
  program itself is verified: it was compiled and run against the jar directly.

---

## Task 3: Commit and generate the 26.1.2 dataset

**Repository:** `minecraft-protocol`

**Files:**
- Create: `source/java/26.1/physics.json`
- Modify: `manifest.json`, `THIRD_PARTY_NOTICES.md`
- Generated: `generated/java/v26_1/physics.go`

M8.1 left this parameterized, so this task is mechanical: commit the extracted JSON, add the manifest entry with its digest and Mojang provenance, regenerate, and confirm `generate:check` passes with neither `java` nor `javac` on `PATH`.

The notices entry states Mojang provenance rather than inheriting the PrismarineJS attribution, the same way 1.8.9's does.

- [x] **Step 1: Commit the source, add the manifest entry, regenerate**

- [x] **Step 2: Confirm no-JDK generation**

Run the repository's `generate:check` with `java` and `javac` removed from `PATH`. Both versions must render from pinned source.

- [x] **Step 3: Confirm the widths survived**

Add a test asserting the player's constants match the extracted file exactly, including any value whose decimal form is not round. A generator that formatted a `float`-derived constant to fewer digits would lose exactly what M8.2 proved matters, and the round-trip test is what catches it.

- [x] **Step 4 to 5: Verify, commit**

```bash
cd ../minecraft-protocol
git add source/java/26.1/physics.json generated/java/v26_1/physics.go manifest.json THIRD_PARTY_NOTICES.md
git commit -m "feat(data): pin and generate Java 26.1.2 physics constants"
```

**Done 2026-08-18**, as `minecraft-protocol` `5700384`. Four things this task
answered that it was not asked:

- **The blocker this plan recorded was stale.** Task 2 said the workspace has no
  compatibility report for 26.1.2 and that `task reference:prepare` would have to
  be re-run before the dataset could be generated. `minecraft-reference`'s own
  workspace has one, so the dump ran as it stood.
- **Both extractions reproduce.** The physics dumper produces identical bytes on
  two runs, and re-extracting the block measurement at the current tool revision
  reproduces the committed file exactly — which is what makes it honest to move
  the manifest's single `toolRevision` forward to cover both datasets.
- **The trigonometry table is byte-identical to 1.8.9's**, checked here rather
  than taken from the note, and stored again rather than shared. An identical
  measurement of two versions is a fact about both; one shared table would be a
  claim about every version after them.
- **Five blocks differ from the default friction here where three do in 1.8.9**,
  and `slime` is `slime_block`. Both the count and the names are pinned by a
  test, because a table carried over by name would have walked the modern block
  on ordinary friction.

**The release Task 4 was blocked on landed as `v0.5.0`**, on 2026-08-18 and
with the maintainer's say-so. `minecraft-simulation` consumes it as a released
module with no `replace` directive, and every committed 1.8.9 recording still
verifies against it, which is what says the shared dataset did not move under the
older profile.

---

## Task 4a: The collision variant, done before the phases that call it

**Status:** done. It was Task 1's finding that this is a prerequisite rather
than a contingency, and it landed as `collision.ResolveVoxel`, `geom.Shape.GridY`,
and `movement.StepWith`, with `internal/oracle/java/ShapeOracle26.java` checking
each piece against a real 26.1.2 server.

What it settled, beyond the code:

- The axis order is Y and then the larger horizontal axis, so it depends on the
  motion. Every comparison works to a tolerance where 1.8.9 compares exactly.
  The step-up tries whatever heights the obstacle offers, ascending, and takes
  the first that beats the flat move.
- **A shape is a grid, not a list of boxes.** The game snaps a block-local box
  to a power-of-two grid up to eighths and stores which cells are filled, and the
  step-up collects its candidate heights from every line of that grid — so a
  plate an eighth thick offers eight heights, seven of them empty air. A box that
  lands on no grid line keeps its own two faces instead. This is a fact about the
  data model, not about the algorithm, and no reading of the movement code would
  have produced it.
- The horizontal collision flags forgive a shortfall under a hundred-thousandth;
  the vertical flag still compares exactly.

**The step-up assembly is covered too, as of 2026-08-18.** This paragraph used
to say it was not: that building the grounded box, choosing the first improving
candidate, and subtracting the drop were written from the version's own code and
checked only by this module's own tests, because driving `Entity.collide` needs a
Level and a Level was Task 6's job. It turned out not to need Task 6.
`internal/oracle/java/MoveOracle26.java` stands one up and runs the whole
`Entity.collide` against it, and `TestTheWholeCollideMatchesTheGame` compares
2,400 moves bit for bit — a fifth of them stepping.

Three things that cost less than the estimate said, and two the check found:

- **A level does not have to be constructed to be used.** Its constructor wants a
  writable level record, a layered registry access, and a dimension type, and
  eagerly builds a damage-source table from them, so an honest one means loading
  the vanilla data pack. The instance is allocated without a constructor instead,
  which is sound because the whole of what a move asks a level for is entity
  colliders, the world border, and the blocks in a chunk. Every other duty throws,
  so a version that starts asking for something else fails loudly rather than
  being answered with a default.
- **The blocks come from the game, not from the harness.** Scenes are placed by
  registry name and the harness reports each block's shape, so this side builds
  its world from the same answer. A committed table of shapes would have been a
  second transcription to keep true.
- **The shape model round-trips.** `TestTheBlockShapeGridMatchesTheGame` checks
  that a shape rebuilt here from the boxes the game stores offers the game's own
  rises, for six blocks that between them cover the grid at halves, quarters, and
  eighths and the fallback to a box's own faces. It is the premise the whole-move
  comparison rests on, so it is asserted rather than assumed.
- **Found: the sweep gathered per cell where the game gathers per shape.** A
  block whose shape the probe does not reach still contributed its rises, because
  `Gather` collected from every cell the region's bounds touched. That made a body
  floating above an enchanting table step onto it. Fixed by gathering a shape only
  where it overlaps the region in a volume, which is the game's own rule; the
  shape is still collected whole, one collider with every box in it, which is also
  the game's.
- **Found, on both versions: a face is not a collider.** The same defect reported
  a horizontal collision for a body flush against a wall moving into it by less
  than its own coordinates can hold. A 1.8.9 server settled it —
  `TestAFlushFaceIsNotACollider` is that case, and it takes a motion of 1e-18
  because any larger motion carries the sweep past the face and makes the wall a
  collider honestly. One determinism recording changed with the fix, at the single
  tick where a fall carried a 2e-18 motion into a face.

What is still not covered here: the flags. `Entity.move` combines the two
horizontal shortfalls into one boolean and derives `onGround` from the vertical
one, and reaching those means driving `move` rather than `collide` — which wants a
profiler, fall damage, block callbacks, and the sounds this level does not have.
The rules are three lines transcribed from that method, and the tolerance they
compare with is `Mth.equal`'s.

**One divergence is known and not fixed: a body that starts inside a shape.** The
game's clamp works on a shape's grid, scanning from the cell after the one the
moving face is in, so a body already inside a multi-cell shape is stopped at that
shape's next interior grid line. This module clamps against a shape's boxes,
where a face already past a box does not stop at all. The case that found it: a
body at `x 0.58..1.18, y 1.349..3.149, z -0.3..0.3` moving
`(0.302, -0.0205, 0.503)` with stairs at `(0, 1, 0)` is stopped at `z = 0.2` by
the game and not at all here, because the stair's z grid has a line at a half
that the body straddles. Closing it means a discrete voxel grid in `geom` and a
clamp that walks it, which is a task rather than a fix, and it is the last piece
of "a shape is a grid, not a list of boxes" that this milestone left as boxes. It
is owned by M8.7 and needs a task of its own; until then
`TestTheWholeCollideMatchesTheGame` keeps its bodies out of the blocks they stand
against, so a failure there means the assembly.

---

## Task 4: The v26_1 block table

**Files:** `profile/java/v26_1/blocks.go` and its test

The same job as M8.4's block table, against this version's block model. The 1.13 flattening means block identity is a state id rather than an id and metadata pair, so the handle mapping is simpler here and the `ref(name, meta)` helper M8.4 needed for fixtures becomes `ref(name, properties)` or `ref(stateID)`, depending on what `minecraft-protocol` exposes for this version.

Slipperiness narrows to `float32` at this boundary if and only if Task 1 found it to be a `float` in this version. If it is a `double`, keep it `float64` and say so in a comment, because the asymmetry with the 1.8.9 table will otherwise read as a mistake.

- [x] **Steps: as M8.4 Task 6, against this version's data**

```bash
git add profile/java/v26_1/blocks.go profile/java/v26_1/blocks_test.go
git commit -m "feat(profile): map 26.1.2 block data to simulation handles"
```

**Done 2026-08-18**, as `c3db3c2`. The flattening decided the shape of the
table, and it is not the shape this task expected:

- **A handle is a state, and only the shape belongs to it.** The dataset
  describes 1,168 blocks across 29,873 states with 5,128 distinct shapes. The
  shape is the state's — a slab's two halves and a stair's eighty orientations
  are states of one block and they do not stand in the same volume — while the
  friction is the block's, so it is stored once per block and every state of it
  answers the same. So the handle is the state identifier plus one, which is also
  the number this version's protocol carries.
- **The shape list is one per block or one per state, and the table refuses
  anything else.** 410 blocks name a single shape for every state they have and
  758 name one apiece; a list of any other length would be a state-to-shape
  mapping the table would have to guess at, so it is an error instead.
- **The state numbering has to be whole.** An overlap or a hole would leave a
  handle answering with an empty shape and the default friction, which is a
  description of air, and the cell it was read from is not air. The constructor
  checks both. This is also what makes the table refuse a 1.8.9-shaped dataset,
  where every block's state span is zero.
- **There is no way to name a state by its properties.** `minecraft-protocol`
  publishes which properties a block varies over and not which state a
  combination of them lands on, so `ref(name)` answers with the block's default
  state and `refState(id)` is the way in for a world arriving from a server.
  Task 6's fixtures can name blocks; a fixture wanting a stair facing east needs
  the state number until something resolves properties.
- **Slipperiness is a `float` here as it is in 1.8.9**, per Task 1's width table,
  and this dataset stores it as the round decimal the way 1.8.9's does — so the
  narrowing at the boundary recovers the width the game computes at rather than
  losing one. The asymmetry with the step height, which the same dataset stores
  already widened at `0.6000000238418579`, is pinned by its own test here as it
  is there.
- **Five blocks differ from the default friction and one is renamed.** Blue ice
  is `0.989`, which is neither the default nor the `0.98` the other three ices
  carry, and 1.8.9's `slime` is `slime_block`. A test asserts that `slime`
  resolves to nothing, because a fixture carrying the old name over would place
  air and say nothing about it.
- **The shapes feed Task 4a's voxel step-up, not just the sweep.** A test takes
  the default oak slab's `GridY` and asserts it offers zero, a half, and one,
  which is the grid the step-up asks a shape for.

Two things Task 5 inherits: `ErrInvalidProfile` lives in `blocks.go` for now,
because the package cannot have a `profile.go` yet; and the block speed factor
of statement 9 needs a second table — soul sand and honey carry the default
friction here, and the slowing they do is a step in the tick rather than a
friction.

---

## Task 5: The v26_1 profile and its phase list

**Files:** `profile/java/v26_1/profile.go`, `phases.go`, and their tests

Identity is `java/26.1.2@1`. The phase list is this version's, in this version's order, with namespaced identifiers prefixed `v26_1.`.

**Do not copy M8.4's eleven phases and adjust constants.** Establish the order from the version's own movement method, the way M8.4's was established, and let it come out however it comes out. If it happens to match, that is a finding worth a comment; if the plan assumed it and it does not, the trajectories will be wrong in ways fixtures generated from the same assumption will not catch.

The known structural differences to expect, from the parent design: the 1.13 collision rework, and additional movement states. The collision rework matters most, because `collision` is M8.2's package and reproduces 1.8.9 exactly, including the deliberate absence of an epsilon that later versions added. **This is the milestone's second real risk after the oracle**: if 26.1.2's collision differs, this profile cannot reuse `collision.Resolve` unchanged, and the fix is a version-selected variant behind the same interface rather than an epsilon parameter smuggled into the 1.8.9 path.

Determine this early — it is cheap to check and expensive to discover late — and if a variant is needed, add it as its own task before the phases, with the 1.8.9 recordings proving the original path unchanged.

- [x] **Step 1: Establish the tick order and record it in the plan's own terms**

Write the numbered behavioural statements for this version, the way M8.4's plan did, into the profile package's doc comment. Prose in a doc comment is the artifact that survives; prose in a session does not.

**Done 2026-08-18**, read from the version's own land-movement path rather than
from M8.4's list. It is recorded here rather than in the package doc comment
because the package cannot exist until the constants reach a release, and the
statements are what the package will be written from. They move into
`profile/java/v26_1/profile.go` when it lands, unchanged unless the game
disagrees with one.

Scope is the player on land: no fluid, no elytra, no climbing, no levitation, no
riding. Each statement says what 1.8.9 does where the two differ, because the
differences are what a profile written by analogy would get wrong.

1. **The jump delay counts down**, if it is above zero. The same rule and the
   same counter as 1.8.9's.
2. **The motion threshold is a player rule in this version, and a vector one.**
   For a player, the two horizontal components are zeroed *together* when the
   horizontal motion's squared length is below `9.0E-6` — that is, when the
   motion is shorter than 0.003 as a vector. Every other entity tests each
   horizontal axis separately against 0.003. The vertical is tested on its own
   against 0.003 either way. 1.8.9 tests all three axes separately against 0.005,
   so this version discards a smaller motion and discards it as a vector.
3. **Both input axes decay by `0.98F`, before the jump rather than after it.**
   1.8.9 decays after. The jump reads neither axis, so the two orders produce the
   same numbers; the order is recorded because a phase list should follow the
   version rather than the analogy, and because a later mechanic that reads an
   input axis during the jump would make the difference real.
4. **The jump, when the body is standing and the delay is zero.** Its power is
   the jump-strength attribute — `0.42` for a player with no modifiers — times
   the block jump factor, plus the jump-boost bonus. A power of `1.0E-5F` or
   less does nothing at all. Otherwise the vertical motion becomes **the larger
   of** the jump power and the motion it already had, where 1.8.9 assigns
   `0.42` over whatever was there; a sprinting body then gains `0.2` along its
   facing from the float sine and cosine; and the delay is set to 10 ticks.
5. **The friction of the block below gives two different numbers, and this is
   the formula difference that matters most.** The block friction is the block's
   own when the body is on the ground and `1.0F` when it is not. The tick's
   horizontal drag is `blockFriction * 0.91F` at single width, as in 1.8.9. The
   acceleration is the movement speed times `0.21600002F / blockFriction³` — the
   **raw block friction**, cubed. 1.8.9 divides `0.16277136F` by the cube of the
   *product*. On stone the two denominators differ by a factor of `0.91³`, and
   the numerator changed to match, so a profile that ported the 1.8.9 rule and
   swapped the constant would be wrong on every surface that is not stone.
6. **"The block below" is the block the body is standing on, not the block under
   its feet.** It comes from the supporting block the last collision recorded,
   read half a block down, with fences, walls, and fence gates answering with the
   supporting position itself rather than the offset one. 1.8.9 takes the cell at
   `floor(x)`, `floor(box.minY) - 1`, `floor(z)` and keeps no such record. A body
   standing on the edge of a block can therefore disagree between the versions
   about which block it is standing on.
7. **The input becomes motion at double width.** The two input axes and the
   vertical axis form a vector; a squared length below `1.0E-7` contributes
   nothing; a squared length above `1` is normalized. The result scales by the
   acceleration from (5) and rotates by the body's yaw. Only the sine and cosine
   are single width, taken from the table by an index this version computes with
   a `double` multiplier through a `long`. 1.8.9's counterpart is float
   throughout, thresholds at `1.0E-4F`, and indexes the same table with a float
   multiplier through an `int`. The table itself is byte-identical between the
   two versions, which is measured rather than assumed.
8. **The move** resolves the motion against the world through this version's
   shape-based collision — Task 4a's `collision.ResolveVoxel` — which reports the
   applied motion and the collision flags.
9. **A block speed factor multiplies the two horizontal components after the
   move.** It comes from the block at the body's position, falls back to the
   block below when that one is neutral, and is interpolated toward `1` by the
   movement-efficiency attribute. Soul sand and honey are what it exists for.
   1.8.9 has no such step in the tick: it slows a body from inside the block's
   own collision callback instead, which is a different place in the order and a
   different set of blocks.
10. **Gravity is subtracted from the vertical motion as a double**: the gravity
    attribute, `0.08` for a player, or at most `0.01` while falling with slow
    falling. 1.8.9 subtracts a literal.
11. **The drags apply after gravity**, as in 1.8.9: the vertical motion times
    `0.98F` and each horizontal times the tick's friction from (5). Both
    constants are floats widened against a double motion, so the products form at
    double width.

Two further facts that are not steps in the order:

- **The movement speed a tick moves with is the one the previous tick left.**
  The player's tick reads the movement-speed attribute into the field that
  drives (5) *after* the travel that used it. 1.8.9 does the same thing in the
  same place, so this is a shared fact rather than a difference — but it decides
  what a sprint scenario looks like on the tick sprinting starts, and both
  profiles take the speed as an input rather than reading an attribute.
- **What is deliberately not in this list**: swimming, elytra, climbing,
  levitation, powder snow, and riding all branch before or inside the land path
  and are out of this milestone's scope. The phase list must leave room for them
  rather than pretend the branch is not there.

- [x] **Step 2: Check whether collision differs, and branch**

Answered ahead of this task by Task 4a: it differs, the variant landed as
`collision.ResolveVoxel`, and the whole of it — including the step-up assembly —
is checked against a real 26.1.2 server.

- [x] **Step 3 to 7: Test, implement, verify, commit**

```bash
git add profile/java/v26_1/
git commit -m "feat(profile): add the Java 26.1.2 player profile and its tick phases"
```

**Done 2026-08-18**, as `5dadc97`. The tick came out as thirteen phases rather
than the eleven statements, and three of the differences were found by the game
rather than by the reading:

- **Two phases have no counterpart in 1.8.9's list**: the input shaping and the
  block speed factor. The second is in the order and answers 1 for every block,
  because the dataset publishes slipperiness and no other movement property —
  soul sand, honey, slime, and beds are ordinary here, and filling them in is a
  dataset change rather than a reordering.
- **Statement 3 was understated.** The shared entity tick decays both axes by
  0.98F, but a client's own player replaces that with a decay, a sneaking-speed
  factor, and a stretch of a diagonal onto the unit square — one method, and its
  clamp discards the decay for any input that reaches it. So a keyboard diagonal
  walks at the full input rather than at 0.98 of it, and the three steps cannot
  be split across the tick boundary. It is `ShapeInput`, and it is the one rule
  here no jar-backed test covers as a rule, because it lives in a class the
  server jar does not carry.
- **The box is rebuilt around the position, not offset.** This version moves the
  position and derives the box from it and the body's dimensions; 1.8.9 moves the
  box and derives the position. The two orders round differently, and the oracle
  found it on the first tick of the first scenario. `entity.State` carries a
  position for it.
- **The supporting block is remembered, not derived.** Statement 6 said the block
  below comes from "the supporting block the last collision recorded", and the
  implementation recomputed it from the body's own box on the assumption that
  nothing moves in between. It does not hold: a body that walks off the edge of a
  slab is still standing, with nothing under it, and the game then re-probes
  where it came from and keeps *that* block's column for a tick. Found at tick 67
  of a sprint, as a body that kept ice friction where this module had switched to
  stone. `entity.Support` carries it.

Both additions to `entity.State` are written to a tick digest only when they are
set, so every recording made before this milestone still verifies unchanged.

---

## Task 6: The gate

**Files:** `internal/oracle/java/MovementOracle26.java`, `internal/oracle/movement26_test.go`, `mctest/testdata/26_1/*.json`

Which of these exist depends on Task 1.

**If a jar-backed oracle is possible:** mirror M8.4 Task 8 and Task 9. Differential test over the six scenarios, comparing every tick bit for bit; then generate the fixtures from the harness; then confirm they replay with no JDK.

**If it is not:** build the fixtures from the behavioural statements in Task 5, and mark every one of them as unverified against the game in the fixture file itself, with a field that says so. Then M8.8's live check against a 26.1.2 server is the first real verification, and the master plan must say that this milestone's exit criterion is weaker than M8.4's. A fixture that records a belief and does not say so is the thing that makes a later failure hard to diagnose.

- [x] **Step 1: Build whichever gate Task 1 permits**

The strong one. `internal/oracle/java/MovementOracle26.java` drives a whole tick
through `LivingEntity.aiStep` on a real 26.1.2 server, and
`TestMovementMatchesTheGame26` compares 4,800 ticks — six scenarios over eight
random worlds each, a hundred ticks apiece — position, motion, and the collision
flags, every tick rather than at the end.

The body is a living entity of the player type rather than a `Player`. The type
is what the tick branches on and what carries the attribute supplier; the class
adds an inventory, an entity pickup that needs a populated level, and a re-read
of the speed attribute — and a server's `Player` is client-authoritative and does
not travel at all, so the tick under test is the one a client runs for its own
body. Four overrides remove what is not movement, one restores the player's
airborne speed, and the file says which is which.

`mctest/testdata/26_1` then carries the same six scenarios as fixtures that
replay with no JDK, generated from the harness rather than from this module.

- [x] **Step 2: Add recordings to the M8.6 matrix**

Four recordings, 220 to 260 ticks each: a sprinting diagonal through a full turn
of yaw, a walk over three different ices, a hundred and thirty blocks of falling
into repeated jumps, and a walk through a field of slabs and stairs. They are not
the other version's five with a different profile — each reaches arithmetic this
version has and 1.8.9 does not.

There is no slime-and-soul-sand recording here, because both blocks act through
per-block behaviour this profile cannot express yet, and a recording of them
would pin an assumption rather than the game's arithmetic.

The matrix needed no workflow change: the determinism task selects by test-name
prefix, and the second version's test shares it.

- [x] **Step 3: Confirm 1.8.9 is untouched**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task determinism`

Every 1.8.9 recording must still verify. If a shared extraction changed one, revert the extraction.

- [x] **Step 4 to 5: Verify, commit**

```bash
git add internal/oracle/ mctest/testdata/26_1/ replay/testdata/26_1/
git commit -m "test(profile): gate the 26.1.2 player on the scenario suite"
```

**Done 2026-08-18**, as `354b5db`. Every 1.8.9 recording still verifies, which is
what says the two additions to the shared state and the digest changed nothing
for the older profile.

---

## Task 7: Extract only what both versions share, and record the milestone

**Files:** `movement/shared.go`, `README.md`, `CHANGELOG.md`

- [x] **Step 1: Extract, one function at a time**

For each rule that is identical in both profiles — same constants, same widths, same position in the order — move it to `movement` and have both call it. After each extraction, run the determinism task. An extraction that changes any committed digest, of either version, is reverted rather than debugged, because a shared function that alters behaviour is not shared.

- [x] **Step 2: Record what stayed duplicated, and why**

This is the milestone's most useful documentation. Two versions of a rule that look similar and are not identical are the trap every later mechanic will walk into, and the list of them is worth more than the abstraction that would have hidden them.

- [x] **Step 3: Changelog, verify, commit**

```bash
git add movement/shared.go README.md CHANGELOG.md
git commit -m "docs: record M8.7, and what the two profiles actually share"
```

**Done 2026-08-18**, as `bbd6afc`. The extraction is one function — `movement.Box`
— and it landed in `movement/box.go` rather than the `shared.go` this plan named,
because a file called shared is an invitation to put the next similar-looking
rule in it.

Three things are shared: that box construction, the four rules that were already
shared when this milestone started (the jump countdown, gravity, and the two
drags), and the trigonometry table's read. Nine are duplicated on purpose, and
the README carries the table of which and why. The table is the deliverable: M9's
mechanics land on both profiles from the start, and it is what tells each of them
whether it is writing one implementation or two.

---

## Definition of done

- The dumper supports 26.1.2, still rejects unknown versions explicitly, and every transcribed constant was confirmed twice.
- `source/java/26.1/physics.json` is committed and digest-pinned with Mojang provenance, `generated/java/v26_1/physics.go` renders from it, and `generate:check` passes with no JDK.
- `profile/java/v26_1` supplies this version's constants, block table, and phase order, established from the version's own movement method and written down in its doc comment.
- Whether 26.1.2's collision differs from 1.8.9's is answered, and if it does, the variant is version-selected rather than added to the 1.8.9 path.
- **The M8.4 scenario suite passes on 26.1.2** — walk, sprint, jump, sneak, fall, collide — against the strongest gate Task 1 found available, with the strength of that gate stated plainly in the master plan.
- 26.1.2 recordings join the M8.6 matrix and pass on every target, and every 1.8.9 recording still verifies unchanged.
- Every rule shared between the profiles is provably identical; every rule that is not is duplicated on purpose and listed.
- `devbox run -- task verify` passes in all three repositories.

## Follow-on

M8.8 runs both profiles through the client and server adapters, and its live gate now has two versions to satisfy. If Task 1 found no jar-backed oracle for this version, M8.8's 26.1.2 lane is the first real verification of these constants and should be scheduled with that in mind rather than as a formality.

M9's mechanics land on both profiles from the start, and the list of deliberately duplicated rules from Task 7 is what tells each of those milestones whether it is writing one implementation or two.
