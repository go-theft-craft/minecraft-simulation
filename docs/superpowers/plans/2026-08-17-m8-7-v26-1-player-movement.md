# M8.7 v26_1 Player Movement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver `profile/java/v26_1` for the player, so that the M8.4 scenario suite passes on Java Edition 26.1.2, and extend the ground-truth pipeline far enough to supply that version's constants.

**Status:** Task 1 is done and its answer is in
`docs/superpowers/notes/2026-08-17-v26-1-oracle-feasibility.md`. A jar-backed
oracle is possible and cheaper than 1.8.9's, because Mojang ships this version
unobfuscated — so this milestone takes the strong branch below and the gate is
the game. Three things the note changes: the collision variant is a prerequisite
task rather than a contingency, the oracle's world stub is its largest piece
rather than an afterthought, and four quantities differ from 1.8.9 in formula
rather than in width, so nothing may be ported by analogy.

**Architecture:** The same shape as M8.4 and deliberately not the same code. `profile/java/v26_1` owns 26.1.2's constants, its `float32` arithmetic, and its own phase order. Rules that genuinely coincide with 1.8.9's move into `movement` as shared functions; rules that differ get their own. `collision`, `geom`, `sim`, and `runtime` do not change.

**Tech Stack:** Go 1.26.6, `minecraft-reference` for extraction, `minecraft-protocol` for generated data, Devbox, go-task.

## This plan spans three repositories, and starts with what nobody knows yet

M8.4's plan could state the tick it reproduced, line by line, because a deobfuscated 1.8.9 server sat in the reference workspace and could be read and executed. Almost none of that is true here yet, and pretending otherwise would produce a plan whose tasks are guesses.

What is known:

- **The dumper refuses this version.** `minecraft-reference` gates extraction on the version string and reports that only 1.8.9 is implemented. Its Java dumper source names 1.8.9 identifiers throughout, so extending it is a new dumper rather than a new argument.
- **There is no deobfuscated 26.1.2 jar in the workspace.** `prepare` leaves an original and an executable jar for this version, and decompiled sources, but not the named jar the 1.8.9 oracle compiles against. Whether a named jar can be produced, and by which path, is the first thing to find out, because both the extraction and the oracle depend on it.
- **The movement code is structurally different.** The parent design says so and rates this the largest single risk in the whole subproject: "It shares little structure with 1.8.9, and the version is recent enough that its behavior can still change." Swimming, elytra flight, and the 1.13 collision rework are different code paths, not flags.
- **The constants pipeline is already version-parameterized.** M8.1 left extracted data optional in the manifest schema and the render plan, so a second version generates without touching the first. That part is ready.

So Tasks 1 and 2 are research with committed artifacts as output, and the plan states what it will do with either answer. Everything after them is contingent on what they find, and the plan says where.

## Global Constraints

- Modules touched: `minecraft-reference` (the dumper), `minecraft-protocol` (the dataset and generated package), `minecraft-simulation` (the profile).
- **Only `profile/java/v26_1` imports a protocol package**, and it imports only `minecraft-protocol/data`.
- **`float32` stays in the profile.** 26.1.2's widths must be established independently. Do not assume they match 1.8.9's: a constant that is a `double` in one version may be a `float` in the other, and M8.2 showed what assuming costs.
- No decompiled Java, no Mojang asset, and no source text from the reference workspace is committed, in any of the three repositories.
- The 1.8.9 profile, its fixtures, and its recordings must not change. If a shared rule needs a different shape for 26.1.2, the 1.8.9 behaviour is what proves the refactor was safe, and its committed digests are the check.
- Formatting and commit conventions as in the other plans. No `Co-Authored-By` or `Claude-Session` trailer.

## Design decisions this plan settles

### Shared rules are extracted only after both versions exist

The temptation is to design a shared movement abstraction now and implement both profiles against it. That is backwards. M8.4's rules were written against one version; which of them 26.1.2 shares is a finding, not a premise.

So: implement 26.1.2's rules in its own package first, duplicating whatever looks similar, and extract shared functions into `movement` only where the two are demonstrably identical — same constants, same widths, same order. The 1.8.9 recordings from M8.6 are the safety net for every extraction, because an extraction that changes a 1.8.9 digest is a broken extraction and the matrix says so immediately.

Duplication that survives to the end of this milestone is a finding worth recording, not a failure. Two versions that genuinely differ should read as two implementations.

### The scenario suite is the contract, not the rules

The exit criterion is that the M8.4 suite passes on 26.1.2, which is a statement about scenarios — walk, sprint, jump, sneak, fall, collide — and not about expected numbers. The expected numbers are this version's, generated from this version's jar.

