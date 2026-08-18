# M8.8 Consumer Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status:** Complete, 2026-08-18. The gate passed on both versions on its first
real run: six scenarios, 220 ticks apiece, zero corrections from a real 1.8.9
server and a real 26.1.2 server, and no complaint in either log. What that cost
and what it found is recorded task by task below.

**Goal:** Run one kernel from both consumers — the headless client predicting locally, the server deciding authoritatively — and prove it by connecting to a real vanilla Java Edition 1.8.9 server and executing scripted input that draws zero correction packets.

**Architecture:** `minecraft-simulation` gains a small adapter contract and nothing else; the simulation does not learn about packets. `headless-minecraft` has already gained its outbound action path (see Task 1, reconciled below); this plan adds a prediction loop that forks a snapshot and correction handling that discards the fork and replays retained commands. `server` drives the same kernel through `runtime.Runner` and treats its result as authoritative.

> **Reconciled 2026-08-17.** Task 1 landed in `headless-minecraft` ahead of this
> plan, as `version/action.go` and `client/action.go`. Its section below now
> records what landed and where it differs from the sketch, and Task 4 absorbs
> the one responsibility the landed API deliberately declined: choosing which
> movement packet a tick warrants. Execution starts at Task 2.

**Tech Stack:** Go 1.26.6, `minecraft-protocol`, a pinned vanilla 1.8.9 server jar, Devbox, go-task.

## What this milestone closes, and what it discovers

This is the milestone the whole subproject is arranged around. It is the first point where the simulation faces something that disagrees with it for reasons of its own, and the first check that survives being wrong in the fifteenth decimal place: a vanilla server accumulates our error tick after tick and eventually teleports us back.

Two prerequisites are in place. M6.3 delivered a headless client that reaches play on protocol 47, and M7 delivered observed world state with immutable snapshots and wire-ordered reducers, including a player domain that already models the server's relative-position updates. M8.4 delivered movement checked against the game's own bytecode, so a correction here is unlikely to mean a wrong constant.

Two prerequisites were missing when this plan was written, and finding that out is why it starts where it does. One has since been met.

**The client can now send gameplay packets.** When this plan was written its send path was unexported, because M7's scope was observation, and there was no public way to tell the server where the player went. The outbound action path has since landed in `headless-minecraft` ahead of this milestone: `Client.Do` sends a version-neutral intent that the profile's adapter encodes, which is the shape Task 1 asked for. Task 1 is recorded as complete below, with the differences between what it sketched and what landed, and execution starts at Task 2.

**Only offline mode is available.** M6.4, Microsoft device-code authentication, is postponed by deliberate decision, and the master plan already records the cost: "every check from here runs against offline mode, including M8's and M9's vanilla lanes." A local vanilla server in offline mode is a complete oracle for movement, so this milestone is not blocked — but the gate must state that it ran offline, and nothing here should be read as evidence about online-mode behaviour.

## Global Constraints

- Repositories touched: `minecraft-simulation` (the adapter contract), `headless-minecraft` (prediction), `server` (authority).
- **The simulation learns nothing about packets.** No package in `minecraft-simulation` gains a protocol import in this milestone. Commands and events cross the boundary; wire types do not.
- **One kernel, two drivers.** If the client and the server need different rules to agree with vanilla, that is a finding about the rules and it is fixed in the profile, not by giving each consumer its own variant. Two variants would make the milestone's claim untrue while appearing to pass.
- The live check needs a pinned server jar, runs in offline mode, and is opt-in: an ordinary `task verify` must not download or start a server. It skips when the jar is absent, the way the M8.2 oracle skips without a JDK.
- No decompiled Java, no Mojang asset, and no game jar committed, in any repository.
- Formatting and commit conventions as in the other plans. No `Co-Authored-By` or `Claude-Session` trailer.

## Design decisions this plan settles

### A correction is defined before it is counted

The exit criterion is "zero corrections", and a vanilla 1.8.9 server sends the player a position-and-look packet during login as a matter of course. Counting that as a correction would make the gate impossible to pass; ignoring position packets generally would make it impossible to fail.

