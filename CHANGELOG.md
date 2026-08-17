# Changelog

This file records notable user-visible changes. It follows [Keep a Changelog](https://keepachangelog.com/en/2.0.0/) and [Semantic Versioning](https://semver.org/).

## Unreleased

### Added

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
- `internal/oracle`: differential tests that run the geometry primitives, the
  whole movement path, and a whole movement tick against a real 1.8.9 server jar
  and require bit-identical results. They skip when no prepared jar or JDK is
  present.
