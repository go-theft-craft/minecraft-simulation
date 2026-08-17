# M8.6 Replay and Determinism Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver canonical recording and replay on top of the digest M8.3 built, and prove that the same input produces an identical digest on Linux, macOS, and Windows, on amd64 and arm64.

**Architecture:** `replay` records a run as an ordered sequence of tick inputs and result digests, and replays it against a profile, failing at the first tick whose digest differs. The canonical encoding and the digest already exist in `sim`; this milestone adds the recording around them and the continuous integration that makes the cross-platform claim true rather than asserted.

**Tech Stack:** Go 1.26.6, standard library only, Devbox, go-task, GitHub Actions.

## The gate's real precondition, checked before anything is built

The sequencing design flags this milestone's risk and says to confirm it before the milestone starts, not during: "The determinism matrix needs runners. Whether that CI capacity exists is unverified, and the gate is meaningless without it."

It has now been checked, and the answer is a qualified yes with one genuine obstacle.

`minecraft-simulation/.github/workflows/ci.yml` runs a single job on `ubuntu-latest` and drives every check through `devbox run -- task verify`. GitHub-hosted runners can cover all six targets the gate names — Linux on amd64 and arm64, macOS on both, Windows on both — so the machines are not the problem.

**Devbox is the problem.** It provisions the toolchain through Nix, which does not run natively on Windows. Every existing check in every one of these repositories assumes `devbox`, so a Windows job cannot simply reuse `task verify`.

Task 1 resolves this before any code is written, because the resolution decides what the rest of the milestone builds. The recommended resolution, and the one the tasks below assume unless Task 1 finds otherwise, is:

- The **verification** job stays as it is: `devbox run -- task verify` on `ubuntu-latest`. One platform runs lint, secrets, vulnerabilities, and the full suite. Nothing about that needs six platforms.
- The **determinism** job is separate, runs on all six targets, and uses `actions/setup-go` with the Go version pinned to the one `devbox.json` names. It runs one command: a digest check. It does not lint, does not scan, and does not need Nix.

Two toolchains is a cost, and it buys the only thing that matters here: the digest is computed by the same Go version on six platforms. If the two ever disagree about the Go version, the matrix silently stops testing what it claims to, so Task 1 also adds a check that they agree.

## Global Constraints

- Module is `github.com/go-theft-craft/minecraft-simulation`. Go directive is `1.26.6`.
- No protocol imports outside `profile/java/*`. `replay` imports `sim`, `world`, `entity`, `movement`, and the standard library.
- Standard library only. No new module dependencies.
- The determinism job must not require a JDK, a game jar, or a network. It replays committed recordings.
- Recording files are committed, are text, and have a stable field order, so a diff is reviewable.
- Formatting is `gci` then `gofumpt`. Commit messages use Conventional Commits, with no `Co-Authored-By` or `Claude-Session` trailer.

## Design decisions this plan settles

### A recording holds inputs and digests, not results

A recording stores the initial world, the initial bodies, and per tick the commands and the resulting digest. It does not store whole results.

Storing digests is what makes the cross-platform claim sharp: if a platform disagrees, it disagrees about a hash, and the hash covers every field of a result because M8.3's `TestDigestNoticesEveryField` proves it does. Storing whole results would let a reviewer skim a diff and think they had checked something they had not, and would make every recording large enough that nobody reads it.

For debugging a mismatch, the failure needs more than "tick 47 differs". `replay.Verify` therefore reports the first differing tick and, for that tick alone, the whole expected and actual result rendered for a human. One tick of detail on demand beats every tick of detail always.

### A recording pins the profile and the game data

A replay fails before simulating if the recording's profile identity or game-data digest does not match the profile it was given, which is the parent design's rule. A recording that ran against different constants is not evidence about this build.

### The matrix hashes movement, not the encoder

The encoder is integer and byte work; it was never plausibly platform-dependent. What this matrix actually tests is M8.4's `float32` arithmetic and the sine-table indexing, because those are the only places where a compiler, an architecture, or a fused multiply-add could change a result.

That has a consequence for what the recordings must contain: a matrix over empty ticks would pass on six platforms and prove nothing. The committed recordings must exercise the numerics — sprinting diagonals, ice, jumps mid-air, and the step-up retry — or the gate is decoration. Task 4 says so explicitly and the definition of done requires it.

### Go's floating-point guarantees are relied on, and the reliance is stated

Go specifies IEEE 754 arithmetic for `float32` and `float64` and requires that an explicit conversion round to the target precision. It permits fusing only when the result is not explicitly rounded, and `movement`'s intermediate values are all named `float32` variables, which forces rounding at each step. `math.Sqrt` is exactly rounded on every supported platform.

That is the argument for why the matrix should pass. The matrix exists because an argument is not evidence.

## File structure

