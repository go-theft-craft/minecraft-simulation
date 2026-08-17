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
- `internal/oracle`: differential tests that run the geometry primitives and the
  whole movement path against a real 1.8.9 server jar and require bit-identical
  results. They skip when no prepared jar or JDK is present.