A correction is therefore defined as a clientbound player position-and-look packet received **after** the client has acknowledged the login sequence and begun sending its own positions. The acknowledgement boundary is explicit state in the prediction loop, not a timeout, because a timeout would make the gate flaky on a slow machine and flaky gates get deleted.

Two further signals are counted alongside it, because a server can disagree without teleporting: the server's "moved wrongly" and "moved too quickly" log lines, and any velocity packet the scripted input should not have provoked. The gate reports all three and requires all three to be zero. Reading the server log is what catches the case where the server tolerates our drift for a while — a check that watched only for teleports would call that a pass.

### Prediction forks a snapshot; a correction throws the fork away

This is the parent design's rule and it is adopted unchanged: "The client can apply predicted change sets to a forked snapshot. After a server correction, the client discards the affected fork and replays retained commands from the new authoritative snapshot."

`runtime.Memory.Snapshot` from M8.3 is the fork. The revision check is what makes discarding safe: a predicted change set is computed against the fork's revision, so it can never be applied to the authoritative store by accident. That is the property M8.3's second gate proved, and this milestone is where it earns its place.

Retained commands are kept in a bounded ring. Unbounded retention is a memory leak that only appears under a server that stops responding, which is precisely when a client is least able to afford one.

### The server is authoritative and does not predict

`server` drives `runtime.Runner` directly: build input from its store, step, apply. It keeps no fork, because it has nothing to reconcile against. That asymmetry is the whole point of the two adapters — they share the kernel and differ in what they do with a result — and a shared "adapter" abstraction that hid it would be a worse design than two small drivers.

### The adapter contract is small enough to be obviously correct

`minecraft-simulation` gains one package, `adapter`, holding the seam both consumers implement: how a tick's input is assembled, and what a consumer does with a result. It holds no networking, no packet types, and no goroutines. If it grows past a couple of hundred lines, the wrong thing is being pushed into it.

## File structure

```text
minecraft-simulation/
  adapter/adapter.go          The seam: Source, Sink, and a Drive helper
  adapter/adapter_test.go

headless-minecraft/
  version/action.go           The intents and the measured cadence rule — landed
  client/action.go            The outbound action path, Client.Do — landed
  client/action_test.go       — landed
  predict/predict.go          The prediction loop, forks, and reconciliation
  predict/reconcile.go
  predict/*_test.go
  internal/vanilla/server.go  Launching a pinned vanilla server for tests
  client/vanilla_e2e_test.go  The zero-corrections gate
  Taskfile.yml                A test:vanilla task, opt-in

server/
  internal/sim/driver.go      Driving the kernel authoritatively
  internal/sim/driver_test.go
```

---

## Task 1: An outbound action path for the client — complete, landed ahead of this plan

**Repository:** `headless-minecraft`

- [x] Landed as `version/action.go` and `client/action.go` before this plan
  started executing, with `ActionRespawn` following it. Nothing is left in this
  task.

What landed matches the sketch in shape — `Client.Do` is the only public entry
point, serialized against the loop's own replies, refusing before play
(`ErrNotInPlay`) and after close, surfacing a failed write — and differs in
five particulars Task 4 must know before consuming it:

- **The intents live in `version`, not `client`.** `version.Action` is the
  interface — one method, `ActionKind() string` — and `client` re-exports every
  type as an alias, so `client.ActionMove` and `version.ActionMove` are the
  same type. Either import works.
- **`version.Adapter` gained `EncodeAction`**, the seam the sketch asked for,
  plus `version.ErrUnsupportedAction` and the shared `UnsupportedAction`
  helper, so both protocols refuse an inexpressible intent the same way.
- **Every movement intent carries `HorizontalCollision`** alongside `OnGround`.
  Protocol 775 carries it in its movement flags; protocol 47 has no field for
  it and drops it. The kernel computes it — M8.2's collision flags — so the
  prediction loop must feed it from the tick's result rather than defaulting it
  false.
- **`ActionRespawn` exists too**, beyond the sketch, because a dead client that
  cannot respawn is stuck. This milestone's scenarios never die, but the loop
  should not assume every action is a movement.