```text
minecraft-simulation/
  replay/recording.go        Recording, Tick, Load, Save
  replay/recording_test.go
  replay/record.go           Recording a run from a runtime.Runner
  replay/record_test.go
  replay/verify.go           Verify, and the first-mismatch report
  replay/verify_test.go
  replay/testdata/*.json     Committed recordings, exercising the numerics
  Taskfile.yml               A determinism task with no devbox assumptions
  .github/workflows/ci.yml   The verification job, unchanged
  .github/workflows/determinism.yml   The six-target matrix
```

---

## Task 1: Settle the toolchain question, and pin the Go version once

**Files:**
- Modify: `Taskfile.yml`
- Create: `.github/workflows/determinism.yml`
- Create: `internal/buildcheck/toolchain_test.go`

**Interfaces:**
- Produces a `determinism` task that runs `go test ./replay/ -run TestRecordingsAreReproducible` and depends on nothing but a Go toolchain.

Nothing else in this milestone is worth building if the matrix cannot run, so the matrix runs first, against a placeholder test that passes trivially. A green six-target matrix on an empty check is a real result: it retires the milestone's stated risk.

- [ ] **Step 1: Confirm each target actually starts**

Add `determinism.yml` with a matrix over `ubuntu-latest`, `ubuntu-24.04-arm`, `macos-latest`, `macos-13`, `windows-latest`, and `windows-11-arm`, each using `actions/setup-go` and running `go version` followed by `go test ./replay/`. Push it and read the result.

If a target does not exist or does not start, **do not silently drop it.** Record which target failed and why in the workflow file as a comment and in the master plan, and state the reduced claim the gate now makes. A matrix that quietly covers four platforms while the plan claims six is worse than a matrix that covers four and says so.

- [ ] **Step 2: Pin the Go version in one place**

The workflow must not name a Go version that `devbox.json` does not. Add a test to `internal/buildcheck` that reads `devbox.json` and `.github/workflows/determinism.yml` and fails if the Go versions disagree. Two toolchains are acceptable; two versions are not, because then the matrix tests a compiler nothing else uses.

- [ ] **Step 3: Add the task**

```yaml
  determinism:
    desc: Replay committed recordings and compare digests, without devbox
    cmds:
      - go test ./replay/ -run TestRecordingsAreReproducible {{.CLI_ARGS}}
```

- [ ] **Step 4: Verify and commit**

```bash
git add Taskfile.yml .github/workflows/determinism.yml internal/buildcheck/toolchain_test.go
git commit -m "ci: add the six-target determinism matrix, and pin one Go version"
```

Record in the commit message which targets started and which did not.

---

## Task 2: The recording format

**Files:**
- Create: `replay/recording.go`
- Test: `replay/recording_test.go`

**Interfaces:**
- Produces:
  - `type Recording struct { Profile sim.ProfileID; DataDigest string; Blocks []Block; Bodies []Body; Ticks []Tick; Limits sim.Limits; Random sim.RandomState }`
  - `type Tick struct { Input []Command; Digest string }`
  - `func Load(path string) (Recording, error)`, `func (r Recording) Save(path string) error`
  - `var ErrProfileMismatch`, `var ErrDataMismatch`
  - `func (r Recording) checkProfile(profile sim.Profile) error`

Blocks are named, with metadata, never handles, for the reason M8.4's fixtures are: a handle means nothing outside the profile that minted it.

`Command` in a recording is a serialized form rather than the `sim.Command` interface, because an interface does not round-trip through JSON. It carries a kind string and the fields the movement input needs. When later mechanics add commands, this type grows; a recording naming a kind the loader does not know must fail loudly rather than replay a tick with no input, which would produce a plausible wrong digest.

- [ ] **Step 1: Write the failing test**

Cover: save then load round-trips; field order in the written file is stable across saves, checked by saving twice and comparing bytes; a recording whose profile identity differs fails `checkProfile` with `ErrProfileMismatch` before any simulation; a differing data digest fails with `ErrDataMismatch`; an unknown command kind fails to load with an error naming the kind; and an empty recording loads without error but is reported as covering nothing.

- [ ] **Step 2 to 5: Fail, implement, verify, commit**

```bash
git add replay/recording.go replay/recording_test.go
git commit -m "feat(replay): add the canonical recording format"
```

---

## Task 3: Recording a run, and verifying one

**Files:**
- Create: `replay/record.go`, `replay/verify.go`
- Test: `replay/record_test.go`, `replay/verify_test.go`

**Interfaces:**
- Produces:
  - `func Record(profile sim.Profile, setup Recording, ticks int) (Recording, error)`
  - `func Verify(profile sim.Profile, recording Recording) error`
  - `type Mismatch struct { Tick int; Want, Got string; Detail string }` implementing `error`

`Record` builds a store from the recording's initial state, runs the requested ticks through `runtime.Runner`, and fills in each tick's digest. `Verify` does the same and compares.

`Verify` stops at the first mismatch. A run that diverges at tick 12 will differ at every tick after it, and reporting 88 consequences of one cause is noise. `Mismatch.Detail` carries the expected and actual result for that tick alone, rendered for a human.

