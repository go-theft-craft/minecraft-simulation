# M8.3 Kernel Contracts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver `sim`, `entity`, `runtime`, the block-state half of `world`, and an in-memory store, so that a tick which performs no work runs deterministically, carries its own digest, and produces a change set that a store at a newer revision refuses.

**Architecture:** `sim` owns the tick contract: what goes in, what comes out, how a result is canonically encoded, and the kernel that runs a profile's phases and nothing else. `entity` owns entity identity and body state. `world` gains an opaque block-state handle beside the collision shapes M8.2 added. `runtime` owns the store, the revision check, and a tick runner. No package here knows anything about movement, and none imports a protocol package.

**Tech Stack:** Go 1.26.6, standard library only, Devbox, go-task, `gofumpt` plus `gci`.

**Status:** complete. Six places where the code differs from this plan — the home of
`Locomotion`, `Runner.Step`'s signature, `TickState.Commands`, where
`ProfileID.validate` landed, why the empty-tick digest needed no re-pinning, and
the two `Clone` methods — are recorded in
`docs/superpowers/notes/2026-08-17-m8-3-m8-4-implementation-record.md`. Read that
before treating this plan as a description of the code.

## Global Constraints

- Module is `github.com/go-theft-craft/minecraft-simulation`. Go directive is `1.26.6`.
- **No protocol imports.** `geom`, `world`, `entity`, `collision`, `sim`, and `runtime` must not import `github.com/go-theft-craft/minecraft-protocol/...`. Only `profile/java/v1_8`, which M8.4 adds, may.
- Standard library only. No new module dependencies.
- Determinism: no wall clock, no global random state, and no result that depends on map iteration order. Every returned slice has a defined order, and every test that reads a map sorts before comparing.
- All arithmetic here is `float64`. This milestone performs no physics, so it introduces no `float32`. See the step-height note under "What M8.2 learned" for why that is a rule about arithmetic rather than about storage.
- No decompiled Java, no Mojang asset, and no source text from the reference workspace is committed.
- Formatting is `gci` with the section order `standard, default, prefix(github.com/go-theft-craft), prefix(github.com/go-theft-craft/minecraft-simulation)`, then `gofumpt`. Run `devbox run -- task fmt` before every commit.
- Commit messages use Conventional Commits. Never add a `Co-Authored-By` or `Claude-Session` trailer.
- Every task ends with `devbox run -- task verify` passing.

## What M8.2 learned that this plan must respect

M8.2 finished with a differential harness that runs a real 1.8.9 server jar, and it found two things by asking the game rather than by reading a plan. Both bear on the contracts below.

**A quantity's width is part of its value.** `stepHeight` is a `float` in Java, widened where it is applied, so the player's value is `0.6000000238418579` and not `0.6`. `physics.json` already stores the widened form, so M8.4's profile will hand over the right number provided nothing rounds it on the way. That is why `MotionConstants` below mirrors `data.EntityMotion` field for field instead of inventing a tidier shape: every reshaping is a chance to lose the low bits.

**A recorded value is not always the obvious one.** After a step-up, vanilla records the settle as the tick's Y motion rather than the climb plus the settle. `collision.Result` reports what the game reports. Phases in M8.4 will consume `Result` directly and must not "correct" it.

The harness is reusable and M8.3 gets no benefit from it, because nothing here has a vanilla counterpart: the game has no object corresponding to a change set or a digest. The first milestone that can use it again is M8.4, and it should, rather than waiting for M8.8's live server. That obligation is recorded in this plan's follow-on section so it is not lost.

## Design decisions this plan settles

The sequencing design froze the seams and left the interiors open. These are the interior decisions, stated once here rather than repeated per task.

### Dependency direction, and one deviation from the parent layout

Packages depend in one direction only:

```text
geom  ->  world  ->  entity  ->  sim  ->  runtime
              \                   ^
               \-> collision -----/
```

`world.BlockRef` lives in `world` because block identity is a world concept, and `entity.Family` lives in `entity` for the same reason. `sim` composes both.

The parent design's package layout lists a `profile/` package for "profile contracts, manifests, and custom builder". This plan puts the `Profile` interface in `sim` instead, and the deviation is deliberate: a profile supplies the kernel's tick phases, a `Phase` is written against the kernel's own tick state, and so a `profile` package holding `Profile` would have to import `sim` while `sim` needs `Profile` — an import cycle. `profile/java/v1_8` still arrives in M8.4 as the concrete implementation, and a `profile` package for the builder and manifest can still arrive later, holding everything except the interface the kernel is written against.

### A change set carries block handles, never shapes

`geom.Shape` keeps its boxes unexported so a shared shape cannot be mutated, which is right for M8.2 and wrong for a change set: an operation must be canonically encodable, and an opaque type is not. Operations therefore carry a `world.BlockRef`, and the store resolves it to a shape through the profile. That adds `Shape(world.BlockRef) (geom.Shape, bool)` to the `Profile` interface, which is the same seam the frozen `Slipperiness` uses and the natural home for the adaptation of `data.CollisionShapes` that M8.4 performs anyway.

### The digest covers the result, so commands may be an interface

The frozen contract puts the digest on the result. Nothing hashes a `TickInput`. That frees `Command` to be an interface, which matters because M8.3 cannot know what a movement intent looks like and should not guess at a payload union. What the digest does cover is the ordered command *outcomes*, which are concrete.

### Events carry a namespaced string kind for now

A `uint8` event enum invented before any rule emits an event would be renamed by M8.4. Events carry `Kind string`, namespaced like `movement.collided`. Strings encode canonically, cost nothing at this scale, and survive being added to.

### Negative zero and NaN are canonicalized when encoding

`math.Float64bits(-0.0)` differs from `math.Float64bits(0.0)`, and a step-up settle can legitimately produce a negative zero. Two results that differ only in the sign of a zero describe the same state: adding either to a coordinate gives the same coordinate, and `value < 0` is false for both. The encoder therefore folds `-0.0` to `0.0`. It also folds every NaN to one bit pattern, because NaN payloads are not portable, and a separate invariant test asserts that no NaN reaches a result at all — hashing a NaN silently is the failure mode worth avoiding, not the bit pattern.

### Operation order is semantic; dependency order is not

Operations keep insertion order — phase order, then order within a phase — because a later operation may overwrite an earlier one and sorting would change meaning. Read dependencies are deduplicated and sorted canonically, because their order is an artefact of how a rule happened to walk the world, and a digest must not depend on that.

### The store copies, and says so

`runtime.Memory` is the reference store the parent design asks for, not a fast one. `Snapshot` copies its maps. A copy-on-write store belongs to whichever consumer measures a problem, and pretending otherwise now would bake a structure nobody has evidence for.

## What this plan deliberately does not decide

- Any movement rule, and therefore the concrete contents of a phase list. M8.4.
- The cross-platform digest matrix. M8.6 owns the runners; this plan owns the encoding the matrix will hash, which the sequencing design explicitly allows to land here.
- The fixture file format, and `mctest`. M8.4 needs fixtures and should define them against real rules.
- Persistence, scheduled block work, fluids, and dimensions. Later subprojects.

## File structure

```text
minecraft-simulation/
  world/state.go              BlockRef, StateView, View; Blocks gains block handles
  world/state_test.go
  entity/entity.go            ID, Family, State
  entity/entity_test.go
  entity/view.go              View, and an in-memory Bodies implementation
  entity/view_test.go
  sim/identity.go             Revision, Tick, ProfileID
  sim/identity_test.go
  sim/completeness.go         Dependency, Completeness
  sim/completeness_test.go
  sim/change.go               OpKind, Op, ChangeSet
  sim/change_test.go
  sim/command.go              Command, CommandOutcome
  sim/event.go                DomainEvent, PresentationEvent
  sim/limits.go               Limits, ErrLimitExhausted
  sim/random.go               RandomState, RandomStream
  sim/contract_test.go        Covers command.go, event.go, limits.go, random.go
  sim/canon.go                The canonical encoder
  sim/canon_test.go
  sim/result.go               TickInput, TickResult, Scope
  sim/digest.go               Digest, and encoding a result
  sim/digest_test.go
  sim/profile.go              Profile, MotionConstants, Phase, TickState
  sim/kernel.go               NewKernel, Step
  sim/kernel_test.go
  sim/fake.go                 StaticProfile, for tests and later fixtures
  runtime/store.go            Store, ErrStaleRevision
  runtime/memory.go           The in-memory reference store
  runtime/memory_test.go
  runtime/runner.go           Runner, Step, Fork
  runtime/runner_test.go
```

`sim/fake.go` ships in the non-test build for the same reason `world/fake.go` does: `runtime` cannot be tested without a profile, and a later `mctest` needs one to build fixtures against. It is small and it implements only the interface.

---

## Task 1: Revisions, ticks, and profile identity

**Files:**
- Create: `sim/identity.go`
- Test: `sim/identity_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Revision uint64`
  - `type Tick uint64`
  - `type ProfileID struct { Edition, GameVersion, RulesRevision string }`
  - `func (p ProfileID) String() string`
  - `func (p ProfileID) IsZero() bool`

`Revision` counts applied change sets and `Tick` counts simulated ticks. They are separate types because they move independently: a tick that is incomplete, cancelled, or rejected advances the tick counter and produces no revision. Giving them one type would let a caller pass either where the other belongs, and the compiler would not notice.

- [x] **Step 1: Write the failing test**

```go
package sim

import "testing"

func TestProfileIDString(t *testing.T) {
	id := ProfileID{Edition: "java", GameVersion: "1.8.9", RulesRevision: "1"}
	if got, want := id.String(), "java/1.8.9@1"; got != want {
		t.Fatalf("String = %q, want %q", got, want)
	}
}

func TestProfileIDIsZero(t *testing.T) {
	if !(ProfileID{}).IsZero() {
		t.Error("the zero identity does not report itself as zero")
	}
	if (ProfileID{Edition: "java"}).IsZero() {
		t.Error("a partly filled identity reports itself as zero")
	}
}

func TestRevisionAndTickAreDistinctTypes(t *testing.T) {
	// A compile-time check with a runtime assertion attached, so that the test
	// fails loudly rather than being silently deleted if the types merge.
	var revision Revision = 7
	var tick Tick = 7
	if uint64(revision) != uint64(tick) {
		t.Fatal("the two counters disagree on the value seven")
	}
}
```

- [x] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/`

Expected: FAIL to build, with `undefined: ProfileID`.

- [x] **Step 3: Write the implementation**

```go
// Package sim owns the tick contract: what one simulation step consumes, what
// it produces, how a result is canonically encoded, and the kernel that runs a
// profile's phases.
//
// Nothing here implements a game rule. A profile supplies the rules; this
// package supplies the shape they run in, the record they produce, and the
// guarantees that record carries. Nothing in this package reads the clock, uses
// global random state, or depends on map iteration order.
package sim

import (
	"fmt"
	"strings"
)

// Revision counts change sets a store has applied. A change set names the
// revision it was computed against and applies only to a store still holding
// it.
type Revision uint64

// Tick counts simulated ticks. It is not a Revision: an incomplete, cancelled,
// or rejected tick advances this counter and produces no revision at all.
type Tick uint64

// ProfileID names a set of rules. The game version and the rules revision are
// separate fields because a fix to our implementation of 1.8.9 changes the
// second without touching the first, and a replay must be able to tell those
// apart.
type ProfileID struct {
	// Edition is the game edition, such as "java".
	Edition string
	// GameVersion is the version whose behaviour is reproduced, such as "1.8.9".
	GameVersion string
	// RulesRevision is our implementation revision of those rules.
	RulesRevision string
}

