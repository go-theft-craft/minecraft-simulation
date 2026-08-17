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
