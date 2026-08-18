# minecraft-simulation

Protocol-independent Minecraft simulation for Go clients and servers.

The module will provide deterministic game-state transitions that server and
client applications can share. Protocol codecs, packet IDs, networking,
persistence, rendering, and AI remain outside this repository.

## Packages

| Package | Responsibility |
| --- | --- |
| `geom` | Vectors, block positions, axis-aligned boxes, and per-block voxel shapes |
| `world` | The tri-state block view and a deterministic in-memory implementation |
| `collision` | Swept candidate gathering and vanilla-order axis resolution with step-up |
| `terrain` | Static predicates over a world view: fit, ground, passability, hazards, fluids |
| `navigation` | The edge vocabulary, a body's capability, a bounded deterministic route search, and a planner that caches terrain across searches |
| `entity` | Entity identity, physics families, and the body state a tick moves |
| `sim` | The tick contract: input, result, canonical digest, profile, phases, kernel |
| `movement` | The movement rules a profile's phases call: friction, heading, jump, gravity, drags |
| `runtime` | The store, its revision check, and a runner that drives one tick after another |
| `adapter` | The seam a consumer implements to drive one kernel, and the tick assembly they share |
| `profile/java/v1_8` | Java Edition 1.8.9: the constants, the block table, the widths, and the tick's phase order |
| `profile/java/v26_1` | Java Edition 26.1.2: the same four things, and not one of them the same |
| `scene` | A world described by name: a filled region and the blocks in it |
| `mctest` | Recorded trajectories and a replay that needs no jar |
| `replay` | Recording a run as inputs and per-tick digests, and replaying it |

Packages depend in one direction only:

```text
geom  ->  world  ->  entity  ->  movement  ->  sim  ->  runtime  ->  adapter
              \                     ^
               \-> collision -------/

profile/java/v1_8  ->  sim, movement, and one version's game data
profile/java/v26_1 ->  sim, movement, collision, and another version's game data
scene              ->  sim, world, geom
mctest, replay     ->  sim, runtime, movement, scene
terrain            ->  geom, world, collision
navigation         ->  terrain, geom, world
```

`terrain` and `navigation` import `geom`, `world`, and `collision` and nothing
else. Neither imports `sim`, so a version's numbers reach them as arguments the
same way they reach `movement` — a body's width and step height on a value, a
block's hazard through an oracle the profile supplies. That is what lets one
search serve a 1.8.9 mob and a 26.1.2 bot.

The two profile packages are the only ones here that import game data. Everything
below them is version neutral: a rule that needs a 1.8.9 number receives it as an
argument, which is what lets 26.1.2 reuse the same rules while disagreeing about
almost every constant.

The `Profile` interface lives in `sim` rather than in a `profile` package of its
own. A profile supplies the kernel's tick phases and a phase is written against
the kernel's own tick state, so a separate package holding the interface would
have to import `sim` while `sim` needs the interface — a cycle. Concrete
profiles are separate packages and may import whatever their version's data
needs.

A tick reads only what its input carries: no clock, no global random state, no
mutable application object. Its result carries a canonical digest over every
field, and the change set it produces names the store revision it was computed
against, so a store that has moved on refuses it whole. That is what lets a
client apply a prediction to a forked snapshot and discard the fork when the
server disagrees.

`geom`, `world`, and `collision` import nothing outside the standard library.
They know nothing about entities, profiles, or the protocol: a caller supplies
a box, a motion, and a view, and receives the motion that actually applied.

Collision reproduces Java Edition 1.8.9: candidates are gathered once from the
swept region, motion resolves along Y then X then Z with the body translated
after each axis, and a blocked horizontal move retries with a step-up whose
winner is the outcome that travels further in the horizontal plane.

It also reproduces 26.1.2's, which is a different algorithm rather than the same
one with different numbers. `ResolveVoxel` orders the axes by the motion rather
than fixing them, compares to a tolerance where 1.8.9 compares exactly, and
steps up over whatever heights the obstacle's own shape offers — because a shape
in that version is a grid of filled cells rather than a list of boxes, so a plate
an eighth thick offers eight rises and seven of them are empty air. A profile
picks the resolver its version plays by.