// String returns the identity as edition/version@rules.
func (p ProfileID) String() string {
	return fmt.Sprintf("%s/%s@%s", p.Edition, p.GameVersion, p.RulesRevision)
}

// IsZero reports whether every field is empty.
func (p ProfileID) IsZero() bool {
	return p.Edition == "" && p.GameVersion == "" && p.RulesRevision == ""
}

// validate reports why the identity cannot name a profile.
func (p ProfileID) validate() error {
	var missing []string
	if p.Edition == "" {
		missing = append(missing, "edition")
	}
	if p.GameVersion == "" {
		missing = append(missing, "game version")
	}
	if p.RulesRevision == "" {
		missing = append(missing, "rules revision")
	}
	if len(missing) != 0 {
		return fmt.Errorf("sim: profile identity is missing its %s", strings.Join(missing, ", "))
	}

	return nil
}
```

- [x] **Step 4: Run the test to verify it passes**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -v`

Expected: PASS, three tests. `validate` is unused until Task 9; if the linter objects to that, move this function to Task 9 rather than exporting it to satisfy the linter.

- [x] **Step 5: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [x] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add sim/identity.go sim/identity_test.go
git commit -m "feat(sim): add revisions, ticks, and profile identity"
```

---

## Task 2: Block handles and the state view

**Files:**
- Create: `world/state.go`
- Modify: `world/fake.go`
- Test: `world/state_test.go`

**Interfaces:**
- Consumes: `geom.BlockPos`, `geom.Shape`; `Lookup`, `BlockView` from M8.2.
- Produces:
  - `type BlockRef uint32`
  - `type StateView interface { BlockState(pos geom.BlockPos) (BlockRef, Lookup) }`
  - `type View interface { BlockView; StateView }`
  - `func (b *Blocks) SetBlock(pos geom.BlockPos, ref BlockRef, shape geom.Shape)`
  - `func (b *Blocks) BlockState(pos geom.BlockPos) (BlockRef, Lookup)`

A `BlockRef` is an opaque handle a profile assigns to one block state. This package never interprets it: only the profile that minted it can say what it means, which is what keeps `world` free of any version's block numbering. Movement needs it because slipperiness is a property of the block underfoot, and the profile answers that question from the handle.

`Set` and `SetAir` keep their M8.2 meaning and record the zero handle. A caller that cares about handles uses `SetBlock`. Changing what `Set` means would break every M8.2 test for no gain.

- [x] **Step 1: Write the failing test**

```go
package world

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestBlockStateIsUnknownUntilSet(t *testing.T) {
	blocks := NewBlocks()

	ref, lookup := blocks.BlockState(geom.BlockPos{X: 1})
	if lookup != LookupUnknown {
		t.Fatalf("lookup = %v, want LookupUnknown", lookup)
	}
	if ref != 0 {
		t.Errorf("ref = %d, want the zero handle for an unknown position", ref)
	}
}

func TestSetBlockRecordsBothTheHandleAndTheShape(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{X: 2, Y: 3, Z: 4}
	blocks.SetBlock(pos, 77, geom.FullCube())

	ref, lookup := blocks.BlockState(pos)
	if lookup != LookupShape || ref != 77 {
		t.Fatalf("BlockState = (%d, %v), want (77, shape)", ref, lookup)
	}

	shape, lookup := blocks.CollisionShape(pos)
	if lookup != LookupShape || shape.Len() != 1 {
		t.Fatalf("CollisionShape = (%d boxes, %v), want (1, shape)", shape.Len(), lookup)
	}
}

func TestSetBlockWithAnEmptyShapeIsAirThatStillCarriesItsHandle(t *testing.T) {
	// Air is a block state too. A profile that wants to ask about the block
	// underfoot must get an answer for air, and "air" is not the same answer as
	// "nobody told me".
	blocks := NewBlocks()
	pos := geom.BlockPos{Y: 9}
	blocks.SetBlock(pos, 5, geom.EmptyShape())

	ref, lookup := blocks.BlockState(pos)
	if lookup != LookupAir {
		t.Fatalf("lookup = %v, want LookupAir", lookup)
	}
	if ref != 5 {
		t.Errorf("ref = %d, want the handle to survive being air", ref)
	}
}

func TestSetKeepsItsMeaningAndRecordsTheZeroHandle(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{Z: 1}
	blocks.Set(pos, geom.FullCube())

	if ref, lookup := blocks.BlockState(pos); ref != 0 || lookup != LookupShape {
		t.Fatalf("BlockState = (%d, %v), want (0, shape)", ref, lookup)
	}
}

func TestForgetClearsTheHandleToo(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{X: -1}
	blocks.SetBlock(pos, 12, geom.FullCube())
	blocks.Forget(pos)

	if _, lookup := blocks.BlockState(pos); lookup != LookupUnknown {
		t.Fatalf("lookup = %v, want LookupUnknown", lookup)
	}
}

// Blocks satisfies the whole view, not just the collision half.
var _ View = (*Blocks)(nil)
```

- [x] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./world/`

Expected: FAIL to build, with `undefined: View` and `b.SetBlock undefined`.

- [x] **Step 3: Write `world/state.go`**

```go
package world

import "github.com/go-theft-craft/minecraft-simulation/geom"

// BlockRef is an opaque handle for one block state, minted by a profile.
//
// This package never interprets it. Only the profile that produced a handle can
// say which block and which state it names, which is what keeps world free of
// any particular version's block numbering. A profile answers questions about a
// handle, such as its slipperiness or its collision shape.
//
// The zero handle carries no meaning. It is what an implementation records when
// a caller supplied a shape without a handle, and a profile is free to treat it
// as unknown.
type BlockRef uint32

// StateView answers which block state occupies a cell.
//
// The lookup follows the same three-way rule as CollisionShape: a caller can
// tell known air from a known block from a region nobody has described.
type StateView interface {
	BlockState(pos geom.BlockPos) (BlockRef, Lookup)
}

// View is everything the kernel reads about blocks in one tick.
//
// An implementation must be deterministic and must stay valid for the whole of
// a tick: the same position answers the same way from the first phase to the
// last.
type View interface {
	BlockView
	StateView
}
```

- [x] **Step 4: Extend `world/fake.go`**

Replace the `shapes` field and the methods that touch it, so that a cell records both a handle and a shape:

```go
// cell is one described block: its profile handle and its collision shape.
type cell struct {
	ref   BlockRef
	shape geom.Shape
}

// Blocks is an in-memory View. Every position starts unknown, so a test that
// means "empty space" has to say so, and a test that forgets to describe a
// region gets the unknown path rather than a silent floor of air.
//
// Blocks is not safe for concurrent modification. Build it, then read it.
type Blocks struct {
	cells map[geom.BlockPos]cell
}

// NewBlocks returns an empty view in which every position is unknown.
func NewBlocks() *Blocks {
	return &Blocks{cells: make(map[geom.BlockPos]cell)}
}

// Set records a block shape under the zero handle. An empty shape records air,
// because a block nothing collides with is indistinguishable from air to a
// caller that only asked about collision.
func (b *Blocks) Set(pos geom.BlockPos, shape geom.Shape) {
	b.SetBlock(pos, 0, shape)
}

// SetBlock records a block state and its collision shape.
func (b *Blocks) SetBlock(pos geom.BlockPos, ref BlockRef, shape geom.Shape) {
	b.cells[pos] = cell{ref: ref, shape: shape}
}

// SetAir records that the position holds nothing collidable.
func (b *Blocks) SetAir(pos geom.BlockPos) {
	b.SetBlock(pos, 0, geom.EmptyShape())
}

// Forget returns the position to unknown.
func (b *Blocks) Forget(pos geom.BlockPos) {
	delete(b.cells, pos)
}

// Fill records the same shape for every cell in the inclusive range.
func (b *Blocks) Fill(from, to geom.BlockPos, shape geom.Shape) {
	for x := from.X; x <= to.X; x++ {
		for y := from.Y; y <= to.Y; y++ {
			for z := from.Z; z <= to.Z; z++ {
				b.Set(geom.BlockPos{X: x, Y: y, Z: z}, shape)
			}
		}
	}
}

// CollisionShape implements BlockView.
func (b *Blocks) CollisionShape(pos geom.BlockPos) (geom.Shape, Lookup) {
	found, ok := b.cells[pos]
	if !ok {
		return geom.EmptyShape(), LookupUnknown
	}
	if found.shape.IsEmpty() {
		return geom.EmptyShape(), LookupAir
	}

	return found.shape, LookupShape
}

// BlockState implements StateView.
func (b *Blocks) BlockState(pos geom.BlockPos) (BlockRef, Lookup) {
	found, ok := b.cells[pos]
	if !ok {
		return 0, LookupUnknown
	}
	if found.shape.IsEmpty() {
		return found.ref, LookupAir
	}

	return found.ref, LookupShape
}
```

- [x] **Step 5: Run the tests to verify they pass**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./world/ ./collision/ -v`

Expected: PASS. `collision` runs too, because it consumes `Blocks` and this task changed its internals; every M8.2 collision test must still pass untouched.

- [x] **Step 6: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [x] **Step 7: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add world/
git commit -m "feat(world): add opaque block handles and the combined view"
```

---

## Task 3: Entity identity, family, and body state

**Files:**
- Create: `entity/entity.go`
- Create: `entity/view.go`
- Test: `entity/entity_test.go`
- Test: `entity/view_test.go`

**Interfaces:**
- Consumes: `geom.AABB`, `geom.Vec3`.
- Produces:
  - `type ID int32`
  - `type Family uint8` with `FamilyUnknown`, `FamilyPlayer`, and `String()`
  - `type State struct { Family Family; Box geom.AABB; Motion geom.Vec3; OnGround bool; StepHeight float64 }`
  - `type View interface { Entity(id ID) (State, bool); IDs() []ID }`
  - `type Bodies struct { ... }` with `NewBodies`, `Set`, `Remove`, `Entity`, `IDs`

`ID` is `int32` because that is the width Java Edition assigns entity identifiers, and a store that silently widened them would accept identifiers no server could ever send.

`Family` names the physics family rather than the entity type. M8 is a player-only slice, so this milestone defines the player and an explicit unknown. Items and arrows already have constants in `physics.json` and will get families in M9; the type is `uint8` with room, and `String` reports an unnamed value rather than guessing.

`StepHeight` sits on the body rather than being fetched per tick because it is per-entity state in the game too, and because M8.2's `collision.Move` needs it as a widened `float64`. The profile sets it when it spawns a body.

`IDs` returns identifiers in ascending order. A view backed by a map must sort, because a tick that iterated entities in map order would produce a different change set on every run and no digest would ever be stable.

- [x] **Step 1: Write the failing tests**

```go
package entity

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestFamilyString(t *testing.T) {
	for _, test := range []struct {
		family Family
		want   string
	}{
		{FamilyUnknown, "unknown"},
		{FamilyPlayer, "player"},
		{Family(200), "Family(200)"},
	} {
		if got := test.family.String(); got != test.want {
			t.Errorf("Family(%d).String() = %q, want %q", test.family, got, test.want)
		}
	}
}

func TestStateIsComparable(t *testing.T) {
	// State is compared with == throughout the tests below and inside runtime,
	// which requires every field to stay comparable. A slice or a map added to
	// this struct would break that silently, so the check is explicit.
	first := State{Family: FamilyPlayer, Box: geom.AABB{MaxX: 1}, StepHeight: 0.5}
	second := first
	if first != second {
		t.Fatal("a copy of a state does not equal its original")
	}
}
```

