# Changelog

This file records notable user-visible changes. It follows [Keep a Changelog](https://keepachangelog.com/en/2.0.0/) and [Semantic Versioning](https://semver.org/).

## Unreleased

### Added

- `geom`, `world`, and `collision`: swept axis-aligned collision reproducing
  Java Edition 1.8.9 axis order and step-up, over a block view that
  distinguishes known air from unknown regions.
- `internal/oracle`: differential tests that run the geometry primitives and the
  whole movement path against a real 1.8.9 server jar and require bit-identical
  results. They skip when no prepared jar or JDK is present.