## Float widths are part of the rules

Java Edition computes some quantities as `float` and others as `double`, and the
difference is visible: a product formed at single width and one formed at double
width disagree in their last bits, and a hundred ticks of that is a position a
server corrects.

So a `float32` in a signature here is a statement about the game, not an
optimization, and it is never widened early. The rule is:

- Every value the game holds as a `float` is a `float32` here, and every product
  whose operands are all `float` in the game is formed at single width.
- A product that mixes the two — a `float` constant against a `double` motion —
  is formed at double width, because that is what Java does when it widens the
  constant to meet the motion. Both drags are this shape.
- The widening happens once, where the game widens it, and nowhere else.
- `float32` is confined to `profile/java/v1_8` and to the rules in `movement`
  that the profile hands single-width constants to. `geom`, `world`, `collision`,
  `entity`, `sim`, and `runtime` see `float64` only.

Four of the six things the movement oracle caught were width or expression
mistakes that every test written from prose had passed. A later contributor who
sees a `float32` and assumes it is a slip will reintroduce them.

## Checking against the game

`internal/oracle` compares this module against a real Java Edition 1.8.9
server. It compiles a small harness against the locally prepared, deobfuscated
server jar and runs the game's own code: the `AxisAlignedBB` methods, the whole
of `Entity.moveEntity` — candidate gathering, axis passes, the two step-up
attempts, and the settle — and a whole movement tick through
`EntityLivingBase.onLivingUpdate`, which is the jump counter, the motion
threshold, the input decay, the friction lookup, the heading, the move, gravity,
and the two drags. The results must be bit-identical to ours, every tick.

The same is done for 26.1.2, against that version's own server. Mojang ships it
unobfuscated, so the harness compiles against the shipped jar and javac checks it
against the bytecode it will run on: a renamed method fails to compile rather
than throwing halfway through a run. That harness stands up a level without
running its constructor — every duty a move does not need throws, so a version
that starts asking for something else fails loudly — and drives `Entity.collide`,
the block-shape grid the step-up collects its rises from, and a whole tick
through `LivingEntity.aiStep`.

The movement gate runs six scenarios — walk, sprint, jump, sneak, fall, and
collide — over eight randomly obstructed rooms each, a hundred ticks apiece, per
version, and compares position, motion, and the collision flags at every tick
rather than at the end: the first differing tick names the rule that drifted,
where a final position names only the scenario. 4,800 ticks agree with each
version's server.

The harness supplies a block lookup, a minimal entity, and a text protocol, and
it reimplements no game logic. It lives in `internal/oracle/java` and is
committed, so anyone with a prepared workspace can re-run these gates rather
than take their results on trust. What is not committed is the game: no jar, no
mappings, and no decompiled source. The tests therefore skip when the workspace,
`javac`, or `java` is absent; run `task reference:prepare` to make them run.

Six behaviours were found this way rather than by reading. Each had passed
every test written from a careful reading of the game:

- A step-up records the settle as its Y motion, not the climb plus the settle,
  which is why a step leaves the vertical collision flag describing the descent
  onto the surface. `onGround` follows from that flag and the tick's original
  downward motion, so stepping does not by itself put an entity on the ground.
- `stepHeight` is a `float` widened where it is applied, so a player steps with
  `float64(float32(0.6))`. Passing the round `0.6` moves the resulting box in
  its last bits.
- A player's box is not 0.6 wide and 1.8 tall. The game halves a `float` width
  and adds a `float` height to a `double` position, so the body reaches
  `0.30000001192092896` from its centre and stands `1.7999999523162842` tall.
- Both drags are `double` products. The constants are `float` and the motion is
  a `double`, so Java widens the constant to meet it. Narrowing the motion
  first is a different number, and it is wrong for a body doing nothing but
  standing on a floor.
