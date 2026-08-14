# minecraft-simulation

Protocol-independent Minecraft simulation for Go clients and servers.

The module will provide deterministic game-state transitions that server and
client applications can share. Protocol codecs, packet IDs, networking,
persistence, rendering, and AI remain outside this repository.

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