- **The cadence rule is documented and deliberately not encoded.** The sketch
  asked for the rule to be established from the reference workspace and
  recorded in the doc comment; `version/action.go` records it, read off a real
  1.8.9 client's own traffic — 3,703 movement packets over five minutes, 3,700
  agreeing, the two exceptions at the login boundary. Moved means squared
  distance from the last reported position above `9.0e-4`, or twenty ticks
  since a position was reported; rotated means the yaw or pitch differs
  exactly. Moved and rotated send position-look, moved alone position, rotated
  alone look, neither a bare ground flag. Deciding which intent a tick warrants
  belongs to the caller that tracks the previous tick — nothing in `Do` or the
  adapters guesses. **That makes the cadence Task 4's job**, and it converts
  Task 5's likeliest first-failure cause, "the packet cadence", from a rule the
  gate discovers into one the loop implements from a documented, measured spec.

The sketched tests exist as `client/action_test.go`: encoding asserted against
the generated codecs on both protocols, refusal before play and after close,
and serialization of concurrent `Do` calls. One addition worth using: `Do`
publishes every action as a `PacketSent` event, so the gate can watch actions
and corrections in one subscription.

---

## Task 2: The adapter seam

**Repository:** `minecraft-simulation`

**Files:**
- Create: `adapter/adapter.go`
- Test: `adapter/adapter_test.go`

**Interfaces:**
- Produces:
  - `type Source interface { Tick() sim.Tick; Commands() []sim.Command; Limits() sim.Limits; Scope() sim.Scope }`
  - `type Sink interface { Apply(sim.ChangeSet) error; Observe(sim.TickResult) }`
  - `func Drive(ctx context.Context, kernel sim.Kernel, store runtime.Store, source Source, sink Sink) (sim.TickResult, error)`

`Drive` is the one piece of logic both consumers share: assemble a `sim.TickInput` from the store and the source, step the kernel, hand the result to the sink, and apply the change set only when the result is complete. Everything version-specific, network-specific, or policy-specific is on the far side of `Source` and `Sink`.

`Observe` receives every result, complete or not, because a client needs to know a tick was incomplete in order to stop predicting until its chunks arrive, and a server needs to log it.

- [x] **Step 1: Write the failing test**

- [x] **Step 2 to 6: Fail, implement, verify, check, commit**

**Done.** The seam landed ahead of this milestone's execution and is what both
drivers below are written against: `Source`, `Sink`, and a `Drive` that assembles
the input, steps the kernel, observes every result, and applies only a complete
one.

```bash
git add adapter/
git commit -m "feat(adapter): add the consumer seam over one kernel"
```

---

## Task 3: The server driver

**Repository:** `server`

**Files:**
- Create: `internal/sim/driver.go`
- Test: `internal/sim/driver_test.go`

The authoritative side is the simpler of the two and it goes first, because a bug here is a bug in the shared path and finding it without a network in the way is cheaper.

The driver owns a `runtime.Memory`, a kernel built from the v1_8 profile, and a per-connection command queue that a player's movement packets feed. It steps once per game tick and turns the result's domain events into outbound packets.

`server` is the harness M9 and M10 both need, so this driver must not become the only way to run it: a server with no simulation attached must still accept connections, exactly as it does today.

- [x] **Step 1: Write the failing test**

- [x] **Step 2 to 6: Fail, implement, verify, check, commit**

**Done 2026-08-18**, as `server` `2392043`. Two decisions worth recording:

- **An intent whose kind no phase handles is refused at the queue**, not passed
  to a tick that would ignore it. The plan asked for a rejected outcome; a tick
  produces outcomes only for commands its phases answer, so a silently ignored
  intent would reach a connection as neither applied nor rejected. Refusing where
  it arrives is the same information, earlier.
- **The driver is a component, not a stage.** Nothing in the server's tick loop
  was changed, and a server with no driver attached still accepts connections and
  serves its world exactly as before — which is the property the plan asked for
  and the reason attaching it belongs to M9 rather than here.