`mctest` therefore gains nothing here beyond a second set of fixtures, provided M8.4 built the format properly: named blocks, a profile identity, per-tick inputs and expectations. If it did not, fixing that is part of this milestone rather than a workaround inside it.

### Scope is the player on land, again

Swimming and elytra are 26.1.2's own mechanics and they are not in the M8.4 suite. Adding them here would mean this milestone gates on scenarios the 1.8.9 profile has no counterpart for, which is a different milestone. The phase list must leave room for them, and the plan records them as out of scope rather than silently absent.

## File structure

```text
minecraft-reference/
  internal/reference/physics/dumper_source_26_1.go   A second dumper program
  internal/reference/physics/dumper.go               Version selection
  internal/reference/physics/dumper_test.go

minecraft-protocol/
  source/java/26.1/physics.json      Committed, digest-pinned
  generated/java/v26_1/physics.go    Generated
  manifest.json                      A second physics entry
  THIRD_PARTY_NOTICES.md             Mojang provenance for the new file

minecraft-simulation/
  profile/java/v26_1/profile.go
  profile/java/v26_1/phases.go
  profile/java/v26_1/blocks.go
  profile/java/v26_1/*_test.go
  movement/shared.go                 Only what both versions provably share
  mctest/testdata/26_1/*.json        Generated fixtures
  replay/testdata/26_1/*.json        Recordings, joining the M8.6 matrix
  internal/oracle/java/MovementOracle26.java
  internal/oracle/movement26_test.go
```

---

## Task 1: Find out whether a 26.1.2 oracle is possible

**Files:**
- Create: `docs/superpowers/notes/2026-08-17-v26-1-oracle-feasibility.md`

**Interfaces:** produces no code. It produces the decision the rest of the plan branches on.

Answer four questions and commit the answers with the commands that produced them:

1. **Can a named 26.1.2 server jar be produced?** Mojang publishes official mappings for modern versions. Determine whether `mcreference prepare` already fetches them, whether it applies them, and if not, what it would take. Record the exact artifact paths that exist today.
2. **Does the movement code reflect cleanly in a headless process?** The 1.8.9 oracle needed an explicit bootstrap call before registries populated. Establish the equivalent for this version, and whether a modern server jar can be initialized far enough to move an entity without starting a world.
3. **Is the entity movement method reachable the same way?** The 1.8.9 harness drove a minimal living-entity subclass. Determine whether the modern hierarchy allows the same, or whether a concrete player type is needed, or whether neither works.
4. **Which constants are `float` and which are `double`?** For gravity, the drags, the friction product, the step height, and the input scale. This is the M8.2 lesson applied before writing code rather than after.

- [x] **Step 1: Answer all four, with evidence**

- [x] **Step 2: State the consequence for this milestone**

Write, explicitly, one of:

- **A jar-backed oracle is possible.** Then this milestone follows M8.4's shape exactly: differential test first, generated fixtures second, and the gate is the game.
- **A jar-backed oracle is not possible.** Then say why, and the milestone's gate degrades to hand-derived fixtures plus M8.8's live server against a 26.1.2 server. Say plainly in the notes and in the master plan that this is the weaker gate M8.4 avoided, and that a constant wrong in the fifteenth decimal place will not be caught until a real client connects.

Do not start Task 3 until this is written down. A milestone whose verification strategy is decided halfway through is a milestone that will be verified by whatever was convenient.

- [x] **Step 3: Commit**

```bash
git add docs/superpowers/notes/2026-08-17-v26-1-oracle-feasibility.md
git commit -m "docs(notes): record whether a 26.1.2 movement oracle is possible"
```

---

## Task 2: Extend the dumper to 26.1.2

**Repository:** `minecraft-reference`

**Files:**
- Create: `internal/reference/physics/dumper_source_26_1.go`
- Modify: `internal/reference/physics/dumper.go`
- Test: `internal/reference/physics/dumper_test.go`

**Interfaces:**
- Produces: version selection replacing the current single-version rejection, and a second embedded dumper program.

The existing dumper rejects every version but 1.8.9 with an explicit error, which is the right shape: it means adding a version is adding a case, and a version nobody has written a dumper for still fails loudly rather than producing an empty dataset.

What the new dumper must produce, matching the 1.8.9 schema so the generator needs no new shape: block slipperiness with its default, the per-family motion constants, and the trigonometry table if this version still uses one. **If it does not use a lookup table**, that is a schema question — record the table as absent and let the profile compute, and say so in the notes from Task 1, because a profile computing a sine is exactly the last-place-divergence risk the table was dumped to avoid.