```go
package entity

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func player(id ID) State {
	return State{
		Family:     FamilyPlayer,
		Box:        geom.AABB{MinX: -0.3, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3},
		StepHeight: float64(float32(0.6)),
	}
}

func TestBodiesReturnsWhatItWasGiven(t *testing.T) {
	bodies := NewBodies()
	bodies.Set(7, player(7))

	got, ok := bodies.Entity(7)
	if !ok {
		t.Fatal("Entity reported the body missing")
	}
	if got != player(7) {
		t.Fatalf("Entity = %+v, want %+v", got, player(7))
	}
}

func TestBodiesReportsAMissingEntity(t *testing.T) {
	bodies := NewBodies()
	if _, ok := bodies.Entity(1); ok {
		t.Fatal("Entity found a body that was never set")
	}
}

func TestRemoveDropsTheBody(t *testing.T) {
	bodies := NewBodies()
	bodies.Set(3, player(3))
	bodies.Remove(3)

	if _, ok := bodies.Entity(3); ok {
		t.Fatal("Entity found a removed body")
	}
}

func TestIDsAreSortedAndStable(t *testing.T) {
	bodies := NewBodies()
	for _, id := range []ID{9, -4, 0, 2, 100, -1} {
		bodies.Set(id, player(id))
	}

	want := []ID{-4, -1, 0, 2, 9, 100}
	for attempt := range 20 {
		got := bodies.IDs()
		if len(got) != len(want) {
			t.Fatalf("attempt %d: IDs returned %d entries, want %d", attempt, len(got), len(want))
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("attempt %d: IDs = %v, want %v", attempt, got, want)
			}
		}
	}
}

var _ View = (*Bodies)(nil)
```

`TestIDsAreSortedAndStable` repeats because a single pass over a small map can come out sorted by luck. Twenty passes over six keys will not.

- [x] **Step 2: Run the tests to verify they fail**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./entity/`

Expected: FAIL to build, with `undefined: Family`.

- [x] **Step 3: Write `entity/entity.go`**

```go
// Package entity owns entity identity and the body state a simulation moves.
//
// A body is geometry and motion, not a game object: there is no health here, no
// inventory, and no behaviour. Rules live in a profile, and the kernel reads
// bodies through the views in this package.
package entity

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// ID identifies one entity.
//
// It is int32 because that is the width Java Edition assigns entity
// identifiers. Widening it here would let a store hold identifiers no server
// could send, and the mismatch would surface as a lost entity rather than as an
// error.
type ID int32

// Family is the physics family a body belongs to. Movement constants are per
// family, not per entity type: every dropped item falls the same way.
type Family uint8

const (
	// FamilyUnknown is a body whose family nobody has set. A profile treats it
	// as an error rather than guessing at constants for it.
	FamilyUnknown Family = iota
	// FamilyPlayer is a player body.
	FamilyPlayer
)

// String returns the family's name.
func (f Family) String() string {
	switch f {
	case FamilyUnknown:
		return "unknown"
	case FamilyPlayer:
		return "player"
	default:
		return fmt.Sprintf("Family(%d)", uint8(f))
	}
}

// State is one body at one instant.
//
// Every field is comparable, so a change set can hold a state by value and a
// store can tell whether an operation changed anything.
type State struct {
	// Family selects the movement constants a profile applies.
	Family Family
	// Box is the body's collision box in world space.
	Box geom.AABB
	// Motion is the body's velocity, in blocks per tick.
	Motion geom.Vec3
	// OnGround is the standing state the last tick left behind.
	OnGround bool
	// StepHeight is how far the body may rise to clear an obstacle.
	//
	// Java Edition stores this as a float and widens it where the step-up
	// applies it, so a player's value is float64(float32(0.6)). A profile is
	// responsible for putting the widened value here; see the note on
	// collision.Move.StepHeight.
	StepHeight float64
}
```

- [x] **Step 4: Write `entity/view.go`**

```go
package entity

import "slices"

// View is everything the kernel reads about entities in one tick.
//
// An implementation must be deterministic and must stay valid for the whole of
// a tick.
type View interface {
	// Entity returns the body with the given identifier.
	Entity(id ID) (State, bool)
	// IDs returns every identifier the view holds, in ascending order.
	//
	// The order is part of the contract. A tick that walked entities in map
	// order would emit its operations in a different order on every run, and no
	// result digest could ever be stable.
	IDs() []ID
}

// Bodies is an in-memory View.
//
// Bodies is not safe for concurrent modification. Build it, then read it.
type Bodies struct {
	states map[ID]State
}

// NewBodies returns an empty view.
func NewBodies() *Bodies {
	return &Bodies{states: make(map[ID]State)}
}

// Set records a body.
func (b *Bodies) Set(id ID, state State) {
	b.states[id] = state
}

// Remove drops a body.
func (b *Bodies) Remove(id ID) {
	delete(b.states, id)
}

// Entity implements View.
func (b *Bodies) Entity(id ID) (State, bool) {
	state, ok := b.states[id]

	return state, ok
}

// IDs implements View, in ascending order.
func (b *Bodies) IDs() []ID {
	ids := make([]ID, 0, len(b.states))
	for id := range b.states {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	return ids
}

// Clone returns a view that does not alias this one, which is what lets a store
// hand out a snapshot a later tick cannot change underneath a reader.
func (b *Bodies) Clone() *Bodies {
	clone := &Bodies{states: make(map[ID]State, len(b.states))}
	for id, state := range b.states {
		clone.states[id] = state
	}

	return clone
}
```

- [x] **Step 5: Run the tests to verify they pass**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./entity/ -v`

Expected: PASS, six tests. `Clone` has no test of its own here; Task 10 covers it.

- [x] **Step 6: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [x] **Step 7: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add entity/
git commit -m "feat(entity): add identity, families, and body state"
```

---

## Task 4: Dependencies and completeness

**Files:**
- Create: `sim/completeness.go`
- Test: `sim/completeness_test.go`

**Interfaces:**
- Consumes: `geom.BlockPos`, `entity.ID`.
- Produces:
  - `type DependencyKind uint8` with `DependencyBlock`, `DependencyEntity`, `DependencyRegistry`, and `String()`
  - `type Dependency struct { Kind DependencyKind; Block geom.BlockPos; Entity entity.ID; Name string }`
  - `func (d Dependency) String() string`
  - `type Completeness struct { Complete bool; Missing []Dependency }`
  - `func sortDependencies(in []Dependency) []Dependency`

A `Dependency` says what a tick read, and the same type says what it could not read. One type for both is deliberate: an incomplete tick's `Missing` set is a subset of what it tried to read, and a caller that wants to load the gap and retry needs the two described identically.

The struct is a flat union rather than an interface so that it stays comparable and canonically encodable. Fields not relevant to a kind are zero, which the encoder writes anyway; a variable-length encoding per kind would save bytes nobody is counting and add a branch that could differ between platforms.

`sortDependencies` deduplicates and orders by kind, then by name, then by block coordinate, then by entity. It is unexported because callers receive already-sorted slices; it exists as a named function so Task 8 and Task 9 sort identically.

- [x] **Step 1: Write the failing test**

```go
package sim

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestDependencyKindString(t *testing.T) {
	for _, test := range []struct {
		kind DependencyKind
		want string
	}{
		{DependencyBlock, "block"},
		{DependencyEntity, "entity"},
		{DependencyRegistry, "registry"},
		{DependencyKind(9), "DependencyKind(9)"},
	} {
		if got := test.kind.String(); got != test.want {
			t.Errorf("DependencyKind(%d).String() = %q, want %q", test.kind, got, test.want)
		}
	}
}

func TestDependencyStringNamesOnlyWhatMatters(t *testing.T) {
	block := Dependency{Kind: DependencyBlock, Block: geom.BlockPos{X: 1, Y: -2, Z: 3}}
	if got, want := block.String(), "block(1,-2,3)"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}

	body := Dependency{Kind: DependencyEntity, Entity: entity.ID(42)}
	if got, want := body.String(), "entity(42)"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}

	registry := Dependency{Kind: DependencyRegistry, Name: "blocks"}
	if got, want := registry.String(), "registry(blocks)"; got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

func TestSortDependenciesDeduplicatesAndOrders(t *testing.T) {
	in := []Dependency{
		{Kind: DependencyEntity, Entity: 5},
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 1}},
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 0, Y: 4}},
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 1}},
		{Kind: DependencyRegistry, Name: "blocks"},
		{Kind: DependencyEntity, Entity: 2},
	}

	got := sortDependencies(in)
	want := []Dependency{
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 0, Y: 4}},
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 1}},
		{Kind: DependencyEntity, Entity: 2},
		{Kind: DependencyEntity, Entity: 5},
		{Kind: DependencyRegistry, Name: "blocks"},
	}
	if len(got) != len(want) {
		t.Fatalf("sortDependencies returned %d entries, want %d: %v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("sortDependencies[%d] = %+v, want %+v", index, got[index], want[index])
		}
	}
}

func TestSortDependenciesDoesNotTouchItsInput(t *testing.T) {
	in := []Dependency{
		{Kind: DependencyEntity, Entity: 5},
		{Kind: DependencyBlock, Block: geom.BlockPos{X: 1}},
	}
	first := in[0]

	sortDependencies(in)
	if in[0] != first {
		t.Fatalf("sortDependencies reordered its argument: %+v", in)
	}
}

func TestAnIncompleteResultNamesWhatWasMissing(t *testing.T) {
	completeness := Completeness{Missing: []Dependency{{Kind: DependencyBlock}}}
	if completeness.Complete {
		t.Error("a completeness with missing dependencies reports itself complete")
	}
}
```

- [x] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -run TestDependency`

Expected: FAIL to build, with `undefined: DependencyBlock`.

- [x] **Step 3: Write the implementation**

