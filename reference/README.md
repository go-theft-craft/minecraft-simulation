# Minecraft reference boundary

This repository stores only independently written behavior notes, reviewed
symbol identities, original fixtures, and expected outputs. Never commit or
publish Minecraft jars, mappings, classes, decompiled Java, generated indexes,
or caches.

The separate `mcreference` command writes local research material below the
ignored `reference/work` directory. `reference:prepare` accepts these Task
variables:

- `VERSIONS`: comma-separated supported versions, required
- `SIDES`: `client`, `server`, or both; defaults to both
- `REFERENCE_DIR`: repository-local output directory; defaults to
  `reference/work`
- `MCREFERENCE_VERSION`: `minecraft-reference` tag or revision; defaults to
  `main` until the first tagged release

The default workspace has this layout:

```text
reference/work/
  cache/                         Verified metadata and pinned tools
  versions/<version>/<side>/     Original, executable, and named jars
  sources/<version>/<side>/      Decompiled Java sources
  index/<version>/<side>/        JVM and source JSON Lines indexes
```

Remove the generated workspace with:

```bash
devbox run -- task reference:clean REFERENCE_DIR=reference/work
```

The cleanup command rejects paths outside this repository, the repository
root, and parent directories.