- The heading converts degrees in two `float` steps — multiply by a `float` pi,
  then divide by 180 — while the jump impulse three rules away multiplies by a
  single pre-divided constant. They are different expressions in the game and
  they disagree, and at some yaws they read neighbouring entries of the sine
  table.
- The tick discards any component of motion whose magnitude is below `0.005`,
  before the jump. Nothing had described this rule, and without it a body
  walking at any angle other than square on diverges within four ticks.

Two divergences are known and recorded rather than fixed. The vertical clamp is
not a rule of the tick at all: the game calls the landing behaviour of the block
under the body's feet, whose default zeroes the vertical motion and whose slime
override negates it, bouncing the body. This module implements the default and
has no per-block landing hook, so slime is kept out of the differential worlds
and its slipperiness is unchecked against the game. And the sneak scale is a
transform the 1.8.9 client applies to its own input before it sends it, so the
server entity the harness drives never sees it; it is folded into the profile at
the width the client computes it, and it is indistinguishable from the
single-width product for the axes a keyboard actually produces.

## What the two versions share, and what they only look like they share

Two profiles ship here, and the list of rules they share is short. It is short
because sharing was allowed only where both versions were measured doing the same
arithmetic, never where two rules looked alike:

- **`movement.Box`** builds a body's collision box from a position and two
  dimensions. Both versions halve the width at single width and add the halves to
  a double position, so a body 0.6 wide stands in a box a sixteenth of a
  millionth wider than that. Each version's oracle caught this independently.
- **`movement.Countdown`, `ApplyGravity`, `ApplyVerticalDrag`,
  `ApplyHorizontalDrag`** are the same rule and the same widths in both. The
  constants they multiply by are not: they arrive from each version's dataset.
- **The trigonometry table** is byte-identical between the two, which is a
  measurement of both rather than a claim about either. Each version stores its
  own copy and computes its own index; `Table.At` is the shared read.

Everything else in the two tick lists is duplicated on purpose. Each of these
looks shareable and is not:

| Rule | Why it is two implementations |
| --- | --- |
| The motion threshold | 1.8.9 tests each axis against 0.005; 26.1.2 tests the horizontal *vector* against 0.003 and only for a player |
| The input handling | 1.8.9's client scales for sneaking and the shared tick then decays; 26.1.2's client decays, scales, and stretches the input onto the unit square in one method, and the clamp at the end of it discards the decay |
| The jump | 1.8.9 assigns 0.42 over the vertical motion; 26.1.2 takes the larger of an attribute-derived power and what was there, and adds a sprinting body's impulse through a double multiply where 1.8.9 uses a float |
| Friction and acceleration | The same tick drag falls out of both, but 26.1.2 divides its acceleration by the cube of the *raw* block friction and 1.8.9 by the cube of the product — and their numerators differ to match |
| The block underfoot | 1.8.9 takes the cell under the body's feet; 26.1.2 takes the column of the block its last move recorded as supporting it, read half a block down, which survives the body walking off an edge |
| The heading | 1.8.9 is float throughout and thresholds at 1e-4; 26.1.2 normalizes at double width, thresholds at 1e-7, and only its sine and cosine are single width |
| The table index | A float multiplier through an int against a double multiplier through a long |
| The move | Two collision algorithms, and two ways of committing one: 1.8.9 offsets the box, 26.1.2 moves the position and rebuilds the box around it |
| The tick's shape | 26.1.2 has two phases 1.8.9 does not — the input shaping and the block speed factor — and 1.8.9 decays its input after the jump where 26.1.2 shapes it before |

That table is the most useful thing this milestone produced. A later mechanic
landing on both profiles reads it to find out whether it is writing one
implementation or two.

## Replaying without a jar

`mctest` replays recorded trajectories against a profile, with no JDK, no jar,
and no prepared workspace. The fixtures in `mctest/testdata` are the six
scenarios above for 1.8.9 and, in `mctest/testdata/26_1`, the same six for
26.1.2. Every expectation in them is the game's own answer, recorded by the
oracle rather than by us: a fixture and the differential test cannot disagree
about what vanilla does, only about whether this module still matches it.

