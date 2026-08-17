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