```go
package sim

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// DependencyKind says which kind of data a dependency names.
type DependencyKind uint8

const (
	// DependencyBlock names one block cell.
	DependencyBlock DependencyKind = iota + 1
	// DependencyEntity names one entity.
	DependencyEntity
	// DependencyRegistry names a whole registry, such as the block registry.
	DependencyRegistry
)

// String returns the kind's name.
func (d DependencyKind) String() string {
	switch d {
	case DependencyBlock:
		return "block"
	case DependencyEntity:
		return "entity"
	case DependencyRegistry:
		return "registry"
	default:
		return fmt.Sprintf("DependencyKind(%d)", uint8(d))
	}
}

// Dependency names one piece of data a tick read, or one it needed and could
// not get.
//
// The same type serves both because an incomplete tick's missing set is a
// subset of what it tried to read, and a caller that wants to load the gap and
// try again needs the two described the same way.
//
// The struct is a flat union rather than an interface so that it stays
// comparable and canonically encodable. Fields that a kind does not use are
// zero.
type Dependency struct {
	Kind  DependencyKind
	Block geom.BlockPos
	Entity entity.ID
	Name  string
}

// String names only the field the kind uses.
func (d Dependency) String() string {
	switch d.Kind {
	case DependencyBlock:
		return fmt.Sprintf("block(%d,%d,%d)", d.Block.X, d.Block.Y, d.Block.Z)
	case DependencyEntity:
		return fmt.Sprintf("entity(%d)", d.Entity)
	case DependencyRegistry:
		return fmt.Sprintf("registry(%s)", d.Name)
	default:
		return d.Kind.String()
	}
}

// Completeness reports whether a tick had everything it needed.
//
// A tick that did not is not an error: the caller is expected to load the named
// data and run the tick again. The result of an incomplete tick carries no
// applicable change set and no events, so applying it is impossible rather than
// merely discouraged.
type Completeness struct {
	// Complete is true when every rule in scope had the data it asked for.
	Complete bool
	// Missing names the data that was not available, sorted and deduplicated.
	Missing []Dependency
}

// sortDependencies returns a deduplicated copy in a total order.
//
// The order is by kind, then name, then block coordinate, then entity, and it
// exists so that a digest cannot depend on the order in which a rule happened
// to walk the world. The argument is not modified.
func sortDependencies(in []Dependency) []Dependency {
	if len(in) == 0 {
		return nil
	}

	out := slices.Clone(in)
	slices.SortFunc(out, func(a, b Dependency) int {
		return cmp.Or(
			cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.Name, b.Name),
			cmp.Compare(a.Block.X, b.Block.X),
			cmp.Compare(a.Block.Y, b.Block.Y),
			cmp.Compare(a.Block.Z, b.Block.Z),
			cmp.Compare(a.Entity, b.Entity),
		)
	})

	return slices.Compact(out)
}
```

- [x] **Step 4: Run the test to verify it passes**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -v`

Expected: PASS.

- [x] **Step 5: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [x] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add sim/completeness.go sim/completeness_test.go
git commit -m "feat(sim): add data dependencies and tick completeness"
```

---

## Task 5: Change sets and operations

**Files:**
- Create: `sim/change.go`
- Test: `sim/change_test.go`

**Interfaces:**
- Consumes: `geom.BlockPos`, `entity.ID`, `entity.State`, `world.BlockRef`, `Revision`.
- Produces:
  - `type OpKind uint8` with `OpSetEntity`, `OpRemoveEntity`, `OpSetBlock`, and `String()`
  - `type Op struct { Kind OpKind; Entity entity.ID; State entity.State; Block geom.BlockPos; Ref world.BlockRef }`
  - `type ChangeSet struct { BaseRevision Revision; Ops []Op }`
  - `func (c ChangeSet) IsEmpty() bool`

An `Op` carries a `world.BlockRef` and never a `geom.Shape`, because a shape keeps its boxes unexported and cannot be canonically encoded. The store resolves the handle through the profile, which is where the block data lives anyway.

Operations keep insertion order. A later operation may overwrite an earlier one, so sorting them would change what a change set means.

There is no partial apply. `ChangeSet` records the revision it was computed against, and a store applies all of it or none of it.

- [x] **Step 1: Write the failing test**

```go
package sim

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestOpKindString(t *testing.T) {
	for _, test := range []struct {
		kind OpKind
		want string
	}{
		{OpSetEntity, "set-entity"},
		{OpRemoveEntity, "remove-entity"},
		{OpSetBlock, "set-block"},
		{OpKind(7), "OpKind(7)"},
	} {
		if got := test.kind.String(); got != test.want {
			t.Errorf("OpKind(%d).String() = %q, want %q", test.kind, got, test.want)
		}
	}
}

func TestChangeSetIsEmpty(t *testing.T) {
	if !(ChangeSet{BaseRevision: 4}).IsEmpty() {
		t.Error("a change set with no operations reports itself non-empty")
	}
	if (ChangeSet{Ops: []Op{{Kind: OpRemoveEntity}}}).IsEmpty() {
		t.Error("a change set with an operation reports itself empty")
	}
}

func TestOpsKeepInsertionOrder(t *testing.T) {
	// Two writes to the same cell, where the second is the one that must win.
	// This is why operations are never sorted.
	pos := geom.BlockPos{X: 1}
	changes := ChangeSet{Ops: []Op{
		{Kind: OpSetBlock, Block: pos, Ref: 1},
		{Kind: OpSetBlock, Block: pos, Ref: 2},
	}}

	if changes.Ops[len(changes.Ops)-1].Ref != 2 {
		t.Fatalf("the last operation is not the last one appended: %+v", changes.Ops)
	}
}

func TestOpIsComparable(t *testing.T) {
	// A change set is compared field by field in tests and hashed in Task 8,
	// both of which need Op to stay free of slices and maps.
	first := Op{Kind: OpSetEntity, Entity: 3, State: entity.State{Family: entity.FamilyPlayer}}
	if first != first { //nolint:gocritic // the point is that == compiles at all
		t.Fatal("an operation does not equal itself")
	}
}
```

- [x] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -run 'TestOp|TestChangeSet'`

Expected: FAIL to build, with `undefined: OpKind`.

- [x] **Step 3: Write the implementation**

```go
package sim

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// OpKind says what an operation does.
type OpKind uint8

const (
	// OpSetEntity writes a body, creating it if the store has none.
	OpSetEntity OpKind = iota + 1
	// OpRemoveEntity drops a body.
	OpRemoveEntity
	// OpSetBlock writes a block state.
	OpSetBlock
)

// String returns the kind's name.
func (o OpKind) String() string {
	switch o {
	case OpSetEntity:
		return "set-entity"
	case OpRemoveEntity:
		return "remove-entity"
	case OpSetBlock:
		return "set-block"
	default:
		return fmt.Sprintf("OpKind(%d)", uint8(o))
	}
}

// Op is one state change.
//
// The block field carries a handle rather than a shape. A geom.Shape keeps its
// boxes unexported and cannot be canonically encoded, and the store can resolve
// a handle through the profile, which is where block data belongs.
//
// Fields a kind does not use are zero.
type Op struct {
	Kind   OpKind
	Entity entity.ID
	State  entity.State
	Block  geom.BlockPos
	Ref    world.BlockRef
}

// ChangeSet is every state change one tick produced.
//
// It is fully applicable or not applicable. There is no partial apply, and the
// operations are in the order the tick produced them: a later operation may
// overwrite an earlier one, so reordering them would change their meaning.
type ChangeSet struct {
	// BaseRevision is the store revision the tick was computed against. A store
	// that has moved on refuses the whole set.
	BaseRevision Revision
	// Ops are the changes, in tick order.
	Ops []Op
}

// IsEmpty reports whether the set changes nothing.
func (c ChangeSet) IsEmpty() bool {
	return len(c.Ops) == 0
}
```

- [x] **Step 4: Run the test to verify it passes**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -v`

Expected: PASS.

- [x] **Step 5: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [x] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add sim/change.go sim/change_test.go
git commit -m "feat(sim): add change sets and state operations"
```

---

## Task 6: Commands, events, limits, and random state

**Files:**
- Create: `sim/command.go`
- Create: `sim/event.go`
- Create: `sim/limits.go`
- Create: `sim/random.go`
- Test: `sim/contract_test.go`

**Interfaces:**
- Consumes: `entity.ID`, `geom.BlockPos`.
- Produces:
  - `type Command interface { CommandKind() string }`
  - `type CommandOutcome struct { Index int; Kind string; Accepted bool; Reason string }`
  - `type DomainEvent struct { Kind string; Entity entity.ID; Block geom.BlockPos }`
  - `type PresentationEvent struct { Kind string; Entity entity.ID; Block geom.BlockPos }`
  - `type Limits struct { EntitySteps, BlockUpdates, CollisionCandidates, Events int }`
  - `func (l Limits) withDefaults() Limits`
  - `var ErrLimitExhausted = errors.New(...)`
  - `type RandomStream struct { Name string; State uint64 }`
  - `type RandomState struct { Streams []RandomStream }`
  - `func (r RandomState) Clone() RandomState`
  - `func (r RandomState) Stream(name string) (uint64, bool)`
  - `func (r RandomState) WithStream(name string, state uint64) RandomState`

`Command` is an interface because this milestone cannot know what a movement intent contains, and a payload union invented now would be replaced by M8.4. Nothing hashes a command: the digest covers the result, and the result carries concrete outcomes.

`CommandOutcome` records the index of the command it answers, so a rejection is traceable to its input without the outcome having to copy it.

Events carry a namespaced `Kind` string, such as `movement.collided`. A `uint8` enum would be renamed once real rules exist.

`Limits` holds the budgets M8 can actually exhaust, drawn from the parent design's longer list; the rest arrive with the mechanics that need them. A zero field means the default rather than "no work allowed", because a caller who leaves the struct blank wants sensible bounds, not a tick that refuses to do anything.

`RandomState` is a list of named streams, each a single `uint64`, which holds Java's 48-bit generator seed with room to spare. Streams are named because the parent design requires separate sources to stay separate when the game version uses them.

- [x] **Step 1: Write the failing test**

```go
package sim

import (
	"testing"
)

// walkCommand stands in for the movement intents M8.4 will add. A command is an
// interface, so this test needs one implementation to prove the seam works.
type walkCommand struct {
	forward float64
}

func (walkCommand) CommandKind() string { return "movement.walk" }

func TestACommandNamesItsKind(t *testing.T) {
	var command Command = walkCommand{forward: 1}
	if got, want := command.CommandKind(), "movement.walk"; got != want {
		t.Fatalf("CommandKind = %q, want %q", got, want)
	}
}

func TestLimitsFillInDefaults(t *testing.T) {
	got := Limits{}.withDefaults()
	if got.EntitySteps <= 0 || got.BlockUpdates <= 0 ||
		got.CollisionCandidates <= 0 || got.Events <= 0 {
		t.Fatalf("withDefaults left a budget unusable: %+v", got)
	}

	// An explicit budget is never raised to the default.
	explicit := Limits{EntitySteps: 1, BlockUpdates: 2, CollisionCandidates: 3, Events: 4}
	if got := explicit.withDefaults(); got != explicit {
		t.Fatalf("withDefaults changed an explicit budget: %+v", got)
	}
}

func TestRandomStateStreams(t *testing.T) {
	state := RandomState{}.WithStream("world", 42).WithStream("entity", 7)

	if got, ok := state.Stream("world"); !ok || got != 42 {
		t.Fatalf("Stream(world) = (%d, %v), want (42, true)", got, ok)
	}
	if _, ok := state.Stream("absent"); ok {
		t.Error("Stream found a name that was never set")
	}

	// Replacing a stream must not add a second entry with the same name.
	state = state.WithStream("world", 43)
	if got, _ := state.Stream("world"); got != 43 {
		t.Errorf("Stream(world) = %d after replacement, want 43", got)
	}
	if len(state.Streams) != 2 {
		t.Fatalf("state holds %d streams, want 2: %+v", len(state.Streams), state.Streams)
	}
}

func TestRandomStateStreamsAreSortedByName(t *testing.T) {
	// The order is part of the contract: the digest encodes these in order, so
	// two states with the same streams must encode identically however they
	// were built.
	first := RandomState{}.WithStream("b", 1).WithStream("a", 2)
	second := RandomState{}.WithStream("a", 2).WithStream("b", 1)

	if len(first.Streams) != len(second.Streams) {
		t.Fatalf("lengths differ: %d vs %d", len(first.Streams), len(second.Streams))
	}
	for index := range first.Streams {
		if first.Streams[index] != second.Streams[index] {
			t.Fatalf("stream %d differs: %+v vs %+v",
				index, first.Streams[index], second.Streams[index])
		}
	}
}