Regenerating them is deliberate and leaves a diff:

```bash
devbox run -- go test ./internal/oracle/ -run TestGenerateFixtures -args -write-fixtures
```

Without that flag the same test checks the committed fixtures still say what the
jar says, so a fixture that has drifted from the game is reported where the game
is present rather than surfacing later as an unexplained replay failure.

## The same answer on six platforms

`replay` records a run as its initial world, its per-tick input, and the digest
every tick produced, and replays it against a profile. A `Determinism` workflow
replays the committed recordings on six targets — Linux, macOS, and Windows,
each on amd64 and arm64 — and every one must produce the same digests. All six
run, and all six agree.

It found a real bug on its first run with real recordings. All three arm64
targets disagreed with both amd64 targets, on exactly three of the five
recordings: the three where strafe and forward are both non-zero. Go permits an
implementation to contract a multiply and an add into one operation with a
single rounding unless an explicit conversion rounds the intermediate, and on
arm64 the compiler takes that permission — so the two products in the heading
rule fused, and the runs with forward alone agreed because their other product
is zero.

That is a correctness bug rather than a tie between two defensible answers.
Java never contracts unless `Math.fma` is called, so the game computes each
product separately, and `internal/oracle` had already checked this module's
unfused arithmetic against a real server. An arm64 build was producing positions
vanilla does not. The fix is the conversion the Go specification names, and the
committed recordings did not change — which is the evidence that it forces the
answer amd64 already had rather than choosing a third one.

Two things about that gate are worth stating plainly, because both are easy to
read the wrong way.

**It tests agreement, not correctness.** These digests are this module's own
output. Whether they match Java Edition is what `internal/oracle` answers, and
it is a separate question with a separate gate. What six platforms agreeing
proves is that a compiler, an architecture, or a fused multiply-add is not
changing an answer underneath us — and so what the matrix actually exercises is
the `float32` arithmetic in `movement` and the truncated sine-table index, not
the canonical encoder. The encoder is integer and byte work and was never
plausibly platform-dependent. That is why the recordings sprint diagonally,
walk on ice, fall a hundred and twenty blocks, and climb slabs: a matrix over
empty ticks would pass everywhere and prove nothing, and a test asserts the
recordings stay that way.

**A recording is never regenerated to make a red matrix green.** Doing that
converts the gate into each platform being compared against itself. If a target
disagrees, the finding is real and the fix is in the code: an intermediate value
that was not a named `float32` and so was allowed to fuse, a `math` function
that is not exactly rounded there, or a genuine compiler difference. Regenerating
is for a deliberate change in behaviour, behind an explicit flag, in a commit
that says which behaviour changed:

```bash
devbox run -- go test ./replay/ -run TestGenerateRecordings -args -write-recordings
```

The determinism job is the only check here that does not run through Devbox.
Devbox provisions through Nix, Nix does not run natively on Windows, and Windows
is a third of what this gate claims — so verification keeps Devbox on one
platform and this job uses `actions/setup-go` on six. Two toolchains are
affordable; two Go versions are not, because the matrix would then be testing a
compiler nothing else uses, so `internal/buildcheck` fails if `devbox.json`, the
workflow, and `go.mod` stop naming the same one.

Locally, the same check with only a Go toolchain:

```bash
devbox run -- task determinism
```

Vanilla Java research uses the separate
[`minecraft-reference`](https://github.com/go-theft-craft/minecraft-reference)
command. This repository pins the command version through the
`MCREFERENCE_VERSION` Task variable. The default is `main` until the first
tagged tool release.

```bash
devbox run -- task reference:prepare \
  VERSIONS=1.8.9,26.1.2 \
  SIDES=client,server \
	REFERENCE_DIR=reference/work \
	MCREFERENCE_VERSION=main
```

Generated game artifacts remain ignored. See
[`reference/README.md`](reference/README.md) for the boundary.