The constants must be captured at their real width. The 1.8.9 dumper reached most values reflectively and needed twelve transcribed by hand because they are literals inside method bodies. Expect the same division here, and expect the split to fall differently.

- [ ] **Step 1: Write the failing test**

Cover: `Dump` for an unsupported version still fails with a message naming the version; `Dump` for 26.1.2 without a prepared jar fails with a clear error rather than a panic; and with a prepared jar, the output validates against the schema and contains a non-empty slipperiness index and motion constants for the player.

- [ ] **Step 2: Write the dumper and run it**

- [ ] **Step 3: Verify each transcribed constant twice**

Any value a reflective dumper cannot reach is transcribed, and M8.1's rule applies: confirm it from two independent readings before committing it. Record both in the reference notes.

- [ ] **Step 4 to 6: Verify, check, commit**

```bash
cd ../minecraft-reference
git add internal/reference/physics/
git commit -m "feat(physics): add a 26.1.2 physics dumper"
```

---

## Task 3: Commit and generate the 26.1.2 dataset

**Repository:** `minecraft-protocol`

**Files:**
- Create: `source/java/26.1/physics.json`
- Modify: `manifest.json`, `THIRD_PARTY_NOTICES.md`
- Generated: `generated/java/v26_1/physics.go`

M8.1 left this parameterized, so this task is mechanical: commit the extracted JSON, add the manifest entry with its digest and Mojang provenance, regenerate, and confirm `generate:check` passes with neither `java` nor `javac` on `PATH`.

The notices entry states Mojang provenance rather than inheriting the PrismarineJS attribution, the same way 1.8.9's does.

- [ ] **Step 1: Commit the source, add the manifest entry, regenerate**

- [ ] **Step 2: Confirm no-JDK generation**

Run the repository's `generate:check` with `java` and `javac` removed from `PATH`. Both versions must render from pinned source.

- [ ] **Step 3: Confirm the widths survived**

Add a test asserting the player's constants match the extracted file exactly, including any value whose decimal form is not round. A generator that formatted a `float`-derived constant to fewer digits would lose exactly what M8.2 proved matters, and the round-trip test is what catches it.

- [ ] **Step 4 to 5: Verify, commit**

```bash
cd ../minecraft-protocol
git add source/java/26.1/physics.json generated/java/v26_1/physics.go manifest.json THIRD_PARTY_NOTICES.md
git commit -m "feat(data): pin and generate Java 26.1.2 physics constants"
```

---

## Task 4: The v26_1 block table

**Files:** `profile/java/v26_1/blocks.go` and its test

The same job as M8.4's block table, against this version's block model. The 1.13 flattening means block identity is a state id rather than an id and metadata pair, so the handle mapping is simpler here and the `ref(name, meta)` helper M8.4 needed for fixtures becomes `ref(name, properties)` or `ref(stateID)`, depending on what `minecraft-protocol` exposes for this version.

Slipperiness narrows to `float32` at this boundary if and only if Task 1 found it to be a `float` in this version. If it is a `double`, keep it `float64` and say so in a comment, because the asymmetry with the 1.8.9 table will otherwise read as a mistake.

- [ ] **Steps: as M8.4 Task 6, against this version's data**

```bash
git add profile/java/v26_1/blocks.go profile/java/v26_1/blocks_test.go
git commit -m "feat(profile): map 26.1.2 block data to simulation handles"
```

---

## Task 5: The v26_1 profile and its phase list

**Files:** `profile/java/v26_1/profile.go`, `phases.go`, and their tests

Identity is `java/26.1.2@1`. The phase list is this version's, in this version's order, with namespaced identifiers prefixed `v26_1.`.

**Do not copy M8.4's eleven phases and adjust constants.** Establish the order from the version's own movement method, the way M8.4's was established, and let it come out however it comes out. If it happens to match, that is a finding worth a comment; if the plan assumed it and it does not, the trajectories will be wrong in ways fixtures generated from the same assumption will not catch.

The known structural differences to expect, from the parent design: the 1.13 collision rework, and additional movement states. The collision rework matters most, because `collision` is M8.2's package and reproduces 1.8.9 exactly, including the deliberate absence of an epsilon that later versions added. **This is the milestone's second real risk after the oracle**: if 26.1.2's collision differs, this profile cannot reuse `collision.Resolve` unchanged, and the fix is a version-selected variant behind the same interface rather than an epsilon parameter smuggled into the 1.8.9 path.

