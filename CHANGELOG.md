# Changelog

This file records notable user-visible changes. It follows [Keep a Changelog](https://keepachangelog.com/en/2.0.0/) and [Semantic Versioning](https://semver.org/).

## Unreleased

## 0.4.0 - 2026-08-19

### Changed

- `navigation`: `Find` and `Planner.Plan` take a `Goal` — a heuristic and a
  completion test — instead of one exact cell. `GoalBlock` is the old
  behaviour under the new signature; `GoalXZ`, `GoalYLevel`, `GoalNear`,
  `GoalGetToBlock`, `GoalRunAway`, `GoalComposite`, `GoalInverted`, and
  `GoalAxis` are the shapes a destination actually takes. Callers wrap their
  cell in `GoalBlock{Pos: cell}` and behave exactly as before. A nil goal is
  `ErrNoGoal`; there is no default destination.

### Added

- `navigation`: `EdgeDig` — a body that carries a tool enters a blocked cell by
  breaking what is in the way, priced at the walk it replaces plus the break
  time of every cell of its span that is still filled. `Capability.Breaker` is
  the seam: an interface this package declares rather than a dependency on
  `mining`, because `mining` imports `sim` and nothing in `navigation` does. A
  caller closes over its held tool, its effects, and its version profile and
  answers per block handle; `mining.BreakTicks` is what it answers from.
  `Capability.DigBudget` bounds a route's holes. With `Breaker` nil — the
  default — the search is exactly what it was, to the allocation.

- `navigation`: `Capability.HazardPenalty` charges edges that arrive beside a
  hazard, in ticks. Zero — the default — is the search exactly as it was;
  a positive value makes a route one lane away from lava beat the rim walk.
  It is a penalty and not a refusal, so a body whose only route is the rim
  still takes it.

## 0.3.0 - 2026-08-18

### Added

- `combat`: what one swing does, gated on both jars. `InReach` measures to the
  nearest point of the target's box — measuring to the centre makes a tall
  entity unhittable at its feet — and each profile declares its own attack and
  interact distances: 1.8.9's are the client's, transcribed from
  `EntityRenderer.getMouseOver` and `PlayerControllerMP` because the server's
  6-block tolerance is slack a vanilla client never uses, and 26.1.2's come
  from the generated interaction-range attributes plus the two creative
  modifiers `ServerPlayer` adds.

- `combat.Cooldown`: the attack cooldown, and 1.8.9's absence of one as a
  value with a required reason rather than a nil — so shared damage code needs
  no version branch and a conformance report can tell "verified absent" from
  "never checked". The 26.1.2 charge curve runs at float32 width, as the
  game's does, and matches its jar exactly at every sampled tick.

- `combat.Damage` and `combat.Knockback`: one formula serves both versions
  because 1.8.9's charge is always 1, at which 26.1.2's scale factor is
  exactly 1 in float32. Knockback is an impulse the next movement tick
  resolves through collision, never a position write, at the games' own float
  widths — the distance is `MathHelper.sqrt_double`'s float widened, and the
  0.4 constants are Java floats, not 0.4. Zero distance does not produce the
  NaN the kernel would reject.

- `combat.Attack` and `combat.Phase`: the kernel phase for reach, cooldown,
  damage, knockback, and death. One death event per death, and the body goes
  with it; an out-of-reach attack is refused with a reason and no change set;
  an attack naming a body the tick cannot see reports an incomplete tick
  rather than a refusal.

- `entity.Vitals`: a body's health and the tick of its last swing, encoded
  under an appended digest tag only when non-zero — so every recording made
  before combat existed still verifies.

- `sim.TickState.MissingEntities`: the entity counterpart of `MissingBlocks`,
  for a command that names a body the tick cannot read.

- `mctest.CombatCorpus` and the `CombatOracle` harnesses: the 1.8.9 lane holds
  full strikes answered by `EntityPlayer.attackTargetEntityWithCurrentItem`
  over a stub world and compared exactly wherever no bonus rides the
  attacker's yaw; the 26.1.2 lane holds the charge curve and the base impulse,
  because `Entity.hurtOrSimulate` takes the server branch only over a real
  `ServerLevel`, and its corpus's `Dropped` list says so rather than
  presenting the two lanes as equivalent.

### Fixed

- `navigation`: a jump starts from the ground, not from water.

### Changed

- The module consumes `minecraft-protocol` v0.8.0.

## 0.2.0 - 2026-08-18

### Added

- `navigation`: the edges the design named and never had. `EdgeJumpGap` crosses
  a gap, which nothing in the shipped vocabulary could — `EdgeStep` rises into an
  adjacent cell and `EdgeFall` descends into one, so a body at the edge of a
  two-block hole had no edge that reached the far side and every route around
  every gap was a detour. `EdgeWaterDrop` takes a drop past the safe fall when
  there is enough water at the bottom, `EdgeClimb` goes up and down ladders and
  vines, and `EdgeDoor` opens a door on the way through.

- `navigation/reach`: the jump reach, measured by running a profile's own
  movement kernel over flat ground rather than guessed. The 2026-08-17 plan
  deferred the jump edge rather than ship a maximum gap this repository could not
  verify, and this is the deliverable that unblocked it. The two versions
  disagree, which is the gate: 1.8.9 clears 2.439107 blocks from a standing
  sprint jump and 26.1.2 clears 2.731274, and a table reading a shared constant
  fails the test that compares them.

- `navigation`: `PostureSneak`, `PostureFall`, and `PostureCrawl`. Crawling is
  the first behaviour here that one supported version has and the other does not
  — 26.1.2 fits through a one-block gap and 1.8.9 has no crawl at all — and the
  absence is asserted in both directions rather than skipped. Sneaking is a
  ledge-crossing posture and buys no headroom, deliberately: both versions
  shorten the body when it crouches and on a block grid that changes nothing,
  because 1.8 and 1.5 need the same two cells.

- `navigation`: `Overlay`, `EdgePlace`, and `EdgePillar`, with the re-run-and-ban
  validation loop the design specifies. A search expands nodes with no notion of
  which placements a route has already made, so a winning path can put a block in
  a cell one of its own later edges needs; validating the winner forward through
  an overlay and banning the offending edge is what resolves it. `EdgePillar` is
  the edge the design's list had no member for: `Place` bridges horizontally and
  nothing gained height, so a body with a stack of blocks was capped by its step
  height exactly as a body with none. It comes with a vertical envelope and a
  per-column limit, because a search that can reach every Y coordinate will
  otherwise spend its whole node budget finding that out.

- `terrain`: `Facts` answers `Climbable` and `Door`. Neither is readable from a
  collision shape — a ladder's box is empty and a closed door looks like a wall —
  so both arrive from the profile, the same way hazards and fluids do.

- `geom`: `AABB.Nearest` and `AABB.Reaches`, and the aiming arithmetic —
  `Vec3.Pitch`, `Vec3.Look`, `Vec3.Toward`, `Vec3.Yaw`, `Vec3.HorizontalDistance`,
  and the `Behind`, `Lead`, `Tangent`, and `Away` functions. Reach is measured to
  a box's nearest point rather than to its centre, because that is what the game
  measures and a client using the centre refuses hits the server accepts. The
  package still imports nothing outside the standard library.

- `placement`: whether a block may go where a click points, and what state it
  becomes. `Resolve` turns a clicked cell and a face into a target, `Check`
  answers legality over the replaced block and the entities standing in the way,
  and the `Place` command and its phase carry the click plus the three things a
  tick cannot know: what is held, where the eye is, and how far it reaches. The
  write is outside the decision, so a refusal cannot reach the world; a refused
  placement that still wrote would put a block in the client the server does not
  have, standing there until the next chunk update took it away. An undescribed
  cell is neither: the outcome names the cell so a caller knows what to load.

- `placement.Placer`, answered by both profiles in their own vocabularies:
  1.8.9 computes four bits of metadata, 26.1.2 an offset into the block's state
  range. The rules that decide which value — the facing a yaw gives, the half a
  face and a cursor give, the axis a face gives — are version-neutral and live in
  `placement`, because both editions compute them identically. 26.1.2 needs no
  per-family bit layout: the offset is a mixed-radix number over the block's own
  published property list. 1.8.9 publishes no such mapping, so its layouts are
  transcribed from `BlockStairs`, `BlockSlab`, and `BlockLog`, each naming the
  method it came from, and the gate is 24 clicks per version compared against
  `Block.onBlockPlaced` and `Block.getStateForPlacement` on the handle rather
  than the block name. A placement producing the right block in the wrong
  orientation resolves a different handle, which is what makes the transcription
  checkable rather than trusted.

### Changed

- **Breaking for anything that mints or reads a 1.8.9 block handle:**
  `profile/java/v1_8` gives a handle per block *state* rather than per block. The
  table mints sixteen handles per block, which is what four bits of metadata and
  `Block.getMetaFromState` produce, and a name still resolves metadata zero — so
  every fixture, scene, and trace resolves what it resolved before. Callers that
  assumed one handle per block need to say which state they mean.

  It closes a defect that was already there. The dataset lists a stone slab's
  collision shape per metadata, the bottom half for 0 through 7 and the top half
  for 8 through 15, and the old table took the first and answered it for every
  metadata, so a body walked through a top slab. The five 1.8.9 replay
  recordings were regenerated: the only line that moved in each is the data
  digest, because the table is larger, while every tick digest and every position
  is identical.

- Requires `minecraft-protocol` v0.7.0, for `BlockMovementRegistry`'s falling and
  climbable accessors. They are what lets a profile answer `terrain.Facts` from
  measured data instead of a caller carrying its own ladder list.

### Fixed

- `navigation`: the heuristic floor is computed over the edges a capability
  actually enables. It was computed over the movement edges alone, so the moment
  a body could place, jump, or climb more cheaply per block than it could walk,
  the heuristic overestimated and A* stopped returning shortest routes. The
  symptom would have been paths that are merely suboptimal, which is the hardest
  kind of wrongness to notice; a property test over random capabilities with
  random subsets enabled is what now catches it.

## 0.1.0 - 2026-08-18

The first release. Everything below is what the module is on the day it is
published: a deterministic tick over a tri-state block view, collision for two
Java Edition versions, two player profiles gated on those versions' own servers,
route search over the same world model, and the recording and replay that keep
all of it reproducible across six platforms.

### Added

- `profile/java/v26_1`: Java Edition 26.1.2 for a player on land — the block
  table, the constants, the widths, and a thirteen-phase tick order established
  from the version's own movement path rather than from 1.8.9's. 4,800 ticks of
  walk, sprint, jump, sneak, fall, and collide agree with a real 26.1.2 server
  bit for bit, and the six scenarios ship as fixtures in `mctest/testdata/26_1`
  that replay with no JDK.
  Three rules do not survive being written by analogy, and the README's table
  lists the rest: the motion threshold is a vector rule and a player's; the
  input shaping is a client method whose clamp discards the 0.98 decay for any
  input that reaches it; and the block whose friction applies is the column of
  the block the last move recorded as supporting the body, not the cell under
  its feet.
  The block table is keyed by block *state*, because the flattening made the
  state number a block's whole identity and a slab's two halves do not stand in
  the same volume.
- `movement.Box` builds a body's collision box from a position and two
  dimensions, and `movement.Table.At` reads the trigonometry table at an index
  its caller computed. They are two of the three things the two versions
  provably share; everything else in the two tick lists is duplicated on
  purpose, and the README says which and why.
- `entity.State` carries a `Position` and a `Support` record, for a version that
  moves the position and rebuilds the box around it and that remembers what is
  holding a body up. Both are written to a tick digest only when they are set,
  so every recording made before them still verifies unchanged.
- `replay/testdata/26_1`: four determinism recordings on the new profile, in the
  same matrix as the others.

### Fixed

- `navigation`: the search heuristic scaled Manhattan distance by the cheapest
  single edge, but a step and a fall each close two blocks of distance for one
  edge's cost, so it overestimated and the search could return a route that was
  not shortest. It now scales by the lowest cost per block closed.
- `collision`: `Gather` no longer offers a shape the sweep only reaches the cell
  of. The game collects a block's collider where the sweep overlaps the block in
  a volume, and sharing a face is not overlapping; collecting per cell instead
  gave the shape-based step-up rises no candidate list of the game's contains,
  which made a body climb onto a block it was floating above. It also reported a
  horizontal collision, on either version, for a body flush against a wall and
  moving into it by less than its own coordinates can hold. Both were found by
  asking the game: a 26.1.2 server for the first and a 1.8.9 server for the
  second. One committed determinism recording changes with it, at the one tick
  where a fall carried a motion of 2e-18 into a face.
- `movement`: the heading rule and the cosine's table index now narrow each
  product before adding it, which stops arm64 contracting the multiply and the
  add into one rounding. Java never contracts, so the fused answer was a
  position vanilla does not produce; the determinism matrix found it.

### Added

- `navigation.Planner`: a per-body cache of terrain answers, invalidated by the
  cells a caller reports through `Observe`. Where it runs the concrete search it
  returns byte-identically what `Find` returns; the cache changes how long an
  answer takes, never what it says.
- `internal/oracle`: a 26.1.2 harness that drives the game's whole
  `Entity.collide`, so the step-up assembly around the pieces — the grounded box,
  the probe the candidate rises come from, the first improving rise, and the drop
  back to the original feet — is checked against the game rather than against a
  reading of it. It stands up a level the game accepts, which is what the earlier
  shape harness said it would take, and places real blocks by name so the shapes
  compared against are the game's own.
- `geom`, `world`, and `collision`: swept axis-aligned collision reproducing
  Java Edition 1.8.9 axis order and step-up, over a block view that
  distinguishes known air from unknown regions.
- `sim`, `entity`, and `runtime`: the tick contract, entity bodies, a canonical
  result digest, and an in-memory store whose revision check refuses a change
  set computed against older state.
- `adapter`: the seam a headless client and a server implement to drive the same
  kernel, with the tick assembly they share. It carries commands and change
  sets, and no packet type.
- `movement`: the version-neutral movement rules — table-backed trigonometry,
  friction and acceleration, the heading, the jump and its counter, gravity, the
  two drags, and the motion threshold — with every constant supplied by a
  profile at the width the game computes it.
- `profile/java/v1_8`: Java Edition 1.8.9 as a `sim.Profile`. It supplies the
  constants, the sine table, the block table, and the twelve phases of the land
  tick, and it is the only package here that imports game data.
- `sim.BlockNames`: an optional interface a profile implements to resolve a
  block name to the handle it minted, for worlds that arrive from outside the
  simulation.
- `mctest`: recorded trajectories and a replay that needs no jar, with six
  committed fixtures whose expectations are the game's own answers.
- `scene`: a world described by name — a filled region and the named blocks in
  it — which `mctest` fixtures and `replay` recordings both build from.
- `replay`: recording a run as its input and the digest of every tick, and
  replaying it against a profile, reporting the first differing tick with the
  body rendered at full precision.
- `sim.BlockNames` and `sim.DataDigest`: optional interfaces a profile
  implements to resolve a block name to a handle and to name the game data it
  was built from.
- A `Determinism` workflow that replays the committed recordings on Linux,
  macOS, and Windows, on amd64 and arm64, and a `determinism` task that runs the
  same check with only a Go toolchain.
- `internal/oracle`: differential tests that run the geometry primitives, the
  whole movement path, and a whole movement tick against a real 1.8.9 server jar
  and require bit-identical results. They skip when no prepared jar or JDK is
  present.
- `terrain`: static predicates over a world view — whether a body fits, whether
  anything holds it up, and whether a cell is clear, steppable, blocked, or
  undescribed — with the body as a value and every block fact the collision
  shape does not carry supplied through a profile oracle.
- `navigation`: a bounded route search over walk, step, fall, and swim edges,
  costed in ticks, whose frontier breaks ties on a total node order so two
  identical searches return identical paths.