Adding the dependency carried `minecraft-protocol` from v0.2.0 to v0.5.0 in that
repository. Its whole suite passes on it.

```bash
cd ../server
git add internal/sim/
git commit -m "feat(sim): drive the shared kernel authoritatively"
```

---

## Task 4: The prediction loop

**Repository:** `headless-minecraft`

**Files:**
- Create: `predict/predict.go`, `predict/reconcile.go`
- Test: `predict/*_test.go`

**Interfaces:**
- Produces:
  - `type Loop struct { ... }` with `New`, `Start`, `Close`
  - `func (l *Loop) Input(movement.Input)` — the caller's intent for the next tick
  - `func (l *Loop) Predicted() world.PlayerView` — what the client currently believes
  - `type Correction struct { Tick sim.Tick; From, To geom.Vec3 }` published as an event

The loop, once per tick: take the caller's input, build a command, `Drive` against the forked store, send the result through `client.Do`, and retain the command.

**The loop owns the packet cadence.** The landed action API documents the measured rule in `version/action.go` and deliberately encodes none of it, so choosing the intent is this loop's job: track the last reported position and rotation and the twenty-tick counter, and send `ActionMoveLook`, `ActionMove`, `ActionLook`, or `ActionGround` accordingly, carrying `OnGround` and `HorizontalCollision` from the tick's own collision result. A server reads the choice as information — a position for a tick where nothing moved is itself a disagreement — so the cadence is gate-relevant behaviour, not formatting.

On a correction: replace the fork with a store built from the authoritative observed snapshot, drop retained commands the server has acknowledged, replay the rest, and publish a `Correction` event. Publishing it matters beyond diagnostics — the gate counts these, and a consumer wants to know its prediction was wrong.

Reconciliation is where the subtle bugs live, so the tests are the deliverable as much as the code:

- A correction that agrees with the prediction to within the wire's precision must still reset the fork, because 1.8.9 transmits positions as fixed point in units of one thirty-second of a block and a client that treated a rounding difference as a disagreement would fight the server forever.
- A correction arriving while a tick is in flight must not be applied to a half-built state.
- Retained commands must be bounded, and the bound must be reached in a test rather than reasoned about.
- The cadence must match the measured rule without a server present: an idle run reports a bare ground flag each tick and a forced position on the twenty-first, and a drift below the `9.0e-4` squared-distance threshold must not report a position before the counter forces one. A recording writer makes this assertable tick by tick.

- [x] **Step 1: Write the failing tests against a scripted server**

The client's existing end-to-end lane already stands up a stub upstream; reuse it. A stub server that teleports on demand is what makes correction handling testable without a jar.

- [x] **Step 2 to 6: Fail, implement, verify, check, commit**

```bash
git add predict/
git commit -m "feat(predict): predict locally and reconcile against corrections"
```

**Done 2026-08-18**, as `headless-minecraft` `b639a28`. Three things the plan did
not anticipate:

- **The world needed a bridge.** A prediction runs over the terrain the server
  described, and the observed chunks carry block states rather than simulation
  handles. `predict.Terrain` turns one into the other, and the resolver is per
  version because the two protocols disagree about what a block state is: one
  packs an identifier and four bits of metadata into a number and the other
  carries the flattened state that replaced them.
- **The fork is a store, not a copy of one.** Implementing `runtime.Store` over
  the observed world and a local body is cheaper than copying chunks per tick and
  keeps the server's blocks read-only: a change set that tried to write one is
  refused, because a client does not predict block changes in this milestone and
  a silent local edit would be undone by the next chunk update.
- **Reconciliation drops everything it retained.** The plan asked for retained
  commands to be replayed past the acknowledged point; this protocol carries no
  acknowledgement number, so there is nothing to match against. What the server
  corrected, it corrected in full. The bound on retention is still there and
  tested, because the queue is what would leak when a server stops answering.

---

## Task 5: The vanilla gate

**Repository:** `headless-minecraft`

**Files:**
- Create: `internal/vanilla/server.go`, `client/vanilla_e2e_test.go`
- Modify: `Taskfile.yml`

