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
| `entity` | Entity identity, physics families, and the body state a tick moves |
| `sim` | The tick contract: input, result, canonical digest, profile, phases, kernel |
| `runtime` | The store, its revision check, and a runner that drives one tick after another |
| `adapter` | The seam a consumer implements to drive one kernel, and the tick assembly they share |

Packages depend in one direction only:

```text
geom  ->  world  ->  entity  ->  sim  ->  runtime
              \                   ^
               \-> collision -----/
```

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

## Checking against the game

`internal/oracle` compares this module against a real Java Edition 1.8.9
server. It compiles a small harness against the locally prepared, deobfuscated
server jar and runs the game's own `AxisAlignedBB` methods and its own
`Entity.moveEntity` — including the candidate gathering, the axis passes, the
two step-up attempts, and the settle — then requires the resulting box to be
bit-identical to what `collision.Resolve` produces, and the collision flags to
agree.

The harness supplies a block lookup and a minimal entity. It reimplements no
game logic, and no game source is committed. The jar is not committed either,
so these tests skip when the workspace, `javac`, or `java` is absent; run
`task reference:prepare` to make them run.

Two behaviours were found this way rather than by reading:

- A step-up records the settle as its Y motion, not the climb plus the settle,
  which is why a step leaves the vertical collision flag describing the descent
  onto the surface. `onGround` follows from that flag and the tick's original
  downward motion, so stepping does not by itself put an entity on the ground.
- `stepHeight` is a `float` widened where it is applied, so a player steps with
  `float64(float32(0.6))`. Passing the round `0.6` moves the resulting box in
  its last bits.

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