- [ ] **Step 1: Write the failing test**

Cover:
- Recording then verifying the same setup passes.
- Verifying a recording with one digest corrupted fails with a `Mismatch` naming that tick and no other.
- The mismatch detail is non-empty and names a field, so a failure on a Windows runner is actionable by someone reading a log rather than needing a local repro on a machine they do not have.
- A recording is reproducible: recording twice from the same setup produces identical digests, which is the same property the matrix checks and worth having locally too.
- Verify refuses a recording whose profile does not match, before simulating.

- [ ] **Step 2 to 5: Fail, implement, verify, commit**

```bash
git add replay/record.go replay/verify.go replay/*_test.go
git commit -m "feat(replay): record a run and verify it digest by digest"
```

---

## Task 4: Recordings that actually exercise the numerics

**Files:**
- Create: `replay/testdata/*.json`
- Test: `replay/verify_test.go`

**Interfaces:** produces no API. This task produces the evidence the gate rests on.

A matrix over empty ticks passes everywhere and proves nothing. These recordings must reach the `float32` arithmetic M8.4 introduced, because that is the only code here that could plausibly differ between platforms.

Required recordings, each at least 200 ticks:

- **Sprinting diagonal.** Both strafe and forward at full input, sprinting, over flat stone, through a full turn of yaw values. This drives the normalization, the sine table, and the widening in the heading rule, which together are the densest `float32` path in the module.
- **Ice.** The same walk over ice and packed ice, so the friction product and the acceleration division run on values other than the default.
- **Jump and fall.** Repeated jumps from stone, and a long fall, so gravity and the vertical drag accumulate over many ticks. Accumulation is where a one-bit difference becomes visible.
- **Step-up.** Walking a staircase of slabs, which reaches the retry M8.2's oracle checked and the settle value it corrected.
- **Slime and soul sand.** The remaining non-default slipperiness values, so the block table is exercised beyond stone and ice.

Two rules for generating them: they are produced by `Record` on the reference platform and committed with the digest they produced, and any change to them is a deliberate commit whose message says which behaviour changed. A recording silently regenerated to make a red matrix go green would convert the gate into a rubber stamp, so the definition of done requires that no task in this plan regenerates a recording to fix a failure.

- [ ] **Step 1: Generate and commit the recordings**

- [ ] **Step 2: Add `TestRecordingsAreReproducible`**

It loads every file in `testdata`, verifies each, and fails naming the file and tick. This is the single test the determinism task runs, so it must not need a JDK, a jar, or a network.

- [ ] **Step 3: Assert the recordings are not trivial**

Add a test that fails if any committed recording has fewer than 200 ticks, or if the set does not cover every scenario named above. Without it, a future contributor shrinking a recording to speed up CI would quietly weaken the gate.

- [ ] **Step 4 to 6: Verify locally, then read the matrix**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task determinism`

Then push and read all six targets.

**If a target disagrees, that is the milestone's finding and it must be investigated, not worked around.** The likely causes, in order: an intermediate value that is not a named `float32` and so was allowed to fuse; a `math` function that is not exactly rounded on that platform; and a genuine compiler difference. Fix the code, not the recording.

```bash
git add replay/testdata/ replay/verify_test.go
git commit -m "test(replay): commit recordings that exercise the float32 paths"
```

---

## Task 5: The milestone record

**Files:** modify `README.md`, `CHANGELOG.md`

- [ ] **Step 1: Record what the matrix covers and what it does not**

State the six targets, which ones ran, and the two-toolchain arrangement with the reason. State that the matrix tests M8.4's arithmetic more than the encoder.

- [ ] **Step 2: Changelog, verify, commit**

```bash
git add README.md CHANGELOG.md
git commit -m "docs: record M8.6 and the determinism matrix"
```

---

## Definition of done

- A recording round-trips, pins its profile and game-data digest, and refuses to replay against a profile that does not match.
- `Verify` reports the first differing tick with enough detail to act on from a CI log alone.
- Committed recordings cover sprinting diagonals, ice, jump and fall, step-up, and the remaining non-default slipperiness values, each at least 200 ticks, with a test that fails if that coverage shrinks.
- **The determinism job runs on Linux, macOS, and Windows, on amd64 and arm64, and every target produces identical digests.** Any target that could not be run is named in the workflow and in the master plan, with the reduced claim stated plainly.
- The determinism job needs no JDK, no game jar, and no network.
- One Go version is named in one place, with a test that fails if `devbox.json` and the workflow disagree.
- No recording was regenerated to make a failing matrix pass.
- `devbox run -- task verify` passes on the verification job.

## Follow-on

M8.7 adds a second profile, and its recordings join this matrix. The matrix does not need changing to accept them: it replays every file in `testdata`, and a 26.1.2 recording pins its own profile identity.

M8.8's live gate is unaffected by this milestone and is not a substitute for it. A real server agrees with one platform at a time.