func TestRandomStateCloneDoesNotAlias(t *testing.T) {
	state := RandomState{}.WithStream("world", 1)
	clone := state.Clone()
	state.Streams[0].State = 99

	if got, _ := clone.Stream("world"); got != 1 {
		t.Fatalf("the clone followed its original: %d", got)
	}
}
```

- [x] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -run 'TestACommand|TestLimits|TestRandom'`

Expected: FAIL to build, with `undefined: Command`.

- [x] **Step 3: Write `sim/command.go`**

```go
package sim

// Command expresses simulation intent in semantic units: a movement intent, an
// interaction, an impulse, a scheduled block tick. A command carries no packet
// identifier and no encoded wire value.
//
// This is an interface rather than a struct because the intents belong to the
// rules that consume them, and no rule exists yet. Nothing hashes a command:
// the digest covers the result, which records concrete outcomes instead.
//
// The adapter that creates a command remains responsible for authentication and
// network-level authorization. A profile decides only whether the actor and the
// current state permit it.
type Command interface {
	// CommandKind returns a namespaced kind, such as "movement.walk".
	CommandKind() string
}

// CommandOutcome records what a tick did with one command.
//
// Index is the command's position in the tick's input, so a rejection is
// traceable back to what caused it without the outcome copying the command.
type CommandOutcome struct {
	Index    int
	Kind     string
	Accepted bool
	// Reason explains a rejection. It is empty when Accepted is true.
	Reason string
}
```

- [x] **Step 4: Write `sim/event.go`**

```go
package sim

import (
	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// DomainEvent reports a simulation fact: a collision, a landing, a spawn, a
// removal. A server can turn these into packets and a client into observations.
//
// Kind is a namespaced string, such as "movement.collided". A numeric enum
// invented before any rule emits an event would be renamed by the first
// milestone that does; a string costs nothing at this scale and survives being
// added to.
type DomainEvent struct {
	Kind   string
	Entity entity.ID
	Block  geom.BlockPos
}

// PresentationEvent requests a particle, a sound, or an animation. It carries
// no simulation meaning: a consumer may ignore every one of them and still hold
// correct state.
type PresentationEvent struct {
	Kind   string
	Entity entity.ID
	Block  geom.BlockPos
}
```

- [x] **Step 5: Write `sim/limits.go`**

```go
package sim

import "errors"

// ErrLimitExhausted reports that a tick asked for more work than its
// deterministic budget allows. The budget exists so that a malformed input
// cannot make one tick run without bound, and so that the same input costs the
// same work on every machine.
var ErrLimitExhausted = errors.New("sim: deterministic work limit exhausted")

// Limits bounds the work one tick may do.
//
// These are the budgets this milestone can actually exhaust. The parent design
// lists more, covering scheduled events, explosions, fluids, and extensions;
// each arrives with the mechanic that can spend it, because a budget nothing
// counts against is a field that drifts out of date.
//
// A zero field means the default. A caller who leaves the struct blank wants
// sensible bounds, not a tick that refuses to do anything.
type Limits struct {
	// EntitySteps bounds how many bodies one tick may move.
	EntitySteps int
	// BlockUpdates bounds how many block writes one tick may produce.
	BlockUpdates int
	// CollisionCandidates bounds the cells one sweep may visit. It is passed
	// straight to collision.Move.
	CollisionCandidates int
	// Events bounds how many domain and presentation events one tick may emit,
	// counted together.
	Events int
}

// defaultLimits are chosen to be far above any legitimate tick and far below
// anything that could hang a server. They are not tuned; they are a ceiling.
var defaultLimits = Limits{
	EntitySteps:         4096,
	BlockUpdates:        4096,
	CollisionCandidates: 32768,
	Events:              4096,
}

// withDefaults replaces every non-positive budget with its default.
func (l Limits) withDefaults() Limits {
	if l.EntitySteps <= 0 {
		l.EntitySteps = defaultLimits.EntitySteps
	}
	if l.BlockUpdates <= 0 {
		l.BlockUpdates = defaultLimits.BlockUpdates
	}
	if l.CollisionCandidates <= 0 {
		l.CollisionCandidates = defaultLimits.CollisionCandidates
	}
	if l.Events <= 0 {
		l.Events = defaultLimits.Events
	}

	return l
}
```

- [x] **Step 6: Write `sim/random.go`**

```go
package sim

import (
	"cmp"
	"slices"
)

// RandomStream is one named generator's serialized state.
//
// A single uint64 holds Java's 48-bit generator seed with room to spare, and
// keeping the state a plain integer is what lets a result be encoded, hashed,
// stored, and replayed without a custom serializer.
type RandomStream struct {
	Name  string
	State uint64
}

// RandomState is every random stream a tick may draw from.
//
// Streams are named because the parent design requires separate sources to stay
// separate when the game version uses them; folding them into one generator
// would change which numbers a rule sees.
//
// Streams are kept sorted by name so that two states holding the same streams
// encode identically however they were assembled.
type RandomState struct {
	Streams []RandomStream
}

// Clone returns a state that does not alias this one.
func (r RandomState) Clone() RandomState {
	return RandomState{Streams: slices.Clone(r.Streams)}
}

// Stream returns a named stream's state.
func (r RandomState) Stream(name string) (uint64, bool) {
	for _, stream := range r.Streams {
		if stream.Name == name {
			return stream.State, true
		}
	}

	return 0, false
}

// WithStream returns a state with the named stream set, replacing any stream
// that already has the name.
func (r RandomState) WithStream(name string, state uint64) RandomState {
	streams := slices.Clone(r.Streams)
	for index := range streams {
		if streams[index].Name == name {
			streams[index].State = state

			return RandomState{Streams: streams}
		}
	}

	streams = append(streams, RandomStream{Name: name, State: state})
	slices.SortFunc(streams, func(a, b RandomStream) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return RandomState{Streams: streams}
}
```

- [x] **Step 7: Run the tests to verify they pass**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -v`

Expected: PASS.

- [x] **Step 8: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [x] **Step 9: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add sim/command.go sim/event.go sim/limits.go sim/random.go sim/contract_test.go
git commit -m "feat(sim): add commands, events, work limits, and random state"
```

---

## Task 7: The canonical encoder

**Files:**
- Create: `sim/canon.go`
- Test: `sim/canon_test.go`

**Interfaces:**
- Consumes: `geom`, `entity`, `world` value types.
- Produces an unexported `encoder` with methods for every value a result holds:
  `tag`, `bool`, `uint8`, `uint32`, `int32`, `uint64`, `float64`, `string`, `count`, `vec`, `box`, `blockPos`, and `bytes`.

Everything is fixed-width big-endian, and every composite value writes a one-byte tag before its fields. The tags are domain separation: without them, two different structures could encode to the same bytes, and a digest that cannot tell them apart is worse than no digest.

Two float rules, stated once and relied on everywhere:

- `-0.0` encodes as `0.0`. A step-up settle can produce a negative zero, and two results that differ only in a zero's sign describe the same state: adding either to a coordinate gives the same coordinate, and `value < 0` is false for both.
- Every NaN encodes as one canonical quiet NaN. NaN payloads are not portable across platforms, which would defeat M8.6's whole purpose. A NaN in simulation state is a bug, and Task 9 asserts separately that none reaches a result; folding the bits here is about the digest being portable, not about tolerating the bug.

- [x] **Step 1: Write the failing test**

```go
package sim

import (
	"bytes"
	"math"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestEncoderIsBigEndianAndFixedWidth(t *testing.T) {
	var e encoder
	e.uint32(1)
	if got, want := e.buf, []byte{0, 0, 0, 1}; !bytes.Equal(got, want) {
		t.Fatalf("uint32(1) = %v, want %v", got, want)
	}
}

func TestEncoderWritesNegativeIntegersAsTwosComplement(t *testing.T) {
	var e encoder
	e.int32(-1)
	if got, want := e.buf, []byte{0xFF, 0xFF, 0xFF, 0xFF}; !bytes.Equal(got, want) {
		t.Fatalf("int32(-1) = %v, want %v", got, want)
	}
}

func TestEncoderFoldsNegativeZero(t *testing.T) {
	var positive, negative encoder
	positive.float64(0)
	negative.float64(math.Copysign(0, -1))

	if !bytes.Equal(positive.buf, negative.buf) {
		t.Fatalf("zero encodes differently by sign: %v vs %v", positive.buf, negative.buf)
	}
}

func TestEncoderFoldsEveryNaNToOnePattern(t *testing.T) {
	first := math.Float64frombits(0x7FF8000000000001)
	second := math.Float64frombits(0xFFF8000000000002)

	var a, b encoder
	a.float64(first)
	b.float64(second)

	if !bytes.Equal(a.buf, b.buf) {
		t.Fatalf("two NaNs encode differently: %v vs %v", a.buf, b.buf)
	}
}

func TestEncoderDistinguishesValuesThatSharePrefixes(t *testing.T) {
	// Without a length prefix, "ab" then "c" and "a" then "bc" would encode
	// alike, and a digest could not tell two different results apart.
	var first, second encoder
	first.string("ab")
	first.string("c")
	second.string("a")
	second.string("bc")

	if bytes.Equal(first.buf, second.buf) {
		t.Fatal("two different string sequences encode identically")
	}
}

func TestEncoderTagsSeparateDomains(t *testing.T) {
	var tagged, plain encoder
	tagged.tag(tagVec)
	tagged.float64(1)
	plain.float64(1)

	if bytes.Equal(tagged.buf, plain.buf) {
		t.Fatal("a tagged value encodes the same as an untagged one")
	}
}

func TestEncoderWritesCompositesReproducibly(t *testing.T) {
	build := func() []byte {
		var e encoder
		e.vec(geom.Vec3{X: 1, Y: -2, Z: 0.5})
		e.box(geom.AABB{MinX: -1, MaxY: 3})
		e.blockPos(geom.BlockPos{X: -5, Y: 60, Z: 7})
		e.count(2)

		return e.buf
	}

	if !bytes.Equal(build(), build()) {
		t.Fatal("encoding the same values twice produced different bytes")
	}
	if len(build()) == 0 {
		t.Fatal("encoding produced no bytes")
	}
}
```

- [x] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -run TestEncoder`

Expected: FAIL to build, with `undefined: encoder`.

- [x] **Step 3: Write the implementation**

```go
package sim