Determine this early — it is cheap to check and expensive to discover late — and if a variant is needed, add it as its own task before the phases, with the 1.8.9 recordings proving the original path unchanged.

- [ ] **Step 1: Establish the tick order and record it in the plan's own terms**

Write the numbered behavioural statements for this version, the way M8.4's plan did, into the profile package's doc comment. Prose in a doc comment is the artifact that survives; prose in a session does not.

- [ ] **Step 2: Check whether collision differs, and branch**

- [ ] **Step 3 to 7: Test, implement, verify, commit**

```bash
git add profile/java/v26_1/
git commit -m "feat(profile): add the Java 26.1.2 player profile and its tick phases"
```

---

## Task 6: The gate

**Files:** `internal/oracle/java/MovementOracle26.java`, `internal/oracle/movement26_test.go`, `mctest/testdata/26_1/*.json`

Which of these exist depends on Task 1.

**If a jar-backed oracle is possible:** mirror M8.4 Task 8 and Task 9. Differential test over the six scenarios, comparing every tick bit for bit; then generate the fixtures from the harness; then confirm they replay with no JDK.

**If it is not:** build the fixtures from the behavioural statements in Task 5, and mark every one of them as unverified against the game in the fixture file itself, with a field that says so. Then M8.8's live check against a 26.1.2 server is the first real verification, and the master plan must say that this milestone's exit criterion is weaker than M8.4's. A fixture that records a belief and does not say so is the thing that makes a later failure hard to diagnose.

- [ ] **Step 1: Build whichever gate Task 1 permits**

- [ ] **Step 2: Add recordings to the M8.6 matrix**

Generate `replay/testdata/26_1/*.json` covering this version's numerics, at least 200 ticks each. The matrix replays every file in `testdata` and each recording pins its own profile, so no workflow change is needed.

- [ ] **Step 3: Confirm 1.8.9 is untouched**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task determinism`

Every 1.8.9 recording must still verify. If a shared extraction changed one, revert the extraction.

- [ ] **Step 4 to 5: Verify, commit**

```bash
git add internal/oracle/ mctest/testdata/26_1/ replay/testdata/26_1/
git commit -m "test(profile): gate the 26.1.2 player on the scenario suite"
```

---

## Task 7: Extract only what both versions share, and record the milestone

**Files:** `movement/shared.go`, `README.md`, `CHANGELOG.md`

- [ ] **Step 1: Extract, one function at a time**

For each rule that is identical in both profiles — same constants, same widths, same position in the order — move it to `movement` and have both call it. After each extraction, run the determinism task. An extraction that changes any committed digest, of either version, is reverted rather than debugged, because a shared function that alters behaviour is not shared.

- [ ] **Step 2: Record what stayed duplicated, and why**

This is the milestone's most useful documentation. Two versions of a rule that look similar and are not identical are the trap every later mechanic will walk into, and the list of them is worth more than the abstraction that would have hidden them.

- [ ] **Step 3: Changelog, verify, commit**

```bash
git add movement/shared.go README.md CHANGELOG.md
git commit -m "docs: record M8.7, and what the two profiles actually share"
```

---

## Definition of done

- The dumper supports 26.1.2, still rejects unknown versions explicitly, and every transcribed constant was confirmed twice.
- `source/java/26.1/physics.json` is committed and digest-pinned with Mojang provenance, `generated/java/v26_1/physics.go` renders from it, and `generate:check` passes with no JDK.
- `profile/java/v26_1` supplies this version's constants, block table, and phase order, established from the version's own movement method and written down in its doc comment.
- Whether 26.1.2's collision differs from 1.8.9's is answered, and if it does, the variant is version-selected rather than added to the 1.8.9 path.
- **The M8.4 scenario suite passes on 26.1.2** — walk, sprint, jump, sneak, fall, collide — against the strongest gate Task 1 found available, with the strength of that gate stated plainly in the master plan.
- 26.1.2 recordings join the M8.6 matrix and pass on every target, and every 1.8.9 recording still verifies unchanged.
- Every rule shared between the profiles is provably identical; every rule that is not is duplicated on purpose and listed.
- `devbox run -- task verify` passes in all three repositories.

## Follow-on

M8.8 runs both profiles through the client and server adapters, and its live gate now has two versions to satisfy. If Task 1 found no jar-backed oracle for this version, M8.8's 26.1.2 lane is the first real verification of these constants and should be scheduled with that in mind rather than as a formality.

M9's mechanics land on both profiles from the start, and the list of deliberately duplicated rules from Task 7 is what tells each of those milestones whether it is writing one implementation or two.
