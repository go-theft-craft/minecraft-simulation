# Changelog

This file records notable user-visible changes. It follows [Keep a Changelog](https://keepachangelog.com/en/2.0.0/) and [Semantic Versioning](https://semver.org/).

## Unreleased

### Fixed

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