import (
	"encoding/binary"
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// The canonical encoding is fixed-width and big-endian, and every composite
// value is preceded by a one-byte tag.
//
// The tags are domain separation. Without them two different structures could
// produce the same bytes, and a digest that cannot tell them apart is worse
// than no digest, because it would report agreement between results that
// disagree.
//
// Tags are append-only. Changing an existing tag's value changes every digest
// ever recorded, so a new kind of value takes a new tag.
const (
	tagResult uint8 = iota + 1
	tagChangeSet
	tagOp
	tagEntityState
	tagDomainEvent
	tagPresentationEvent
	tagCommandOutcome
	tagRandomState
	tagRandomStream
	tagDependency
	tagCompleteness
	tagProfileID
	tagVec
	tagBox
	tagBlockPos
)

// canonicalNaN is the single pattern every NaN encodes as. NaN payloads are not
// portable across platforms, and M8.6 gates on digests matching across six of
// them.
const canonicalNaN uint64 = 0x7FF8000000000000

// encoder builds a canonical byte string. It never fails: every value this
// package holds has an encoding, and a value that did not would be a compile
// error rather than a runtime one.
type encoder struct {
	buf []byte
}

// tag writes a domain separator.
func (e *encoder) tag(value uint8) {
	e.buf = append(e.buf, value)
}

// bool writes one byte.
func (e *encoder) bool(value bool) {
	if value {
		e.buf = append(e.buf, 1)

		return
	}
	e.buf = append(e.buf, 0)
}

// uint8 writes one byte.
func (e *encoder) uint8(value uint8) {
	e.buf = append(e.buf, value)
}

// uint32 writes four bytes, big-endian.
func (e *encoder) uint32(value uint32) {
	e.buf = binary.BigEndian.AppendUint32(e.buf, value)
}

// int32 writes four bytes of two's complement, big-endian.
func (e *encoder) int32(value int32) {
	e.uint32(uint32(value))
}

// uint64 writes eight bytes, big-endian.
func (e *encoder) uint64(value uint64) {
	e.buf = binary.BigEndian.AppendUint64(e.buf, value)
}

// float64 writes the IEEE 754 bits, big-endian, after folding negative zero to
// zero and every NaN to one pattern. See canonicalNaN and the package
// documentation for why.
func (e *encoder) float64(value float64) {
	switch {
	case math.IsNaN(value):
		e.uint64(canonicalNaN)
	case value == 0:
		// Catches both zeros: -0.0 == 0.0 is true.
		e.uint64(0)
	default:
		e.uint64(math.Float64bits(value))
	}
}

// count writes a length as four bytes. Lengths are encoded so that two
// sequences cannot run together into the same bytes.
func (e *encoder) count(value int) {
	e.uint32(uint32(value))
}

// string writes a length-prefixed UTF-8 string.
func (e *encoder) string(value string) {
	e.count(len(value))
	e.buf = append(e.buf, value...)
}

// bytes writes a length-prefixed byte string.
func (e *encoder) bytes(value []byte) {
	e.count(len(value))
	e.buf = append(e.buf, value...)
}

// vec writes a vector.
func (e *encoder) vec(value geom.Vec3) {
	e.tag(tagVec)
	e.float64(value.X)
	e.float64(value.Y)
	e.float64(value.Z)
}

// box writes an axis-aligned box.
func (e *encoder) box(value geom.AABB) {
	e.tag(tagBox)
	e.float64(value.MinX)
	e.float64(value.MinY)
	e.float64(value.MinZ)
	e.float64(value.MaxX)
	e.float64(value.MaxY)
	e.float64(value.MaxZ)
}

// blockPos writes a block position.
func (e *encoder) blockPos(value geom.BlockPos) {
	e.tag(tagBlockPos)
	e.int32(value.X)
	e.int32(value.Y)
	e.int32(value.Z)
}
```

- [x] **Step 4: Run the test to verify it passes**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -v -run TestEncoder`

Expected: PASS, seven tests. `bytes` and some tags are unused until Task 8; if the linter objects, add the remaining encoders in Task 8 instead of leaving dead code here.

- [x] **Step 5: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [x] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add sim/canon.go sim/canon_test.go
git commit -m "feat(sim): add the canonical value encoder"
```

---

## Task 8: Tick input, tick result, and the digest

**Files:**
- Create: `sim/result.go`
- Create: `sim/digest.go`
- Test: `sim/digest_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1 to 7, plus `world.View` and `entity.View`.
- Produces:
  - `type Scope struct { Entities []entity.ID }`
  - `type TickInput struct { Profile Profile; Revision Revision; Tick Tick; Blocks world.View; Entities entity.View; Scope Scope; Commands []Command; Random RandomState; Limits Limits }`
  - `type TickResult struct { Revision Revision; Tick Tick; Changes ChangeSet; Domain []DomainEvent; Presentation []PresentationEvent; Outcomes []CommandOutcome; Random RandomState; Read []Dependency; Completeness Completeness; Digest Digest }`
  - `type Digest [32]byte`
  - `func (d Digest) String() string`
  - `func (d Digest) IsZero() bool`
  - `func (r TickResult) computeDigest(id ProfileID) Digest`

The digest covers the profile identity and every field of the result except the digest itself. Including the identity is what makes a digest from one profile unable to collide with a digest from another; excluding the digest field is obvious but worth stating, since the field is zero while it is being computed.

`Read` is included, sorted. A rule that starts consulting different cells has changed behaviour even when its output happens to match, and a digest that hid that would let a refactor look identical when it is not.

`Scope` names the entities a tick simulates, in order. It is a struct rather than a bare slice so that later milestones can add dimensions and regions without changing every signature.

- [x] **Step 1: Write the failing test**

```go
package sim

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func sampleResult() TickResult {
	return TickResult{
		Revision: 3,
		Tick:     11,
		Changes: ChangeSet{BaseRevision: 3, Ops: []Op{
			{Kind: OpSetEntity, Entity: 1, State: entity.State{
				Family: entity.FamilyPlayer,
				Box:    geom.AABB{MaxX: 0.6, MaxY: 1.8, MaxZ: 0.6},
				Motion: geom.Vec3{Y: -0.0784000015258789},
			}},
			{Kind: OpSetBlock, Block: geom.BlockPos{X: 1, Y: 2, Z: 3}, Ref: 9},
		}},
		Domain:       []DomainEvent{{Kind: "movement.collided", Entity: 1}},
		Presentation: []PresentationEvent{{Kind: "movement.step", Entity: 1}},
		Outcomes:     []CommandOutcome{{Index: 0, Kind: "movement.walk", Accepted: true}},
		Random:       RandomState{}.WithStream("world", 7),
		Read:         []Dependency{{Kind: DependencyBlock, Block: geom.BlockPos{X: 1}}},
		Completeness: Completeness{Complete: true},
	}
}

var sampleProfileID = ProfileID{Edition: "java", GameVersion: "1.8.9", RulesRevision: "1"}

func TestDigestIsStableAcrossRuns(t *testing.T) {
	first := sampleResult().computeDigest(sampleProfileID)
	second := sampleResult().computeDigest(sampleProfileID)

	if first != second {
		t.Fatalf("the same result digested differently: %s vs %s", first, second)
	}
	if first.IsZero() {
		t.Fatal("a non-empty result digested to zero")
	}
}

func TestDigestIgnoresTheDigestField(t *testing.T) {
	result := sampleResult()
	want := result.computeDigest(sampleProfileID)

	result.Digest = want
	if got := result.computeDigest(sampleProfileID); got != want {
		t.Fatalf("digest changed once the field was filled: %s vs %s", got, want)
	}
}

func TestDigestSeparatesProfiles(t *testing.T) {
	other := ProfileID{Edition: "java", GameVersion: "26.1.2", RulesRevision: "1"}
	if sampleResult().computeDigest(sampleProfileID) == sampleResult().computeDigest(other) {
		t.Fatal("two profiles produced the same digest for the same result")
	}
}

func TestDigestNoticesEveryField(t *testing.T) {
	base := sampleResult().computeDigest(sampleProfileID)

	for name, mutate := range map[string]func(*TickResult){
		"revision":     func(r *TickResult) { r.Revision++ },
		"tick":         func(r *TickResult) { r.Tick++ },
		"base revision": func(r *TickResult) { r.Changes.BaseRevision++ },
		"an operation": func(r *TickResult) { r.Changes.Ops[1].Ref++ },
		"operation order": func(r *TickResult) {
			r.Changes.Ops[0], r.Changes.Ops[1] = r.Changes.Ops[1], r.Changes.Ops[0]
		},
		"a motion bit": func(r *TickResult) {
			r.Changes.Ops[0].State.Motion.Y = -0.078400001525878
		},
		"a domain event":       func(r *TickResult) { r.Domain[0].Kind = "movement.landed" },
		"a presentation event": func(r *TickResult) { r.Presentation[0].Kind = "movement.splash" },
		"an outcome":           func(r *TickResult) { r.Outcomes[0].Accepted = false },
		"random state":         func(r *TickResult) { r.Random = r.Random.WithStream("world", 8) },
		"a read dependency":    func(r *TickResult) { r.Read[0].Block.X = 2 },
		"completeness":         func(r *TickResult) { r.Completeness = Completeness{} },
	} {
		t.Run(name, func(t *testing.T) {
			result := sampleResult()
			mutate(&result)
			if got := result.computeDigest(sampleProfileID); got == base {
				t.Fatalf("changing %s did not change the digest", name)
			}
		})
	}
}

func TestDigestStringIsHex(t *testing.T) {
	got := sampleResult().computeDigest(sampleProfileID).String()
	if len(got) != 64 {
		t.Fatalf("String returned %d characters, want 64: %q", len(got), got)
	}
}
```

`TestDigestNoticesEveryField` is the important one. A digest that misses a field is worse than useless, because every later milestone trusts it to prove two runs agree.

- [x] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -run TestDigest`

Expected: FAIL to build, with `undefined: TickResult`.

- [x] **Step 3: Write `sim/result.go`**

Define `Scope`, `TickInput`, and `TickResult` with the fields listed above. Document on `TickInput` that the views must stay valid and unchanged for the whole `Step` call, and that the kernel reads nothing else: no clock, no global random state, no mutable application object. Document on `TickResult` that an incomplete result carries no operations and no events, so that applying one is impossible rather than discouraged.

- [x] **Step 4: Write `sim/digest.go`**

```go
package sim

import (
	"crypto/sha256"
	"encoding/hex"
)

// Digest is a canonical hash of one tick result.
//
// Two results with the same digest describe the same tick: the same changes in
// the same order, the same events, the same outcomes, the same random state,
// the same data read, and the same completeness, under the same profile. That
// is the property M8.6 gates on across operating systems and architectures, and
// every fixture that compares runs depends on it.
type Digest [32]byte

// String returns the digest in lowercase hexadecimal.
func (d Digest) String() string {
	return hex.EncodeToString(d[:])
}

// IsZero reports whether the digest was never computed.
func (d Digest) IsZero() bool {
	return d == Digest{}
}

// computeDigest hashes the canonical encoding of the result under a profile
// identity.
//
// The identity is included so that a digest from one profile cannot collide
// with a digest from another. The result's own Digest field is excluded, since
// it is zero while this runs and filled from the return value.
func (r TickResult) computeDigest(id ProfileID) Digest {
	var e encoder
	e.tag(tagResult)

	e.tag(tagProfileID)
	e.string(id.Edition)
	e.string(id.GameVersion)
	e.string(id.RulesRevision)

	e.uint64(uint64(r.Revision))
	e.uint64(uint64(r.Tick))

	e.tag(tagChangeSet)
	e.uint64(uint64(r.Changes.BaseRevision))
	e.count(len(r.Changes.Ops))
	for _, op := range r.Changes.Ops {
		e.tag(tagOp)
		e.uint8(uint8(op.Kind))
		e.int32(int32(op.Entity))
		e.tag(tagEntityState)
		e.uint8(uint8(op.State.Family))
		e.box(op.State.Box)
		e.vec(op.State.Motion)
		e.bool(op.State.OnGround)
		e.float64(op.State.StepHeight)
		e.blockPos(op.Block)
		e.uint32(uint32(op.Ref))
	}

	e.count(len(r.Domain))
	for _, event := range r.Domain {
		e.tag(tagDomainEvent)
		e.string(event.Kind)
		e.int32(int32(event.Entity))
		e.blockPos(event.Block)
	}

	e.count(len(r.Presentation))
	for _, event := range r.Presentation {
		e.tag(tagPresentationEvent)
		e.string(event.Kind)
		e.int32(int32(event.Entity))
		e.blockPos(event.Block)
	}

	e.count(len(r.Outcomes))
	for _, outcome := range r.Outcomes {
		e.tag(tagCommandOutcome)
		e.count(outcome.Index)
		e.string(outcome.Kind)
		e.bool(outcome.Accepted)
		e.string(outcome.Reason)
	}

	e.tag(tagRandomState)
	e.count(len(r.Random.Streams))
	for _, stream := range r.Random.Streams {
		e.tag(tagRandomStream)
		e.string(stream.Name)
		e.uint64(stream.State)
	}

	e.count(len(r.Read))
	for _, dependency := range r.Read {
		encodeDependency(&e, dependency)
	}

	e.tag(tagCompleteness)
	e.bool(r.Completeness.Complete)
	e.count(len(r.Completeness.Missing))
	for _, dependency := range r.Completeness.Missing {
		encodeDependency(&e, dependency)
	}

	return sha256.Sum256(e.buf)
}

// encodeDependency writes one dependency. Both the read set and the missing set
// use it, so the two can never drift apart.
func encodeDependency(e *encoder, dependency Dependency) {
	e.tag(tagDependency)
	e.uint8(uint8(dependency.Kind))
	e.blockPos(dependency.Block)
	e.int32(int32(dependency.Entity))
	e.string(dependency.Name)
}
```

- [x] **Step 5: Run the tests to verify they pass**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -v -run TestDigest`

Expected: PASS, including all twelve subtests of `TestDigestNoticesEveryField`. A subtest that fails names the field the encoding forgot.

- [x] **Step 6: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [x] **Step 7: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add sim/result.go sim/digest.go sim/digest_test.go
git commit -m "feat(sim): add tick input, tick result, and the canonical digest"
```

---

## Task 9: The profile contract, phases, and the kernel

**Files:**
- Create: `sim/profile.go`
- Create: `sim/kernel.go`
- Create: `sim/fake.go`
- Test: `sim/kernel_test.go`

**Interfaces:**
- Consumes: everything above.
- Produces:
  - `type MotionConstants struct { Gravity, HorizontalDrag, VerticalDrag, StepHeight float64 }`
  - `type Profile interface { ID() ProfileID; Slipperiness(world.BlockRef) float64; Motion(entity.Family) MotionConstants; Shape(world.BlockRef) (geom.Shape, bool); Phases() []Phase }`
  - `type Phase interface { ID() string; Run(ctx context.Context, tick *TickState) error }`
  - `type TickState struct { ... }` with the accessors and recorders listed below
  - `func NewKernel(profile Profile) (Kernel, error)`
  - `var ErrIncomplete`, `var ErrNaNInResult`
  - `type StaticProfile struct { ... }` in `sim/fake.go`

`MotionConstants` mirrors `data.EntityMotion` field for field. That is not laziness: M8.2 established that a quantity's float width is part of its value, and every reshaping between the dataset and the rule is a chance to round `0.6000000238418579` back to `0.6`. `physics.json` already stores the widened forms, so the profile's job is to copy, not to convert.

`Profile` gains `Shape` beyond the three frozen methods, for the reason given under the design decisions: a change set carries handles, and something must resolve them. It is the same seam.

`TickState` is the only thing a phase touches. It exposes reads that record dependencies, writes that append operations, and emitters that respect the event budget:

- `Profile() Profile`, `Tick() Tick`, `Limits() Limits`, `Scope() Scope`
- `Entity(entity.ID) (entity.State, bool)` — records an entity dependency
- `BlockShape(geom.BlockPos) (geom.Shape, world.Lookup)` and `BlockState(geom.BlockPos) (world.BlockRef, world.Lookup)` — record a block dependency, and mark the tick incomplete on `LookupUnknown`
- `Blocks() world.View` — for handing straight to `collision.Resolve`, which does its own unknown reporting; a phase that uses this must feed the returned `Unknown` cells to `MissingBlocks`
- `SetEntity`, `RemoveEntity`, `SetBlock` — append operations against their budgets
- `EmitDomain`, `EmitPresentation`, `RecordOutcome`
- `Random() RandomState` and `SetRandom(RandomState)`
- `MissingBlocks([]geom.BlockPos)` — declare incompleteness explicitly

`Step` is the whole kernel:

1. Validate the input: a profile, a non-zero identity, views present.
2. Fill in limit defaults, clone the random state, build a `TickState`.
3. Run each phase in order, checking `ctx.Err()` before each.
4. If any phase reported missing data, discard every operation and event and return an incomplete result naming the missing set.
5. Otherwise assemble the result, sort the read set, assert no NaN reached it, compute the digest, and return.

Phases run on one goroutine. Cancellation and a limit failure both abort and return no applicable change set, per the parent design's error model.

- [x] **Step 1: Write the failing test**

```go
package sim

import (
	"context"
	"errors"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// phaseFunc adapts a function to Phase, so tests can state a phase inline.
type phaseFunc struct {
	id  string
	run func(context.Context, *TickState) error
}

func (p phaseFunc) ID() string { return p.id }

func (p phaseFunc) Run(ctx context.Context, tick *TickState) error {
	return p.run(ctx, tick)
}

// emptyWorld describes a small region as air, so a lookup inside it is known.
func emptyWorld() *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -4, Y: -4, Z: -4}, geom.BlockPos{X: 4, Y: 4, Z: 4}, geom.EmptyShape())

	return blocks
}