**Interfaces:**
- Produces: `vanilla.Start(t *testing.T, opts vanilla.Options) *vanilla.Server`, with `Addr()`, `LogLines()`, and cleanup registered on the test.

This is the exit criterion. It starts a pinned vanilla 1.8.9 server in offline mode with a fixed seed and a flat or otherwise deterministic world, connects the headless client, runs scripted input, and requires zero corrections.

The harness must:

- **Pin the jar by digest**, and skip with a clear message when it is absent. `task verify` must not download it. The M8.2 oracle's skip behaviour is the model: meaningful where the artifact exists, green where it does not.
- **Write `eula.txt` and a `server.properties`** with `online-mode=false`, a fixed `level-seed`, `level-type=flat` for the first scenarios, spawn protection off, and view distance small enough to keep chunk loading quick.
- **Wait for readiness by reading the log**, not by sleeping, and fail with the captured log if the server never becomes ready.
- **Capture the whole log** so the assertions can search it, and print it on failure. A failing gate whose output is "1 correction" and nothing else will not be diagnosable by whoever sees it next.

Scenarios, matching M8.4's suite: walk a straight line, sprint, jump repeatedly, sneak, fall from height, and walk into a wall. Each runs for at least 200 ticks after the login boundary.

Assertions per scenario:

1. Zero corrections after the acknowledgement boundary.
2. No "moved wrongly" or "moved too quickly" in the server log.
3. The client's predicted position and the server's last acknowledged position agree to within the wire's fixed-point precision.

- [x] **Step 1: Build the harness and confirm it starts and stops cleanly**

Confirm no orphaned process survives a failed test, on a timeout as well as on a normal failure. A test suite that leaks server processes will be disabled by the next person who runs it.

- [x] **Step 2: Add the opt-in task**

```yaml
  test:vanilla:
    desc: Run the vanilla 1.8.9 conformance lane; requires a pinned server jar
    deps: [deps]
    cmds:
      - go test ./client/ -run TestVanilla -tags vanilla -timeout 20m {{.CLI_ARGS}}
```