func testInput(profile Profile) TickInput {
	return TickInput{
		Profile:  profile,
		Revision: 5,
		Tick:     9,
		Blocks:   emptyWorld(),
		Entities: entity.NewBodies(),
	}
}

func TestNewKernelRejectsABadProfile(t *testing.T) {
	if _, err := NewKernel(nil); err == nil {
		t.Error("NewKernel accepted a nil profile")
	}
	if _, err := NewKernel(&StaticProfile{}); err == nil {
		t.Error("NewKernel accepted a profile with no identity")
	}

	duplicate := &StaticProfile{
		Identity: sampleProfileID,
		PhaseList: []Phase{
			phaseFunc{id: "a", run: func(context.Context, *TickState) error { return nil }},
			phaseFunc{id: "a", run: func(context.Context, *TickState) error { return nil }},
		},
	}
	if _, err := NewKernel(duplicate); err == nil {
		t.Error("NewKernel accepted two phases with the same identifier")
	}
}

// TestEmptyTickIsStable is the milestone's first exit criterion: a tick that
// does no work produces the same digest every time it runs.
func TestEmptyTickIsStable(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	first, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	second, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if first.Digest != second.Digest {
		t.Fatalf("an empty tick digested differently: %s vs %s", first.Digest, second.Digest)
	}
	if first.Digest.IsZero() {
		t.Fatal("an empty tick produced no digest")
	}
	if !first.Completeness.Complete {
		t.Fatalf("an empty tick was incomplete: %+v", first.Completeness)
	}
	if !first.Changes.IsEmpty() {
		t.Fatalf("an empty tick produced operations: %+v", first.Changes.Ops)
	}
	if first.Revision != 5 || first.Tick != 9 {
		t.Fatalf("the result did not carry its input revision and tick: %+v", first)
	}
	if first.Changes.BaseRevision != 5 {
		t.Fatalf("BaseRevision = %d, want the input revision", first.Changes.BaseRevision)
	}
}

// TestEmptyTickDigestIsPinned makes an accidental encoding change visible. When
// the encoding changes on purpose, run the test, read the new value from the
// failure, and update this constant in the same commit as the change.
func TestEmptyTickDigestIsPinned(t *testing.T) {
	const want = "REPLACE_WITH_THE_VALUE_THE_FIRST_RUN_PRINTS"

	profile := &StaticProfile{Identity: sampleProfileID}
	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	got, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got.Digest.String() != want {
		t.Fatalf("the empty tick digest is %s, pinned at %s", got.Digest, want)
	}
}

func TestAPhaseWritesThroughTheTickState(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "spawn", run: func(_ context.Context, tick *TickState) error {
		tick.SetEntity(1, entity.State{Family: entity.FamilyPlayer})
		tick.SetBlock(geom.BlockPos{X: 1}, 4)
		tick.EmitDomain(DomainEvent{Kind: "test.spawned", Entity: 1})
		tick.EmitPresentation(PresentationEvent{Kind: "test.puff"})
		tick.RecordOutcome(CommandOutcome{Index: 0, Kind: "test.none", Accepted: true})

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	got, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if len(got.Changes.Ops) != 2 {
		t.Fatalf("the tick produced %d operations, want 2: %+v", len(got.Changes.Ops), got.Changes.Ops)
	}
	if got.Changes.Ops[0].Kind != OpSetEntity || got.Changes.Ops[1].Kind != OpSetBlock {
		t.Fatalf("operations are out of phase order: %+v", got.Changes.Ops)
	}
	if len(got.Domain) != 1 || len(got.Presentation) != 1 || len(got.Outcomes) != 1 {
		t.Fatalf("the tick lost an event or an outcome: %+v", got)
	}
}

func TestAnUnknownLookupMakesTheTickIncompleteAndDropsItsWork(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "read", run: func(_ context.Context, tick *TickState) error {
		// Write first, then read a cell nobody described. The write must not
		// survive: an incomplete result carries no applicable changes.
		tick.SetEntity(1, entity.State{Family: entity.FamilyPlayer})
		if _, lookup := tick.BlockShape(geom.BlockPos{X: 900}); lookup != world.LookupUnknown {
			t.Errorf("lookup = %v, want LookupUnknown", lookup)
		}

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	got, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got.Completeness.Complete {
		t.Fatal("a tick that read an unknown cell reported itself complete")
	}
	if len(got.Completeness.Missing) != 1 {
		t.Fatalf("Missing = %+v, want the one unknown cell", got.Completeness.Missing)
	}
	if !got.Changes.IsEmpty() || len(got.Domain) != 0 {
		t.Fatalf("an incomplete tick kept its work: %+v", got)
	}
	if got.Digest.IsZero() {
		t.Fatal("an incomplete tick produced no digest; it is still a result")
	}
}

func TestReadDependenciesAreSortedAndDeduplicated(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "read", run: func(_ context.Context, tick *TickState) error {
		for _, pos := range []geom.BlockPos{{X: 2}, {X: 1}, {X: 2}, {X: 0}} {
			tick.BlockShape(pos)
		}

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	got, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if len(got.Read) != 3 {
		t.Fatalf("Read holds %d dependencies, want 3: %+v", len(got.Read), got.Read)
	}
	for index := 1; index < len(got.Read); index++ {
		if got.Read[index-1].Block.X >= got.Read[index].Block.X {
			t.Fatalf("Read is not sorted: %+v", got.Read)
		}
	}
}

func TestAPhaseErrorAbortsTheTick(t *testing.T) {
	failure := errors.New("rule failed")
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "boom", run: func(context.Context, *TickState) error {
		return failure
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	got, err := kernel.Step(context.Background(), testInput(profile))
	if !errors.Is(err, failure) {
		t.Fatalf("Step error = %v, want the phase's error", err)
	}
	if !got.Changes.IsEmpty() {
		t.Fatalf("a failed tick returned an applicable change set: %+v", got.Changes)
	}
}

func TestCancellationAbortsTheTick(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "never", run: func(context.Context, *TickState) error {
		t.Error("a phase ran under a cancelled context")

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := kernel.Step(ctx, testInput(profile))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Step error = %v, want context.Canceled", err)
	}
	if !got.Changes.IsEmpty() {
		t.Fatalf("a cancelled tick returned an applicable change set: %+v", got.Changes)
	}
}

func TestTheEventBudgetIsEnforced(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "flood", run: func(_ context.Context, tick *TickState) error {
		for range 10 {
			tick.EmitDomain(DomainEvent{Kind: "test.noise"})
		}

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	input := testInput(profile)
	input.Limits = Limits{Events: 3}

	if _, err := kernel.Step(context.Background(), input); !errors.Is(err, ErrLimitExhausted) {
		t.Fatalf("Step error = %v, want ErrLimitExhausted", err)
	}
}

func TestANaNInAResultIsRefused(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "nan", run: func(_ context.Context, tick *TickState) error {
		tick.SetEntity(1, entity.State{
			Family: entity.FamilyPlayer,
			Motion: geom.Vec3{Y: math.NaN()},
		})

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	if _, err := kernel.Step(context.Background(), testInput(profile)); !errors.Is(err, ErrNaNInResult) {
		t.Fatalf("Step error = %v, want ErrNaNInResult", err)
	}
}
```

Add `"math"` to the test imports for the last case.

- [x] **Step 2: Run the tests to verify they fail**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -run 'TestNewKernel|TestEmptyTick|TestAPhase|TestAnUnknown|TestRead|TestCancellation|TestTheEvent|TestANaN'`

Expected: FAIL to build, with `undefined: NewKernel`.

- [x] **Step 3: Write `sim/profile.go`**

Declare `MotionConstants`, `Profile`, `Phase`, and `TickState` with the members listed above. `TickState` holds the input views, the accumulated operations, events, outcomes, dependencies, missing set, random state, effective limits, and running counters for each budget. Every recorder checks its budget and returns `ErrLimitExhausted` through a stored error that `Step` reports, so a phase that ignores an error cannot smuggle work past a limit.

Document on `Blocks()` that a phase using it must forward `collision.Result.Unknown` to `MissingBlocks`, because `collision` reports incompleteness in its own return value and the kernel cannot see inside it.

- [x] **Step 4: Write `sim/kernel.go`**

Implement `NewKernel` and `Step` following the five steps described above. `NewKernel` validates the profile identity with `ProfileID.validate` and rejects duplicate phase identifiers. `Step` returns `(TickResult, error)` where the result is still digested on the incomplete path, because an incomplete tick is a result a caller may record, and returns a zero-change result on the error paths.

- [x] **Step 5: Write `sim/fake.go`**

```go
package sim

// StaticProfile is a profile whose answers are fields.
//
// It ships in the non-test build for the same reason world.Blocks does: runtime
// cannot be tested without a profile, and the fixture runner a later milestone
// adds needs one to build scenarios against. It implements no game rule, and no
// official profile is built from it.
type StaticProfile struct {
	Identity      ProfileID
	PhaseList     []Phase
	Slipperiness_ map[world.BlockRef]float64
	Default       float64
	Motions       map[entity.Family]MotionConstants
	Shapes        map[world.BlockRef]geom.Shape
}
```

with methods satisfying `Profile`. `Slipperiness` falls back to `Default`, `Motion` returns the zero value for an unknown family, `Shape` reports `false` for an unknown handle, and `Phases` returns the list. Rename the awkward `Slipperiness_` field to whatever reads well once the method set is written; the field and the method cannot share a name.

- [x] **Step 6: Run the tests, then pin the digest**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./sim/ -v`

`TestEmptyTickDigestIsPinned` fails first with the real digest in its message. Copy that value into the constant and re-run. Every other test must pass without editing an expectation.

- [x] **Step 7: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [x] **Step 8: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add sim/profile.go sim/kernel.go sim/fake.go sim/kernel_test.go
git commit -m "feat(sim): add the profile contract, tick phases, and the kernel"
```

---

## Task 10: The in-memory store and the revision check

**Files:**
- Create: `runtime/store.go`
- Create: `runtime/memory.go`
- Test: `runtime/memory_test.go`

**Interfaces:**
- Consumes: `sim`, `world`, `entity`, `geom`.
- Produces:
  - `var ErrStaleRevision = errors.New(...)`
  - `var ErrUnknownBlock = errors.New(...)`
  - `type Store interface { Revision() sim.Revision; Blocks() world.View; Entities() entity.View; Apply(sim.ChangeSet) error }`
  - `type Memory struct { ... }` with `NewMemory(sim.Profile) *Memory`, the `Store` methods, `Snapshot() *Memory`, and `SetBlock`/`SetEntity` builders

`Apply` is all or nothing, and it checks the revision before it writes anything. A set whose `BaseRevision` is not the store's current revision is refused with `ErrStaleRevision`, which is what lets a client apply a predicted change set to a forked snapshot and throw it away after a correction without the store ever having seen it.

`Apply` validates every operation before applying any of them: a set containing one unresolvable block handle changes nothing at all. Doing it in two passes is what makes "fully applicable or not applicable" true rather than aspirational.

`Snapshot` returns a store at the same revision that shares no state, which is the fork the client prediction path needs. It copies, and the doc comment says so.

- [x] **Step 1: Write the failing test**

Cover: a fresh store is at revision zero; applying an empty set at the right revision bumps the revision; applying at a stale revision returns `ErrStaleRevision` and changes nothing; applying at a *future* revision is refused too; a set-entity operation is readable afterwards; a remove-entity operation makes the body absent; a set-block operation is visible through both `CollisionShape` and `BlockState`; a set containing an unknown handle is refused whole, with an earlier valid operation in the same set proving nothing was written; and a snapshot does not follow its origin.

- [x] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./runtime/`

Expected: FAIL to build, with `undefined: NewMemory`.

- [x] **Step 3: Write `runtime/store.go` and `runtime/memory.go`**

`Memory` holds a revision, a `*world.Blocks`, a `*entity.Bodies`, and the profile it resolves handles through. `Apply` runs the revision check, then a validation pass that resolves every `OpSetBlock` handle through `Profile.Shape`, then the write pass, then increments the revision.

- [x] **Step 4: Run the tests to verify they pass**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./runtime/ -v`

- [x] **Step 5: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [x] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add runtime/store.go runtime/memory.go runtime/memory_test.go
git commit -m "feat(runtime): add the in-memory store and the revision check"
```

---

## Task 11: The tick runner, and the milestone record

**Files:**
- Create: `runtime/runner.go`
- Test: `runtime/runner_test.go`
- Modify: `README.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: everything above.
- Produces:
  - `type Runner struct { ... }` with `NewRunner(Store, sim.Kernel, sim.Profile) *Runner`
  - `func (r *Runner) Step(ctx context.Context) (sim.TickResult, error)`
  - `func (r *Runner) Tick() sim.Tick`
  - `func (r *Runner) SetLimits(sim.Limits)`, `func (r *Runner) SetScope(sim.Scope)`, `func (r *Runner) SetRandom(sim.RandomState)`

`Step` builds a `TickInput` from the store's current revision, views, and the runner's tick, scope, random state, and limits; runs the kernel; and applies the change set only when the result is complete. It advances the tick counter whether or not the result was applicable, because a tick that could not run still happened.

The runner keeps the random state the result returned, so a sequence of ticks draws a sequence of numbers rather than the same one repeatedly.

- [x] **Step 1: Write the failing test**

Cover: stepping an empty profile twice advances the tick and leaves the revision alone, since an empty change set still applies but changes nothing — decide and document which of those two it is, and assert it; a phase that writes an entity leaves it readable in the store afterwards; the second step sees the first step's revision as its base; an incomplete result is not applied and the store's revision does not move; and the runner's random state follows the results.

**The exit criterion belongs here.** Add a test that:

1. Runs one tick that writes an entity, and applies it, taking the store to revision one.
2. Takes the resulting change set, which is based at revision zero.
3. Applies it again to the same store and requires `ErrStaleRevision`.

That is the milestone's second gate: a change set that a store at a newer revision rejects.

- [x] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./runtime/`

- [x] **Step 3: Write `runtime/runner.go`**

- [x] **Step 4: Run the tests to verify they pass**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./runtime/ -v`

- [x] **Step 5: Record the packages in the README**

Extend the package table with `sim`, `entity`, and `runtime`, and add a short paragraph stating the dependency direction and the one deviation from the parent layout, so a reader does not have to find it in this plan.

- [x] **Step 6: Add the changelog entry**

Under `## Unreleased`, `### Added`:

```markdown
- `sim`, `entity`, and `runtime`: the tick contract, entity bodies, a canonical
  result digest, and an in-memory store whose revision check refuses a change
  set computed against older state.
```

- [x] **Step 7: Run the full verification**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

Expected: `lint`, `secrets`, `test`, `vuln`, and `build` all pass. `internal/oracle` still passes untouched: nothing in this milestone changes collision, and if an oracle test fails here, this milestone broke M8.2.

- [x] **Step 8: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add runtime/runner.go runtime/runner_test.go README.md CHANGELOG.md
git commit -m "feat(runtime): add the tick runner, and record M8.3"
```

---

## Definition of done

M8.3 is complete when every statement below is true:

- `sim` declares the tick contract the sequencing design froze: `Step` is the only entry point, a result carries its own digest, a change set names its base revision, and an incomplete result names what was missing and carries no applicable work.
- A profile reaches the kernel through an interface that names no protocol type, and `sim`, `world`, `entity`, `collision`, and `runtime` import no protocol package.
- **An empty tick produces a stable digest.** The same input digests identically across runs, the digest is pinned in a test, and changing any field of a result changes it.
- **A stale store rejects a change set.** A set computed against revision N is refused by a store at revision N+1, with nothing written.
- `entity.View.IDs` and every returned slice have a defined order, and no result depends on map iteration.
- The canonical encoder folds negative zero and NaN, and a NaN reaching a result is an error rather than a hash.
- Work limits are enforced, and exhausting one aborts the tick with no applicable change set. Cancellation and a phase error do the same.
- `runtime.Memory` applies a change set atomically and can fork a snapshot that does not follow its origin.
- `devbox run -- task verify` passes, `internal/oracle` included.

## Follow-on

**M8.4 must extend `internal/oracle` to movement.** M8.2's differential harness runs the game's own `Entity.moveEntity`; extending it to a whole movement tick means driving the game's `moveEntityWithHeading` and per-tick motion updates the same way and comparing trajectories. The sequencing design gates M8.4 on recorded fixtures and moves the live-server check to M8.8, and it says plainly that fixtures are the weaker signal: "If M8.4 passes fixtures but M8.8 draws corrections, the fault is likelier in the fixtures than in the kernel." A harness that asks the game removes that risk at the point the code is written rather than three milestones later. It also found both of M8.2's real bugs, neither of which a fixture derived from the same misreading would have caught.

**M8.4 fixes `MotionConstants` and the phase list.** This plan declares the four constants `physics.json` records and one profile-owned phase list. What phases a 1.8.9 player tick has, and in what order, is M8.4's decision, made against real movement rather than guessed at here.

**M8.6 inherits this encoding.** The canonical encoder and the digest land here, as the sequencing design allows. M8.6 adds the cross-platform matrix that hashes them, and its risk is unchanged and unresolved: whether CI capacity exists for three operating systems on two architectures is still unverified, and the design says confirm it before that milestone starts rather than during.