- [x] **Step 3: Run it, and expect to find something**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/headless-minecraft && devbox run -- task test:vanilla`

This is the first time the simulation meets a real server, and something will be wrong. The likely causes, in the order they are worth checking:

- **The packet cadence.** Which movement packet, how often, and whether the ground flag matches. This is the most likely cause and has nothing to do with physics. The rule is now measured and documented in `version/action.go` and Task 4 implements it from that spec, so a cadence failure here means the loop diverged from the documented rule — or the capture's two login-boundary exceptions matter after all.
- **The tick alignment.** Our tick boundary against the server's twenty per second.
- **The acknowledgement boundary.** Corrections counted that are not corrections.
- **A constant.** Least likely, because M8.4 checked them against the game's own bytecode, and the most important reason that milestone was built the way it was.

Record what it actually was. That is the milestone's most valuable output, and if it turns out to be a constant after all, then M8.4's oracle missed something and that fact belongs in the master plan.

- [x] **Step 4 to 6: Verify, check, commit**

```bash
git add internal/vanilla/ client/vanilla_e2e_test.go Taskfile.yml
git commit -m "test(client): gate movement on zero corrections from vanilla 1.8.9"
```

**Done 2026-08-18**, as `headless-minecraft` `d5ab174`. It passed on the first
run that reached a server, which is not what this step expected — so what it
found is worth stating precisely.

**What was wrong was never physics.** Three things stood between the first run
and a green one, and all three were about standing a server up and reading it:

- **A server of this age does not run on a modern JVM as shipped.** Its bundled
  netty reaches a direct buffer's address through `sun.misc.Unsafe`, which is no
  longer permitted, so every read failed with "Unable to access address of
  buffer" and no client completed a handshake. The fix is the server's own
  `use-native-transport=false`; the JVM-side netty properties alone do not reach
  it, because the transport is chosen by the server rather than by netty.
- **The client observes nothing unless a world is installed.** `WithWorld` is
  opt-in, and a prediction over an empty world disagrees with the server about
  everything. The gate's first green scenario was the one that noticed.
- **The third assertion, as the plan wrote it, could not be measured.** A 1.8.9
  server tells a client its position only to correct it: silence is how it
  accepts one. So with zero corrections there is no "last acknowledged position"
  to compare against — the silence *is* the agreement. The assertion now compares
  the two only when the server has spoken, and a fourth was added in its place:
  that the client kept reporting throughout, since a client that went quiet would
  draw no corrections either and would have proved nothing.

**The cadence came out exactly as the measured rule predicts**, which is the
milestone's most useful confirmation. Standing sent 210 bare ground flags and 10
forced positions over 220 ticks; turning on the spot sent 207 looks and 12
positions; walking and sneaking sent a position and rotation every tick; jumping,
which moves without turning, sent 220 bare positions. The rule was read off a
capture in M8.8 Task 1 and implemented from that reading, and a real server
accepted all of it.

---

## Task 6: Both profiles, and the milestone record

**Files:** the vanilla harness, `MASTER_PLAN.md`, `README.md`, `CHANGELOG.md` in each repository

- [x] **Step 1: Run the 26.1.2 lane**

**Done 2026-08-18**, as `headless-minecraft` `48ba5f9`, and it passed on its first
run too. The same six scenarios against a real 26.1.2 server: zero corrections,
no complaint in the log.

The plan expected this lane to be the risky one — "a failure here is expected to
be a physics problem rather than a cadence problem" — on the assumption that
26.1.2 might have no jar-backed oracle. M8.7 built one, and this is what that
bought: the physics arrived already checked against that version's own bytecode,
and the only differences this lane had to know about were on the wire. This
version names its correction packet differently and expects a teleport
confirmation the client already sent. **The cadence rule needed no change at
all** — it was measured off a 1.8.9 client and a 26.1.2 server accepts it.

Repeat Task 5 against a pinned 26.1.2 server with the v26_1 profile. If M8.7 found no jar-backed oracle for that version, this is the first real verification of its constants, and a failure here is expected to be a physics problem rather than a cadence problem — the reverse of the 1.8.9 lane's likely causes.

- [x] **Step 2: Record the milestone honestly**

State in the master plan: that both adapters run one kernel; that the gate passed in **offline mode** and says nothing about online mode until M6.4 is picked up; which scenarios ran on which version; and what the first run found.

- [x] **Step 3: Changelog, verify, commit**

```bash
git commit -m "docs(plan): close M8, and what the vanilla gate found"
```

---

## Definition of done

- One kernel, built from one profile, is driven by both consumers through `adapter.Drive`. Neither consumer has its own movement rules.
- `minecraft-simulation` gained no protocol import.
- The client can send version-neutral actions on protocol 47 and protocol 775, serialized against its writer, with errors surfaced. Already true — landed ahead of this plan — and the loop must additionally implement the cadence rule those actions deliberately leave to it.
- Prediction forks the store, a correction discards the fork and replays retained commands from the authoritative snapshot, retention is bounded, and a difference within the wire's fixed-point precision does not cause the client to fight the server.
- **Scripted walk, sprint, jump, sneak, fall, and collide draw zero corrections from a real vanilla 1.8.9 server**, with no "moved wrongly" or "moved too quickly" in its log, over at least 200 ticks per scenario after the login boundary.
- The same suite runs against 26.1.2 with the v26_1 profile.
- The vanilla lane is opt-in, pins its jar by digest, skips cleanly when it is absent, leaks no server process, and prints the server log on failure.
- `server` still accepts connections with no simulation attached.
- `devbox run -- task verify` passes in all three repositories.
- The master plan records that the gate ran offline, and what the first run found.

## Follow-on

M8 is closed by this milestone. M9 begins, and its first stage is already built: M9.1's capture consumer landed on `relay` and its live check is pending, which is now cheaper to run because this milestone produced a working vanilla harness that M9.1's check can reuse rather than reinvent.

M6.4 remains postponed, and this milestone is the reason to reconsider it: every gate from here is offline-mode only, and the first mechanic whose behaviour differs under authentication will have no check at all until it is picked up.
