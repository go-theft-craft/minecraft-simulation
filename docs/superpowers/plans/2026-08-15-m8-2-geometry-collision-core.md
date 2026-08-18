# M8.2 Geometry and Collision Core Implementation Plan

> **Status: complete, 2026-08-18.** Shipped as M8.2: `geom`, `world`, and
> `collision`, with the three exit properties and a differential harness that
> agrees with a real 1.8.9 server bit for bit across 2,872 whole moves. The
> checkboxes below were never ticked and are not evidence; do not re-run this
> plan.
>
> One thing here has been superseded rather than completed: M8.4 measured what
> this plan's prose statements about vanilla were worth and found two of the
> eight wrong, with no fixture written from the same prose catching either.
> Fixtures the game generates through `internal/oracle` are the catalog now.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver `geom`, `collision`, and a minimal world view in `minecraft-simulation`, reproducing Java Edition 1.8.9 swept-AABB movement exactly, with no entity types and no profiles.

**Architecture:** `geom` holds pure value types: vectors, block positions, axis-aligned boxes, and voxel shapes. `world` declares the tri-state block view the kernel reads through. `collision` gathers candidate block boxes for a swept region and resolves motion one axis at a time in vanilla's order, with the step-up retry layered on top. Nothing here knows about entities, profiles, packets, or the protocol module.

**Tech Stack:** Go 1.26.6, standard library only, Devbox, go-task, `gofumpt` plus `gci`.

## Global Constraints

- Module is `github.com/go-theft-craft/minecraft-simulation`. Go directive is `1.26.6`.
- **Core simulation packages import no protocol package.** `geom`, `world`, and `collision` must not import `github.com/go-theft-craft/minecraft-protocol/...`. Block shapes reach `collision` through the `world.BlockView` interface, which the caller implements. A later profile package adapts `data.CollisionShapes` into `geom.Shape`; that adaptation is not in this plan.
- Standard library only. No new module dependencies.
- Determinism: no wall clock, no global random state, no map iteration in any result. Every returned slice has a defined order.
- All geometry arithmetic is `float64`. Do not introduce `float32` anywhere in this plan; the trigonometry table is the only `float32` in the project and it belongs to movement, not collision.
- No decompiled Java, no Mojang asset, and no source text from the reference workspace is committed. Behavior is described in prose and reproduced independently.
- Formatting is `gci` with the section order `standard, default, prefix(github.com/go-theft-craft), prefix(github.com/go-theft-craft/minecraft-simulation)`, then `gofumpt`. Run `devbox run -- task fmt` before every commit.
- Commit messages use Conventional Commits. Never add a `Co-Authored-By` or `Claude-Session` trailer.
- Every task ends with `devbox run -- task verify` passing.

## Vanilla behavior this plan reproduces

Recorded from the 1.8.9 server reference workspace during planning. These are behavioral statements, not source:

1. **Axis order is Y, then X, then Z.** The entity box is translated after each axis resolves, so the X pass sees the box already moved in Y.
2. **Candidates are gathered once**, from the entity box expanded along the full motion vector, before any axis resolves. The step-up retry gathers a second time with the step height substituted for the Y motion.
3. **Each offset test is one-sided and has no epsilon in 1.8.9.** A block box clamps motion only when the other two axes strictly overlap, using `<=` and `>=` on the far edges. Later versions added an epsilon; 1.8.9 did not.
4. **The receiver is the block box and the argument is the entity box.** `blockBox.CalculateXOffset(entityBox, motion)`.
5. **Step-up runs when** the step height is positive, the entity was on the ground or its downward Y motion was clamped this tick, and X or Z motion was clamped.
6. **Step-up computes two candidate outcomes**, both starting from the box as it stood *before* the move, and keeps whichever travels further in the horizontal plane, compared by `x*x + z*z`. The two differ only in the box their Y clamp tests: the first tests a box stretched horizontally by the motion, so it rises for a block it is about to move over; the second tests the plain box, so it rises only for what is directly above. A tie goes to the second.
7. **The stepped outcome then settles downward**, and only then is it compared against the unstepped result. If the unstepped horizontal distance is greater than or equal to the stepped one, the unstepped result stands.
8. **`onGround` is true when** Y motion was clamped while moving downward.

## File structure

```text
minecraft-simulation/
  geom/vector.go          Vec3, BlockPos, floor conversion
  geom/vector_test.go
  geom/aabb.go            AABB and its translations, expansions, overlap test
  geom/aabb_test.go
  geom/offset.go          The three single-axis clamp functions
  geom/offset_test.go
  geom/shape.go           Shape: an immutable ordered set of boxes for one block
  geom/shape_test.go
  world/view.go           BlockView, Lookup tri-state
  world/view_test.go
  world/fake.go           A deterministic in-memory BlockView for tests
  world/fake_test.go
  collision/candidates.go Broad phase over a swept region, with a work limit
  collision/candidates_test.go
  collision/resolve.go    Move, Result, Resolve: axis passes and step-up
  collision/resolve_test.go
  collision/property_test.go  No tunneling, step bound, zero motion fixed point
```

`world/fake.go` ships in the non-test build on purpose: `collision` tests need it, and a later `runtime` package needs an in-memory store to test against. It is small and has no dependencies beyond `geom`.

---

## Task 1: Vectors and block positions

**Files:**
- Create: `geom/vector.go`
- Test: `geom/vector_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Vec3 struct { X, Y, Z float64 }`
  - `func (v Vec3) Add(other Vec3) Vec3`
  - `func (v Vec3) Sub(other Vec3) Vec3`
  - `func (v Vec3) Scale(factor float64) Vec3`
  - `func (v Vec3) HorizontalLengthSquared() float64`
  - `func (v Vec3) IsZero() bool`
  - `type BlockPos struct { X, Y, Z int32 }`
  - `func Floor(value float64) int32`
  - `func BlockPosOf(v Vec3) BlockPos`

`Floor` rounds toward negative infinity, which is what block coordinates need: the block containing `-0.5` is `-1`, not `0`. Go's integer conversion truncates toward zero, so a plain cast is wrong for negative coordinates and would place an entity in the wrong block on the negative side of the origin.

- [ ] **Step 1: Write the failing test**

```go
package geom

import (
	"math"
	"testing"
)

func TestFloorRoundsTowardNegativeInfinity(t *testing.T) {
	for _, test := range []struct {
		value float64
		want  int32
	}{
		{0, 0},
		{0.5, 0},
		{1, 1},
		{-0.5, -1},
		{-1, -1},
		{-1.5, -2},
		{63.999999, 63},
		{-63.000001, -64},
	} {
		if got := Floor(test.value); got != test.want {
			t.Errorf("Floor(%v) = %d, want %d", test.value, got, test.want)
		}
	}
}

func TestBlockPosOfUsesFloorOnEveryAxis(t *testing.T) {
	got := BlockPosOf(Vec3{X: -0.5, Y: 4.9, Z: -8.1})
	want := BlockPos{X: -1, Y: 4, Z: -9}
	if got != want {
		t.Fatalf("BlockPosOf = %+v, want %+v", got, want)
	}
}

func TestVec3Arithmetic(t *testing.T) {
	a := Vec3{X: 1, Y: 2, Z: 3}
	b := Vec3{X: 0.5, Y: -1, Z: 2}

	if got := a.Add(b); got != (Vec3{X: 1.5, Y: 1, Z: 5}) {
		t.Errorf("Add = %+v", got)
	}
	if got := a.Sub(b); got != (Vec3{X: 0.5, Y: 3, Z: 1}) {
		t.Errorf("Sub = %+v", got)
	}
	if got := a.Scale(2); got != (Vec3{X: 2, Y: 4, Z: 6}) {
		t.Errorf("Scale = %+v", got)
	}
}

func TestHorizontalLengthSquaredIgnoresY(t *testing.T) {
	v := Vec3{X: 3, Y: 100, Z: 4}
	if got := v.HorizontalLengthSquared(); got != 25 {
		t.Fatalf("HorizontalLengthSquared = %v, want 25", got)
	}
}

func TestIsZero(t *testing.T) {
	if !(Vec3{}).IsZero() {
		t.Error("the zero vector is not reported as zero")
	}
	if (Vec3{X: math.SmallestNonzeroFloat64}).IsZero() {
		t.Error("a tiny non-zero vector is reported as zero")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./geom/`

Expected: FAIL to build, with `undefined: Floor` and `undefined: Vec3`.

- [ ] **Step 3: Write the implementation**

```go
// Package geom holds the value types the simulation measures space with:
// vectors, block positions, axis-aligned boxes, and per-block voxel shapes.
//
// Every value in this package is immutable and every operation returns a new
// value. Nothing here reads the clock, allocates shared state, or depends on
// any package outside the standard library.
package geom

import "math"

// Vec3 is a position or a motion in world space.
type Vec3 struct {
	X, Y, Z float64
}

// Add returns the component-wise sum.
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{X: v.X + other.X, Y: v.Y + other.Y, Z: v.Z + other.Z}
}

// Sub returns the component-wise difference.
func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{X: v.X - other.X, Y: v.Y - other.Y, Z: v.Z - other.Z}
}

// Scale returns the vector multiplied by factor.
func (v Vec3) Scale(factor float64) Vec3 {
	return Vec3{X: v.X * factor, Y: v.Y * factor, Z: v.Z * factor}
}

// HorizontalLengthSquared returns the squared length in the XZ plane. Step-up
// picks a winner with this, so it must ignore Y and must not take a root: the
// comparison is exact this way.
func (v Vec3) HorizontalLengthSquared() float64 {
	return v.X*v.X + v.Z*v.Z
}

// IsZero reports whether every component is exactly zero.
func (v Vec3) IsZero() bool {
	return v.X == 0 && v.Y == 0 && v.Z == 0
}

// BlockPos identifies one block cell.
type BlockPos struct {
	X, Y, Z int32
}

// Floor rounds toward negative infinity. A Go conversion truncates toward
// zero, which puts every negative coordinate in the wrong cell.
func Floor(value float64) int32 {
	return int32(math.Floor(value))
}

// BlockPosOf returns the cell containing v.
func BlockPosOf(v Vec3) BlockPos {
	return BlockPos{X: Floor(v.X), Y: Floor(v.Y), Z: Floor(v.Z)}
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./geom/ -v`

Expected: PASS, five tests.

- [ ] **Step 5: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

Expected: all tasks pass.

- [ ] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add geom/vector.go geom/vector_test.go
git commit -m "feat(geom): add vectors and block positions"
```

---

## Task 2: Axis-aligned boxes

**Files:**
- Create: `geom/aabb.go`
- Test: `geom/aabb_test.go`

**Interfaces:**
- Consumes: `Vec3`, `BlockPos` from Task 1.
- Produces:
  - `type AABB struct { MinX, MinY, MinZ, MaxX, MaxY, MaxZ float64 }`
  - `func BlockAABB(pos BlockPos) AABB`
  - `func (b AABB) Offset(delta Vec3) AABB`
  - `func (b AABB) Stretch(delta Vec3) AABB`
  - `func (b AABB) Intersects(other AABB) bool`

`Stretch` is the sweep expansion: it grows the box in the direction of motion only, leaving the opposite face where it was, so the result covers every cell the box can reach this tick. Vanilla calls this `addCoord`; the name here says what it does.

- [ ] **Step 1: Write the failing test**

```go
package geom

import "testing"

func unit() AABB {
	return AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 1, MaxZ: 1}
}

func TestBlockAABBCoversExactlyOneCell(t *testing.T) {
	got := BlockAABB(BlockPos{X: -1, Y: 4, Z: 2})
	want := AABB{MinX: -1, MinY: 4, MinZ: 2, MaxX: 0, MaxY: 5, MaxZ: 3}
	if got != want {
		t.Fatalf("BlockAABB = %+v, want %+v", got, want)
	}
}

func TestOffsetMovesBothFaces(t *testing.T) {
	got := unit().Offset(Vec3{X: 2, Y: -1, Z: 0.5})
	want := AABB{MinX: 2, MinY: -1, MinZ: 0.5, MaxX: 3, MaxY: 0, MaxZ: 1.5}
	if got != want {
		t.Fatalf("Offset = %+v, want %+v", got, want)
	}
}

func TestStretchGrowsOnlyTowardMotion(t *testing.T) {
	positive := unit().Stretch(Vec3{X: 2, Y: 0, Z: 0})
	if positive.MinX != 0 || positive.MaxX != 3 {
		t.Fatalf("positive stretch = %+v, want the far face to move", positive)
	}

	negative := unit().Stretch(Vec3{X: -2, Y: 0, Z: 0})
	if negative.MinX != -2 || negative.MaxX != 1 {
		t.Fatalf("negative stretch = %+v, want the near face to move", negative)
	}

	if got := unit().Stretch(Vec3{}); got != unit() {
		t.Fatalf("zero stretch changed the box: %+v", got)
	}
}

func TestStretchCoversEveryAxisAtOnce(t *testing.T) {
	got := unit().Stretch(Vec3{X: 1, Y: -1, Z: 2})
	want := AABB{MinX: 0, MinY: -1, MinZ: 0, MaxX: 2, MaxY: 1, MaxZ: 3}
	if got != want {
		t.Fatalf("Stretch = %+v, want %+v", got, want)
	}
}

func TestIntersectsIsStrictAndSymmetric(t *testing.T) {
	for _, test := range []struct {
		name  string
		other AABB
		want  bool
	}{
		{"overlapping", AABB{MinX: 0.5, MinY: 0.5, MinZ: 0.5, MaxX: 2, MaxY: 2, MaxZ: 2}, true},
		{"touching faces only", AABB{MinX: 1, MinY: 0, MinZ: 0, MaxX: 2, MaxY: 1, MaxZ: 1}, false},
		{"separated", AABB{MinX: 2, MinY: 0, MinZ: 0, MaxX: 3, MaxY: 1, MaxZ: 1}, false},
		{"contained", AABB{MinX: 0.2, MinY: 0.2, MinZ: 0.2, MaxX: 0.8, MaxY: 0.8, MaxZ: 0.8}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := unit().Intersects(test.other); got != test.want {
				t.Errorf("Intersects = %v, want %v", got, test.want)
			}
			if got := test.other.Intersects(unit()); got != test.want {
				t.Errorf("reversed Intersects = %v, want %v", got, test.want)
			}
		})
	}
}
```

Touching faces must not count as intersecting. An entity standing exactly on a floor shares a plane with it every tick, and a non-strict test would report a permanent collision.

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./geom/ -run 'TestBlockAABB|TestOffset|TestStretch|TestIntersects'`

Expected: FAIL to build, with `undefined: AABB`.

- [ ] **Step 3: Write the implementation**

```go
package geom

// AABB is an axis-aligned box in world space. A box is valid when each Min is
// no greater than its Max; the package does not normalize, because vanilla
// does not, and silently reordering faces would hide a caller's bug.
type AABB struct {
	MinX, MinY, MinZ float64
	MaxX, MaxY, MaxZ float64
}

// BlockAABB returns the full-cube box of one block cell.
func BlockAABB(pos BlockPos) AABB {
	return AABB{
		MinX: float64(pos.X),
		MinY: float64(pos.Y),
		MinZ: float64(pos.Z),
		MaxX: float64(pos.X) + 1,
		MaxY: float64(pos.Y) + 1,
		MaxZ: float64(pos.Z) + 1,
	}
}

// Offset translates the box.
func (b AABB) Offset(delta Vec3) AABB {
	return AABB{
		MinX: b.MinX + delta.X,
		MinY: b.MinY + delta.Y,
		MinZ: b.MinZ + delta.Z,
		MaxX: b.MaxX + delta.X,
		MaxY: b.MaxY + delta.Y,
		MaxZ: b.MaxZ + delta.Z,
	}
}

// Stretch extends the box along delta without moving the opposite faces, so
// the result covers every point the box passes through while travelling delta.
func (b AABB) Stretch(delta Vec3) AABB {
	stretched := b
	if delta.X < 0 {
		stretched.MinX += delta.X
	} else {
		stretched.MaxX += delta.X
	}
	if delta.Y < 0 {
		stretched.MinY += delta.Y
	} else {
		stretched.MaxY += delta.Y
	}
	if delta.Z < 0 {
		stretched.MinZ += delta.Z
	} else {
		stretched.MaxZ += delta.Z
	}

	return stretched
}

// Intersects reports whether the boxes overlap in a volume. Boxes that only
// share a face do not intersect: an entity resting on the ground touches it
// every tick, and treating that as an overlap would report a standing
// collision forever.
func (b AABB) Intersects(other AABB) bool {
	return b.MinX < other.MaxX && b.MaxX > other.MinX &&
		b.MinY < other.MaxY && b.MaxY > other.MinY &&
		b.MinZ < other.MaxZ && b.MaxZ > other.MinZ
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./geom/ -v`

Expected: PASS.

- [ ] **Step 5: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [ ] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add geom/aabb.go geom/aabb_test.go
git commit -m "feat(geom): add axis-aligned boxes"
```

---

## Task 3: Single-axis clamps

**Files:**
- Create: `geom/offset.go`
- Test: `geom/offset_test.go`

**Interfaces:**
- Consumes: `AABB` from Task 2.
- Produces:
  - `func (b AABB) ClampX(mover AABB, motion float64) float64`
  - `func (b AABB) ClampY(mover AABB, motion float64) float64`
  - `func (b AABB) ClampZ(mover AABB, motion float64) float64`

The receiver is the blocking box and `mover` is the entity box, matching vanilla's call direction. Each function returns the motion the mover may still apply on that axis.

There is no epsilon. 1.8.9 compares far edges with `<=` and `>=` and stops there. Adding a tolerance would change contact positions by a measurable amount and break the M8.4 exit criterion, which requires a vanilla server to send zero corrections.

- [ ] **Step 1: Write the failing test**

```go
package geom

import "testing"

// mover is a one-cube entity box sitting at the origin.
func mover() AABB {
	return AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 1, MaxZ: 1}
}

func TestClampXStopsAtTheBlockFace(t *testing.T) {
	// A block occupying x in [2,3]. Moving +5 may only travel 1.
	block := AABB{MinX: 2, MinY: 0, MinZ: 0, MaxX: 3, MaxY: 1, MaxZ: 1}

	if got := block.ClampX(mover(), 5); got != 1 {
		t.Fatalf("ClampX = %v, want 1", got)
	}
	if got := block.ClampX(mover(), 0.25); got != 0.25 {
		t.Fatalf("ClampX shortened a motion that does not reach: %v", got)
	}
	if got := block.ClampX(mover(), -5); got != -5 {
		t.Fatalf("ClampX clamped motion moving away from the block: %v", got)
	}
}

func TestClampXStopsAtTheBlockFaceMovingNegative(t *testing.T) {
	// A block occupying x in [-3,-2]. Moving -5 may only travel -2.
	block := AABB{MinX: -3, MinY: 0, MinZ: 0, MaxX: -2, MaxY: 1, MaxZ: 1}

	if got := block.ClampX(mover(), -5); got != -2 {
		t.Fatalf("ClampX = %v, want -2", got)
	}
}

func TestClampIgnoresBlocksThatDoNotOverlapTheOtherAxes(t *testing.T) {
	// Directly ahead in X, but one whole cell up in Y: no overlap, no clamp.
	block := AABB{MinX: 2, MinY: 5, MinZ: 0, MaxX: 3, MaxY: 6, MaxZ: 1}

	if got := block.ClampX(mover(), 5); got != 5 {
		t.Fatalf("ClampX = %v, want the motion unchanged", got)
	}
}

func TestClampTreatsTouchingAsNoOverlap(t *testing.T) {
	// The block's Y range starts exactly where the mover's ends. 1.8.9 uses
	// >= and <= on the far edges, so this is not an overlap and must not clamp.
	block := AABB{MinX: 2, MinY: 1, MinZ: 0, MaxX: 3, MaxY: 2, MaxZ: 1}

	if got := block.ClampX(mover(), 5); got != 5 {
		t.Fatalf("ClampX = %v, want the motion unchanged", got)
	}
}

func TestClampYAndClampZMirrorClampX(t *testing.T) {
	above := AABB{MinX: 0, MinY: 2, MinZ: 0, MaxX: 1, MaxY: 3, MaxZ: 1}
	if got := above.ClampY(mover(), 5); got != 1 {
		t.Fatalf("ClampY = %v, want 1", got)
	}

	ahead := AABB{MinX: 0, MinY: 0, MinZ: 2, MaxX: 1, MaxY: 1, MaxZ: 3}
	if got := ahead.ClampZ(mover(), 5); got != 1 {
		t.Fatalf("ClampZ = %v, want 1", got)
	}
}

func TestClampNeverReversesMotion(t *testing.T) {
	// A box already overlapping the mover must not push it backwards.
	overlapping := AABB{MinX: 0.5, MinY: 0, MinZ: 0, MaxX: 1.5, MaxY: 1, MaxZ: 1}

	if got := overlapping.ClampX(mover(), 2); got != 2 {
		t.Fatalf("ClampX = %v, want the motion unchanged", got)
	}
	if got := overlapping.ClampX(mover(), -2); got != -2 {
		t.Fatalf("ClampX = %v, want the motion unchanged", got)
	}
}
```

The last case matters: vanilla only clamps when the mover is strictly on the approaching side (`mover.MaxX <= b.MinX` for positive motion). An already-overlapping box returns the motion untouched rather than ejecting the entity. Resolution of overlap is not collision's job.

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./geom/ -run TestClamp`

Expected: FAIL to build, with `b.ClampX undefined`.

- [ ] **Step 3: Write the implementation**

```go
package geom

// ClampX returns how far mover may still travel along X before it would enter
// b. The receiver is the blocking box and mover is the moving box.
//
// The other two axes must strictly overlap for b to block at all. There is no
// tolerance term: Java Edition 1.8.9 compares the far edges directly, and a
// tolerance here would move every contact position by that amount.
func (b AABB) ClampX(mover AABB, motion float64) float64 {
	if mover.MaxY <= b.MinY || mover.MinY >= b.MaxY {
		return motion
	}
	if mover.MaxZ <= b.MinZ || mover.MinZ >= b.MaxZ {
		return motion
	}

	if motion > 0 && mover.MaxX <= b.MinX {
		if gap := b.MinX - mover.MaxX; gap < motion {
			return gap
		}
	}
	if motion < 0 && mover.MinX >= b.MaxX {
		if gap := b.MaxX - mover.MinX; gap > motion {
			return gap
		}
	}

	return motion
}

// ClampY returns how far mover may still travel along Y before it would enter
// b. See ClampX for the shared rules.
func (b AABB) ClampY(mover AABB, motion float64) float64 {
	if mover.MaxX <= b.MinX || mover.MinX >= b.MaxX {
		return motion
	}
	if mover.MaxZ <= b.MinZ || mover.MinZ >= b.MaxZ {
		return motion
	}

	if motion > 0 && mover.MaxY <= b.MinY {
		if gap := b.MinY - mover.MaxY; gap < motion {
			return gap
		}
	}
	if motion < 0 && mover.MinY >= b.MaxY {
		if gap := b.MaxY - mover.MinY; gap > motion {
			return gap
		}
	}

	return motion
}

// ClampZ returns how far mover may still travel along Z before it would enter
// b. See ClampX for the shared rules.
func (b AABB) ClampZ(mover AABB, motion float64) float64 {
	if mover.MaxX <= b.MinX || mover.MinX >= b.MaxX {
		return motion
	}
	if mover.MaxY <= b.MinY || mover.MinY >= b.MaxY {
		return motion
	}

	if motion > 0 && mover.MaxZ <= b.MinZ {
		if gap := b.MinZ - mover.MaxZ; gap < motion {
			return gap
		}
	}
	if motion < 0 && mover.MinZ >= b.MaxZ {
		if gap := b.MaxZ - mover.MinZ; gap > motion {
			return gap
		}
	}

	return motion
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./geom/ -v -run TestClamp`

Expected: PASS, six tests.

- [ ] **Step 5: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [ ] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add geom/offset.go geom/offset_test.go
git commit -m "feat(geom): add single-axis collision clamps"
```

---

## Task 4: Voxel shapes

**Files:**
- Create: `geom/shape.go`
- Test: `geom/shape_test.go`

**Interfaces:**
- Consumes: `AABB`, `BlockPos` from Tasks 1 and 2.
- Produces:
  - `type Shape struct { ... }` with an unexported box slice
  - `func NewShape(boxes ...AABB) Shape`
  - `func FullCube() Shape`
  - `func EmptyShape() Shape`
  - `func (s Shape) IsEmpty() bool`
  - `func (s Shape) Len() int`
  - `func (s Shape) BoxesAt(pos BlockPos, dst []AABB) []AABB`

A shape is stated in block-local coordinates, where the cell spans 0 to 1 on each axis. A fence, a slab, or a stair is several boxes; most blocks are one. `BoxesAt` translates them to a world cell and appends to `dst`, so the broad phase can reuse one buffer across thousands of cells and allocate nothing per block.

`Shape` copies its input in `NewShape` and never exposes the slice, so a caller cannot mutate a shape it already handed over. Shapes are shared across every cell of a block type and must stay immutable.

- [ ] **Step 1: Write the failing test**

```go
package geom

import (
	"reflect"
	"testing"
)

func TestFullCubeIsTheUnitBox(t *testing.T) {
	got := FullCube().BoxesAt(BlockPos{}, nil)
	want := []AABB{{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 1, MaxZ: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FullCube = %+v, want %+v", got, want)
	}
}

func TestEmptyShapeYieldsNoBoxes(t *testing.T) {
	if !EmptyShape().IsEmpty() {
		t.Error("EmptyShape is not empty")
	}
	if got := EmptyShape().BoxesAt(BlockPos{X: 3}, nil); len(got) != 0 {
		t.Fatalf("EmptyShape produced %d boxes", len(got))
	}
}

func TestBoxesAtTranslatesToTheCell(t *testing.T) {
	slab := NewShape(AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 0.5, MaxZ: 1})

	got := slab.BoxesAt(BlockPos{X: 2, Y: -1, Z: 5}, nil)
	want := []AABB{{MinX: 2, MinY: -1, MinZ: 5, MaxX: 3, MaxY: -0.5, MaxZ: 6}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BoxesAt = %+v, want %+v", got, want)
	}
}

func TestBoxesAtAppendsAndPreservesOrder(t *testing.T) {
	fence := NewShape(
		AABB{MinX: 0.375, MinY: 0, MinZ: 0, MaxX: 0.625, MaxY: 1.5, MaxZ: 1},
		AABB{MinX: 0, MinY: 0, MinZ: 0.375, MaxX: 1, MaxY: 1.5, MaxZ: 0.625},
	)

	dst := []AABB{{MinX: -99}}
	got := fence.BoxesAt(BlockPos{}, dst)
	if len(got) != 3 {
		t.Fatalf("BoxesAt returned %d boxes, want the existing one plus two", len(got))
	}
	if got[0].MinX != -99 {
		t.Error("BoxesAt overwrote the destination slice")
	}
	if got[1].MinX != 0.375 || got[2].MinZ != 0.375 {
		t.Fatalf("BoxesAt reordered the shape: %+v", got[1:])
	}
}

func TestNewShapeCopiesItsInput(t *testing.T) {
	boxes := []AABB{{MaxX: 1, MaxY: 1, MaxZ: 1}}
	shape := NewShape(boxes...)
	boxes[0].MaxY = 99

	got := shape.BoxesAt(BlockPos{}, nil)
	if got[0].MaxY != 1 {
		t.Fatalf("mutating the caller's slice changed the shape: %+v", got[0])
	}
}

func TestShapeLen(t *testing.T) {
	if got := FullCube().Len(); got != 1 {
		t.Errorf("FullCube.Len = %d, want 1", got)
	}
	if got := EmptyShape().Len(); got != 0 {
		t.Errorf("EmptyShape.Len = %d, want 0", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./geom/ -run 'TestFullCube|TestEmptyShape|TestBoxesAt|TestNewShape|TestShapeLen'`

Expected: FAIL to build, with `undefined: FullCube`.

- [ ] **Step 3: Write the implementation**

```go
package geom

import "slices"

// Shape is a block's collision volume in block-local coordinates, where the
// cell spans zero to one on every axis. Most blocks are one box; fences,
// slabs, and stairs are several.
//
// A Shape is immutable and safe to share across every cell of a block type.
type Shape struct {
	boxes []AABB
}

// NewShape returns a shape holding a copy of boxes, in the order given. The
// order is preserved because collision resolution visits boxes in order and
// the result must not depend on how the caller built the slice.
func NewShape(boxes ...AABB) Shape {
	return Shape{boxes: slices.Clone(boxes)}
}

// FullCube returns the shape of an ordinary solid block.
func FullCube() Shape {
	return Shape{boxes: []AABB{{MaxX: 1, MaxY: 1, MaxZ: 1}}}
}

// EmptyShape returns the shape of a block nothing collides with.
func EmptyShape() Shape {
	return Shape{}
}

// IsEmpty reports whether the shape has no boxes.
func (s Shape) IsEmpty() bool {
	return len(s.boxes) == 0
}

// Len returns the number of boxes.
func (s Shape) Len() int {
	return len(s.boxes)
}

// BoxesAt appends the shape's boxes, translated into the cell at pos, to dst
// and returns the extended slice. Passing a reused buffer as dst lets a broad
// phase walk thousands of cells without allocating per cell.
func (s Shape) BoxesAt(pos BlockPos, dst []AABB) []AABB {
	origin := Vec3{X: float64(pos.X), Y: float64(pos.Y), Z: float64(pos.Z)}
	for _, box := range s.boxes {
		dst = append(dst, box.Offset(origin))
	}

	return dst
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./geom/ -v`

Expected: PASS.

- [ ] **Step 5: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [ ] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add geom/shape.go geom/shape_test.go
git commit -m "feat(geom): add per-block voxel shapes"
```

---

## Task 5: The block view and its in-memory implementation

**Files:**
- Create: `world/view.go`
- Create: `world/fake.go`
- Test: `world/view_test.go`

**Interfaces:**
- Consumes: `geom.BlockPos`, `geom.Shape` from Tasks 1 and 4.
- Produces:
  - `type Lookup uint8` with `LookupUnknown`, `LookupAir`, `LookupShape`
  - `func (l Lookup) String() string`
  - `type BlockView interface { CollisionShape(pos geom.BlockPos) (geom.Shape, Lookup) }`
  - `type Blocks struct { ... }`
  - `func NewBlocks() *Blocks`
  - `func (b *Blocks) Set(pos geom.BlockPos, shape geom.Shape)`
  - `func (b *Blocks) SetAir(pos geom.BlockPos)`
  - `func (b *Blocks) Forget(pos geom.BlockPos)`
  - `func (b *Blocks) Fill(from, to geom.BlockPos, shape geom.Shape)`
  - `func (b *Blocks) CollisionShape(pos geom.BlockPos) (geom.Shape, Lookup)`

The tri-state lookup is the parent design's rule: a view distinguishes known air, a known block state, and unknown. The kernel never fabricates state, so `collision` must be able to tell "nothing is there" from "nobody told me". `Blocks` defaults to `LookupUnknown` for anything never set, which makes the unknown path the one a test gets for free rather than the one it has to arrange.

- [ ] **Step 1: Write the failing test**

```go
package world

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestUnsetPositionsAreUnknown(t *testing.T) {
	blocks := NewBlocks()

	shape, lookup := blocks.CollisionShape(geom.BlockPos{X: 1, Y: 2, Z: 3})
	if lookup != LookupUnknown {
		t.Fatalf("lookup = %v, want LookupUnknown", lookup)
	}
	if !shape.IsEmpty() {
		t.Error("an unknown position returned a non-empty shape")
	}
}

func TestSetAirIsKnownAndEmpty(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{X: 1}
	blocks.SetAir(pos)

	shape, lookup := blocks.CollisionShape(pos)
	if lookup != LookupAir {
		t.Fatalf("lookup = %v, want LookupAir", lookup)
	}
	if !shape.IsEmpty() {
		t.Error("air returned a non-empty shape")
	}
}

func TestSetShapeIsReturned(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{Y: 4}
	blocks.Set(pos, geom.FullCube())

	shape, lookup := blocks.CollisionShape(pos)
	if lookup != LookupShape {
		t.Fatalf("lookup = %v, want LookupShape", lookup)
	}
	if shape.Len() != 1 {
		t.Fatalf("shape has %d boxes, want 1", shape.Len())
	}
}

func TestSetWithAnEmptyShapeRecordsAir(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{Z: 7}
	blocks.Set(pos, geom.EmptyShape())

	if _, lookup := blocks.CollisionShape(pos); lookup != LookupAir {
		t.Fatalf("lookup = %v, want LookupAir", lookup)
	}
}

func TestForgetRestoresUnknown(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{X: 2, Y: 2, Z: 2}
	blocks.Set(pos, geom.FullCube())
	blocks.Forget(pos)

	if _, lookup := blocks.CollisionShape(pos); lookup != LookupUnknown {
		t.Fatalf("lookup = %v, want LookupUnknown", lookup)
	}
}

func TestLookupString(t *testing.T) {
	for _, test := range []struct {
		lookup Lookup
		want   string
	}{
		{LookupUnknown, "unknown"},
		{LookupAir, "air"},
		{LookupShape, "shape"},
		{Lookup(99), "Lookup(99)"},
	} {
		if got := test.lookup.String(); got != test.want {
			t.Errorf("Lookup(%d).String() = %q, want %q", test.lookup, got, test.want)
		}
	}
}

// BlockView is satisfied by Blocks. A compile-time assertion is cheaper than
// discovering the mismatch from a collision test.
var _ BlockView = (*Blocks)(nil)
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./world/`

Expected: FAIL to build, with `undefined: NewBlocks`.

- [ ] **Step 3: Write `world/view.go`**

```go
// Package world declares the read-only views the simulation looks at the world
// through, and a deterministic in-memory implementation of them.
//
// A view distinguishes known air from an unknown region. The kernel never
// invents state: when a rule needs a region nobody has loaded, the result says
// so instead of guessing that it is empty.
package world

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// Lookup is the outcome of a block query.
type Lookup uint8

const (
	// LookupUnknown means nobody has told this view what is at the position.
	LookupUnknown Lookup = iota
	// LookupAir means the position is known to hold nothing collidable.
	LookupAir
	// LookupShape means the position holds a block with a collision shape.
	LookupShape
)

// String returns the lookup's name.
func (l Lookup) String() string {
	switch l {
	case LookupUnknown:
		return "unknown"
	case LookupAir:
		return "air"
	case LookupShape:
		return "shape"
	default:
		return fmt.Sprintf("Lookup(%d)", uint8(l))
	}
}

// BlockView answers collision queries about single block cells.
//
// An implementation must be deterministic: the same position must produce the
// same answer for the whole of a tick.
type BlockView interface {
	// CollisionShape returns the shape at pos in block-local coordinates. The
	// shape is empty unless the lookup is LookupShape.
	CollisionShape(pos geom.BlockPos) (geom.Shape, Lookup)
}
```

- [ ] **Step 4: Write `world/fake.go`**

```go
package world

import "github.com/go-theft-craft/minecraft-simulation/geom"

// Blocks is an in-memory BlockView. Every position starts unknown, so a test
// that means "empty space" has to say so, and a test that forgets to describe
// a region gets the unknown path rather than a silent floor of air.
//
// Blocks is not safe for concurrent modification. Build it, then read it.
type Blocks struct {
	shapes map[geom.BlockPos]geom.Shape
}

// NewBlocks returns an empty view in which every position is unknown.
func NewBlocks() *Blocks {
	return &Blocks{shapes: make(map[geom.BlockPos]geom.Shape)}
}

// Set records a block shape. An empty shape records air, because a block
// nothing collides with is indistinguishable from air to this package.
func (b *Blocks) Set(pos geom.BlockPos, shape geom.Shape) {
	b.shapes[pos] = shape
}

// SetAir records that the position holds nothing collidable.
func (b *Blocks) SetAir(pos geom.BlockPos) {
	b.shapes[pos] = geom.EmptyShape()
}

// Forget returns the position to unknown.
func (b *Blocks) Forget(pos geom.BlockPos) {
	delete(b.shapes, pos)
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
	shape, ok := b.shapes[pos]
	if !ok {
		return geom.EmptyShape(), LookupUnknown
	}
	if shape.IsEmpty() {
		return geom.EmptyShape(), LookupAir
	}

	return shape, LookupShape
}
```

`Fill` has no test of its own in this task; Task 6 uses it heavily and covers it.

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./world/ -v`

Expected: PASS, six tests.

- [ ] **Step 6: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [ ] **Step 7: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add world/
git commit -m "feat(world): add the tri-state block view and an in-memory implementation"
```

---

## Task 6: Candidate gathering

**Files:**
- Create: `collision/candidates.go`
- Test: `collision/candidates_test.go`

**Interfaces:**
- Consumes: `geom.AABB`, `geom.BlockPos`, `geom.Floor` from Tasks 1 to 4; `world.BlockView`, `world.Lookup` from Task 5.
- Produces:
  - `var ErrCandidateLimit = errors.New("collision: candidate limit exhausted")`
  - `type Candidates struct { Boxes []geom.AABB; Unknown []geom.BlockPos }`
  - `func Gather(view world.BlockView, region geom.AABB, limit int) (Candidates, error)`

`Gather` walks every cell the region touches, in a fixed X then Y then Z order, and appends each cell's translated boxes. Unknown cells are recorded in `Unknown` in the same walk order and contribute no boxes. A caller that receives a non-empty `Unknown` must treat the tick as incomplete rather than moving the entity.

The cell range runs from `Floor(min)` to `Floor(max)` inclusive. A region whose max edge lands exactly on a cell boundary still includes that cell: the cost is one extra column of lookups, and excluding it would miss a block the entity is flush against.

`limit` bounds the number of cells visited, satisfying the parent design's deterministic work limit for collision candidates. A non-positive limit means unlimited.

- [ ] **Step 1: Write the failing test**

```go
package collision

import (
	"errors"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

func TestGatherReturnsBoxesForSolidCells(t *testing.T) {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1, Y: -1, Z: -1}, geom.BlockPos{X: 1, Y: 1, Z: 1}, geom.EmptyShape())
	blocks.Set(geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.FullCube())

	region := geom.AABB{MinX: -0.5, MinY: -0.5, MinZ: -0.5, MaxX: 1.4, MaxY: 1.4, MaxZ: 1.4}
	got, err := Gather(blocks, region, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(got.Unknown) != 0 {
		t.Fatalf("Gather reported unknown cells: %+v", got.Unknown)
	}
	if len(got.Boxes) != 1 {
		t.Fatalf("Gather returned %d boxes, want 1", len(got.Boxes))
	}
	want := geom.AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 1, MaxZ: 1}
	if got.Boxes[0] != want {
		t.Fatalf("box = %+v, want %+v", got.Boxes[0], want)
	}
}

func TestGatherReportsUnknownCellsAndSkipsTheirBoxes(t *testing.T) {
	blocks := world.NewBlocks()
	// Only one cell is described; everything else in the region is unknown.
	blocks.Set(geom.BlockPos{}, geom.FullCube())

	region := geom.AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1.5, MaxY: 0.5, MaxZ: 0.5}
	got, err := Gather(blocks, region, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(got.Boxes) != 1 {
		t.Fatalf("Gather returned %d boxes, want 1", len(got.Boxes))
	}
	if len(got.Unknown) != 1 || got.Unknown[0] != (geom.BlockPos{X: 1}) {
		t.Fatalf("Unknown = %+v, want exactly the cell at x=1", got.Unknown)
	}
}

func TestGatherVisitsCellsInAFixedOrder(t *testing.T) {
	blocks := world.NewBlocks()

	region := geom.AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1.5, MaxY: 1.5, MaxZ: 0.5}
	first, err := Gather(blocks, region, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	second, err := Gather(blocks, region, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	if len(first.Unknown) != 4 {
		t.Fatalf("Unknown has %d cells, want 4", len(first.Unknown))
	}
	for index := range first.Unknown {
		if first.Unknown[index] != second.Unknown[index] {
			t.Fatalf("Gather is not deterministic at index %d: %+v vs %+v",
				index, first.Unknown[index], second.Unknown[index])
		}
	}
	// X outermost, then Y, then Z.
	want := []geom.BlockPos{{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1}}
	for index, pos := range want {
		if first.Unknown[index] != pos {
			t.Fatalf("Unknown[%d] = %+v, want %+v", index, first.Unknown[index], pos)
		}
	}
}

func TestGatherIncludesTheCellAtAFlushMaxEdge(t *testing.T) {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{}, geom.BlockPos{X: 1}, geom.EmptyShape())
	blocks.Set(geom.BlockPos{X: 1}, geom.FullCube())

	// The region's max X lands exactly on the boundary of the solid cell.
	region := geom.AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 0.5, MaxZ: 0.5}
	got, err := Gather(blocks, region, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(got.Boxes) != 1 {
		t.Fatalf("Gather returned %d boxes, want the flush cell to be included", len(got.Boxes))
	}
}

func TestGatherEnforcesTheCandidateLimit(t *testing.T) {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -4, Y: -4, Z: -4}, geom.BlockPos{X: 4, Y: 4, Z: 4}, geom.FullCube())

	region := geom.AABB{MinX: -3, MinY: -3, MinZ: -3, MaxX: 3, MaxY: 3, MaxZ: 3}
	if _, err := Gather(blocks, region, 8); !errors.Is(err, ErrCandidateLimit) {
		t.Fatalf("Gather error = %v, want ErrCandidateLimit", err)
	}
}

func TestGatherOnAnEmptyViewAllocatesNoBoxes(t *testing.T) {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1, Y: -1, Z: -1}, geom.BlockPos{X: 1, Y: 1, Z: 1}, geom.EmptyShape())

	got, err := Gather(blocks, geom.AABB{MaxX: 0.5, MaxY: 0.5, MaxZ: 0.5}, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(got.Boxes) != 0 || len(got.Unknown) != 0 {
		t.Fatalf("Gather = %+v, want nothing", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./collision/`

Expected: FAIL to build, with `undefined: Gather`.

- [ ] **Step 3: Write the implementation**

```go
// Package collision resolves swept axis-aligned motion against the blocks of a
// world view, reproducing Java Edition 1.8.9 exactly.
//
// The package knows nothing about entities, profiles, or the protocol. A
// caller supplies a box, a motion, and a view; it receives the motion that
// actually applied and what the box touched on the way.
package collision

import (
	"errors"
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// ErrCandidateLimit reports that a sweep would visit more cells than its
// deterministic budget allows. The budget exists so that a malformed motion
// cannot make one tick walk the world.
var ErrCandidateLimit = errors.New("collision: candidate limit exhausted")

// Candidates is the outcome of a broad-phase sweep.
type Candidates struct {
	// Boxes are the collision boxes overlapping the region, in visit order.
	Boxes []geom.AABB
	// Unknown holds every cell the view could not answer for, in visit order.
	// A caller that finds this non-empty must abandon the tick rather than
	// treat the region as empty.
	Unknown []geom.BlockPos
}

// Gather collects the collision boxes of every cell the region touches.
//
// Cells are visited X outermost, then Y, then Z, so both returned slices have
// an order that does not depend on map iteration. A non-positive limit means
// no limit.
func Gather(view world.BlockView, region geom.AABB, limit int) (Candidates, error) {
	minPos := geom.BlockPos{
		X: geom.Floor(region.MinX),
		Y: geom.Floor(region.MinY),
		Z: geom.Floor(region.MinZ),
	}
	maxPos := geom.BlockPos{
		X: geom.Floor(region.MaxX),
		Y: geom.Floor(region.MaxY),
		Z: geom.Floor(region.MaxZ),
	}

	var candidates Candidates
	visited := 0
	for x := minPos.X; x <= maxPos.X; x++ {
		for y := minPos.Y; y <= maxPos.Y; y++ {
			for z := minPos.Z; z <= maxPos.Z; z++ {
				visited++
				if limit > 0 && visited > limit {
					return Candidates{}, fmt.Errorf("%w: %d cells", ErrCandidateLimit, visited)
				}

				pos := geom.BlockPos{X: x, Y: y, Z: z}
				shape, lookup := view.CollisionShape(pos)
				switch lookup {
				case world.LookupUnknown:
					candidates.Unknown = append(candidates.Unknown, pos)
				case world.LookupAir:
				case world.LookupShape:
					candidates.Boxes = shape.BoxesAt(pos, candidates.Boxes)
				}
			}
		}
	}

	return candidates, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./collision/ -v`

Expected: PASS, six tests.

- [ ] **Step 5: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [ ] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add collision/candidates.go collision/candidates_test.go
git commit -m "feat(collision): add broad-phase candidate gathering"
```

---

## Task 7: Axis resolution without stepping

**Files:**
- Create: `collision/resolve.go`
- Test: `collision/resolve_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1 to 6.
- Produces:
  - `type Move struct { Body geom.AABB; Motion geom.Vec3; OnGround bool; StepHeight float64; CandidateLimit int }`
  - `type Result struct { Body geom.AABB; Applied geom.Vec3; CollidedX, CollidedY, CollidedZ bool; OnGround bool; Stepped bool; Unknown []geom.BlockPos }`
  - `func (r Result) CollidedHorizontally() bool`
  - `func Resolve(view world.BlockView, move Move) (Result, error)`

This task implements the Y, X, Z passes and the collision flags. Step-up is Task 8; leave `StepHeight` unused here and add it in the next task, so the axis order is proven on its own first.

When `Gather` reports unknown cells, `Resolve` returns a result with `Unknown` populated, the body unmoved, and no applied motion. The caller decides what an incomplete tick means; collision does not guess.

- [ ] **Step 1: Write the failing test**

```go
package collision

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// player returns a box roughly the size of a 1.8.9 player, standing on the
// floor built by floorAt.
func player() geom.AABB {
	return geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}
}

// worldWithFloor returns a view whose y=-1 layer is solid across a wide area
// and whose remaining cells up to y=4 are air.
func worldWithFloor() *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -8, Y: -1, Z: -8}, geom.BlockPos{X: 8, Y: -1, Z: 8}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -8, Y: 0, Z: -8}, geom.BlockPos{X: 8, Y: 4, Z: 8}, geom.EmptyShape())

	return blocks
}

func TestResolveAppliesFreeMotionUntouched(t *testing.T) {
	got, err := Resolve(worldWithFloor(), Move{
		Body:     player().Offset(geom.Vec3{Y: 1}),
		Motion:   geom.Vec3{X: 0.2, Y: 0.1, Z: -0.3},
		OnGround: false,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Applied != (geom.Vec3{X: 0.2, Y: 0.1, Z: -0.3}) {
		t.Fatalf("Applied = %+v, want the motion unchanged", got.Applied)
	}
	if got.CollidedX || got.CollidedY || got.CollidedZ || got.OnGround {
		t.Fatalf("free motion reported a collision: %+v", got)
	}
}

func TestResolveStopsOnTheFloorAndSetsOnGround(t *testing.T) {
	got, err := Resolve(worldWithFloor(), Move{
		Body:   player(),
		Motion: geom.Vec3{Y: -5},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Applied.Y != 0 {
		t.Fatalf("Applied.Y = %v, want 0: the body already rests on the floor", got.Applied.Y)
	}
	if !got.CollidedY || !got.OnGround {
		t.Fatalf("landing did not set the vertical flags: %+v", got)
	}
}

func TestResolveFallsToTheFloorExactly(t *testing.T) {
	got, err := Resolve(worldWithFloor(), Move{
		Body:   player().Offset(geom.Vec3{Y: 2}),
		Motion: geom.Vec3{Y: -5},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Applied.Y != -2 {
		t.Fatalf("Applied.Y = %v, want -2", got.Applied.Y)
	}
	if got.Body.MinY != 0 {
		t.Fatalf("Body.MinY = %v, want the body flush with the floor", got.Body.MinY)
	}
	if !got.OnGround {
		t.Error("OnGround is false after landing")
	}
}

func TestResolveStopsAtAWall(t *testing.T) {
	blocks := worldWithFloor()
	blocks.Fill(geom.BlockPos{X: 2, Y: 0, Z: -8}, geom.BlockPos{X: 2, Y: 4, Z: 8}, geom.FullCube())

	got, err := Resolve(blocks, Move{
		Body:     player(),
		Motion:   geom.Vec3{X: 5},
		OnGround: true,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Applied.X != 1.7 {
		t.Fatalf("Applied.X = %v, want 1.7", got.Applied.X)
	}
	if !got.CollidedX || !got.CollidedHorizontally() {
		t.Fatalf("hitting a wall did not set the horizontal flags: %+v", got)
	}
	if got.CollidedZ {
		t.Error("CollidedZ set for motion that had no Z component")
	}
}

func TestResolveMovesYBeforeXSoTheXPassSeesTheMovedBody(t *testing.T) {
	// A one-block ledge occupying x=1, y=0. The body starts above it and
	// descends while moving forward.
	//
	// Resolving Y first drops the body to y=0, where the ledge blocks X and
	// only 0.7 of the motion applies. Resolving X first would test the body at
	// its old height, where the ledge is below it and nothing blocks, applying
	// the full 1. The two orders give different answers, which is what makes
	// this a real ordering test.
	blocks := worldWithFloor()
	blocks.Fill(geom.BlockPos{X: 1, Y: 0, Z: -1}, geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.FullCube())

	got, err := Resolve(blocks, Move{
		Body:   player().Offset(geom.Vec3{Y: 1.2}),
		Motion: geom.Vec3{X: 1, Y: -1.2},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Applied.Y != -1.2 {
		t.Fatalf("Applied.Y = %v, want the full -1.2", got.Applied.Y)
	}
	if got.Applied.X != 0.7 {
		t.Fatalf("Applied.X = %v, want 0.7; 1 means X resolved before Y", got.Applied.X)
	}
	if !got.CollidedX {
		t.Error("CollidedX not set after the ledge blocked the move")
	}
}

func TestResolveReportsUnknownAndDoesNotMove(t *testing.T) {
	blocks := world.NewBlocks()
	// Nothing described at all: every cell is unknown.
	got, err := Resolve(blocks, Move{
		Body:   player(),
		Motion: geom.Vec3{X: 1},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got.Unknown) == 0 {
		t.Fatal("Resolve did not report unknown cells")
	}
	if !got.Applied.IsZero() {
		t.Fatalf("Applied = %+v, want no motion on an incomplete view", got.Applied)
	}
	if got.Body != player() {
		t.Fatalf("Body = %+v, want the body unmoved", got.Body)
	}
}

func TestResolveZeroMotionIsAFixedPoint(t *testing.T) {
	body := player()
	got, err := Resolve(worldWithFloor(), Move{Body: body, OnGround: true})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Body != body {
		t.Fatalf("Body = %+v, want %+v", got.Body, body)
	}
	if !got.Applied.IsZero() {
		t.Fatalf("Applied = %+v, want zero", got.Applied)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./collision/ -run TestResolve`

Expected: FAIL to build, with `undefined: Resolve`.

- [ ] **Step 3: Write the implementation**

```go
package collision

import "github.com/go-theft-craft/minecraft-simulation/geom"
import "github.com/go-theft-craft/minecraft-simulation/world"

// Move is one entity's intended motion for one tick.
type Move struct {
	// Body is the entity's box before the move.
	Body geom.AABB
	// Motion is the intended displacement.
	Motion geom.Vec3
	// OnGround is the entity's standing state entering the tick. It is one of
	// the two conditions that allow a step-up.
	OnGround bool
	// StepHeight is how far the entity may rise to clear an obstacle. Zero
	// disables stepping.
	StepHeight float64
	// CandidateLimit bounds how many cells the sweep may visit. Zero means no
	// limit.
	CandidateLimit int
}

// Result is the outcome of a move.
type Result struct {
	// Body is the box after the move.
	Body geom.AABB
	// Applied is the displacement that actually happened.
	Applied geom.Vec3
	// CollidedX, CollidedY, and CollidedZ report which axes were clamped.
	CollidedX bool
	CollidedY bool
	CollidedZ bool
	// OnGround is true when downward motion was clamped.
	OnGround bool
	// Stepped is true when the body rose over an obstacle.
	Stepped bool
	// Unknown holds the cells the view could not answer for. When it is
	// non-empty the move did not happen and every other field is the input
	// state.
	Unknown []geom.BlockPos
}

// CollidedHorizontally reports whether either horizontal axis was clamped.
func (r Result) CollidedHorizontally() bool {
	return r.CollidedX || r.CollidedZ
}

// Resolve applies move against view one axis at a time.
//
// The axis order is Y, then X, then Z, and the body is translated after each
// axis, so the X pass tests a body that has already moved vertically. That
// order is vanilla's and is observable: it decides whether an entity rising
// into a ledge is stopped by the ledge or slides under it.
func Resolve(view world.BlockView, move Move) (Result, error) {
	candidates, err := Gather(view, move.Body.Stretch(move.Motion), move.CandidateLimit)
	if err != nil {
		return Result{}, err
	}
	if len(candidates.Unknown) != 0 {
		return Result{Body: move.Body, Unknown: candidates.Unknown}, nil
	}

	body := move.Body
	applied := move.Motion

	for _, box := range candidates.Boxes {
		applied.Y = box.ClampY(body, applied.Y)
	}
	body = body.Offset(geom.Vec3{Y: applied.Y})

	for _, box := range candidates.Boxes {
		applied.X = box.ClampX(body, applied.X)
	}
	body = body.Offset(geom.Vec3{X: applied.X})

	for _, box := range candidates.Boxes {
		applied.Z = box.ClampZ(body, applied.Z)
	}
	body = body.Offset(geom.Vec3{Z: applied.Z})

	return Result{
		Body:      body,
		Applied:   applied,
		CollidedX: applied.X != move.Motion.X,
		CollidedY: applied.Y != move.Motion.Y,
		CollidedZ: applied.Z != move.Motion.Z,
		OnGround:  move.Motion.Y < 0 && applied.Y != move.Motion.Y,
	}, nil
}
```

Note the import style: two separate single-import lines will be merged into one block by `gofumpt`. Run `task fmt` before reading the file back.

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./collision/ -v -run TestResolve`

Expected: PASS, seven tests.

- [ ] **Step 5: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [ ] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add collision/resolve.go collision/resolve_test.go
git commit -m "feat(collision): resolve motion one axis at a time"
```

---

## Task 8: Step-up

**Files:**
- Modify: `collision/resolve.go`
- Test: `collision/resolve_test.go`

**Interfaces:**
- Consumes: `Move`, `Result`, `Resolve` from Task 7.
- Produces: no new exported names. `Move.StepHeight` becomes meaningful and `Result.Stepped` becomes reachable.

Step-up runs when all three hold: `StepHeight` is positive; the entity was on the ground entering the tick or its downward Y motion was clamped; and X or Z was clamped. It gathers a second candidate set from the *pre-move* body stretched by the horizontal motion and the step height.

Both candidate outcomes start from the pre-move body and run Y, then X, then Z, with the step height as the Y motion. They differ in one thing only: **which box the Y clamp tests.**

- The first tests the body stretched horizontally by the motion, so it rises for a block it is about to move over.
- The second tests the plain body, so it rises only for what is directly above it.

The winner is whichever travels further horizontally, comparing `x*x + z*z`; a tie goes to the second. The winner then settles downward onto whatever it climbed. Only after settling is it compared against the unstepped result: if the unstepped horizontal distance is greater than or equal to the stepped one, the unstepped result stands.

Both candidates are needed because neither wins in every geometry, and the order of the comparison matters: settling first and comparing second is what lets a step that gained height but no ground lose to the plain move.

- [ ] **Step 1: Write the failing test**

```go
// slab is a half-height block. A 0.6 step height clears one of these and
// cannot clear a full cube, which is exactly vanilla's behaviour: a player
// walks onto a slab and has to jump onto a full block.
func slab() geom.Shape {
	return geom.NewShape(geom.AABB{MaxX: 1, MaxY: 0.5, MaxZ: 1})
}

func TestResolveStepsOntoASlab(t *testing.T) {
	blocks := worldWithFloor()
	blocks.Set(geom.BlockPos{X: 1, Y: 0, Z: 0}, slab())
	blocks.Set(geom.BlockPos{X: 1, Y: 0, Z: -1}, slab())

	got, err := Resolve(blocks, Move{
		Body:       player(),
		Motion:     geom.Vec3{X: 1},
		OnGround:   true,
		StepHeight: 0.6,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !got.Stepped {
		t.Fatalf("Resolve did not step: %+v", got)
	}
	if got.Body.MinY != 0.5 {
		t.Fatalf("Body.MinY = %v, want 0.5: the body should stand on the slab", got.Body.MinY)
	}
	if got.Applied.X != 1 {
		t.Fatalf("Applied.X = %v, want the full motion after stepping", got.Applied.X)
	}
	if !got.OnGround {
		t.Error("OnGround is false after stepping onto a surface")
	}
}

func TestResolveCannotStepOntoAFullBlock(t *testing.T) {
	blocks := worldWithFloor()
	blocks.Set(geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.FullCube())
	blocks.Set(geom.BlockPos{X: 1, Y: 0, Z: -1}, geom.FullCube())

	got, err := Resolve(blocks, Move{
		Body:       player(),
		Motion:     geom.Vec3{X: 1},
		OnGround:   true,
		StepHeight: 0.6,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Stepped {
		t.Fatalf("Resolve stepped onto a full block with a 0.6 step height: %+v", got)
	}
	if got.Applied.X != 0.7 {
		t.Fatalf("Applied.X = %v, want 0.7", got.Applied.X)
	}
}

func TestResolveDoesNotStepAboveTheStepHeight(t *testing.T) {
	blocks := worldWithFloor()
	// A two-block wall: 0.6 of step height cannot clear it.
	blocks.Fill(geom.BlockPos{X: 1, Y: 0, Z: -1}, geom.BlockPos{X: 1, Y: 1, Z: 0}, geom.FullCube())

	got, err := Resolve(blocks, Move{
		Body:       player(),
		Motion:     geom.Vec3{X: 1},
		OnGround:   true,
		StepHeight: 0.6,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Stepped {
		t.Fatalf("Resolve stepped over a two-block wall: %+v", got)
	}
	if got.Body.MinY != 0 {
		t.Fatalf("Body.MinY = %v, want the body still on the floor", got.Body.MinY)
	}
	if got.Applied.X != 0.7 {
		t.Fatalf("Applied.X = %v, want 0.7", got.Applied.X)
	}
}

func TestResolveDoesNotStepWhileAirborne(t *testing.T) {
	blocks := worldWithFloor()
	blocks.Set(geom.BlockPos{X: 1, Y: 1, Z: 0}, geom.FullCube())
	blocks.Set(geom.BlockPos{X: 1, Y: 1, Z: -1}, geom.FullCube())

	got, err := Resolve(blocks, Move{
		Body:       player().Offset(geom.Vec3{Y: 1}),
		Motion:     geom.Vec3{X: 1, Y: 0.2},
		OnGround:   false,
		StepHeight: 0.6,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Stepped {
		t.Fatalf("Resolve stepped while rising through the air: %+v", got)
	}
}

func TestResolveDoesNotStepWithoutAHorizontalCollision(t *testing.T) {
	got, err := Resolve(worldWithFloor(), Move{
		Body:       player(),
		Motion:     geom.Vec3{X: 0.2},
		OnGround:   true,
		StepHeight: 0.6,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Stepped {
		t.Fatalf("Resolve stepped with nothing in the way: %+v", got)
	}
	if got.Applied.X != 0.2 {
		t.Fatalf("Applied.X = %v, want 0.2", got.Applied.X)
	}
}

func TestResolveStepHeightZeroNeverSteps(t *testing.T) {
	// A slab the entity could clear if it had any step height at all.
	blocks := worldWithFloor()
	blocks.Set(geom.BlockPos{X: 1, Y: 0, Z: 0}, slab())
	blocks.Set(geom.BlockPos{X: 1, Y: 0, Z: -1}, slab())

	got, err := Resolve(blocks, Move{
		Body:     player(),
		Motion:   geom.Vec3{X: 1},
		OnGround: true,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Stepped {
		t.Fatalf("Resolve stepped with a zero step height: %+v", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./collision/ -run TestResolveStep`

Expected: FAIL. `TestResolveStepsOntoASlab` reports `Resolve did not step`, because `StepHeight` is still unused.

- [ ] **Step 3: Extract the axis passes**

Replace the three inline loops in `Resolve` with a helper, so the step-up path can run the same passes on a different body. Add to `collision/resolve.go`:

```go
// applyAxes runs the Y, X, Z passes against boxes and returns the moved body
// with the motion that survived.
//
// yProbe is the box the Y clamp tests. An ordinary move passes the body
// itself; step-up passes a horizontally stretched body for one of its two
// attempts, which is the only way the two differ.
func applyAxes(boxes []geom.AABB, body, yProbe geom.AABB, motion geom.Vec3) (geom.AABB, geom.Vec3) {
	for _, box := range boxes {
		motion.Y = box.ClampY(yProbe, motion.Y)
	}
	body = body.Offset(geom.Vec3{Y: motion.Y})

	for _, box := range boxes {
		motion.X = box.ClampX(body, motion.X)
	}
	body = body.Offset(geom.Vec3{X: motion.X})

	for _, box := range boxes {
		motion.Z = box.ClampZ(body, motion.Z)
	}
	body = body.Offset(geom.Vec3{Z: motion.Z})

	return body, motion
}
```

and rewrite the middle of `Resolve` to use it:

```go
	body, applied := applyAxes(candidates.Boxes, move.Body, move.Body, move.Motion)
```

- [ ] **Step 4: Run the tests to confirm the extraction changed nothing**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./collision/ -run TestResolve`

Expected: the Task 7 tests still pass; the five step tests still fail.

- [ ] **Step 5: Implement stepping**

Add to `collision/resolve.go`:

```go
// stepUp retries a blocked horizontal move by rising up to height.
//
// Both attempts start from the pre-move body and rise by the full step height.
// They differ only in the box their Y clamp tests: the first tests a body
// stretched horizontally by the motion, so it rises for a block it is about to
// move over; the second tests the plain body, so it rises only for what is
// directly above. Neither wins in every geometry, so vanilla computes both and
// keeps the one that travels further horizontally. A tie goes to the second.
//
// The winner settles downward onto whatever it climbed, and only then is it
// weighed against the unstepped result. The bool reports whether it won.
func stepUp(boxes []geom.AABB, body geom.AABB, motion geom.Vec3, height float64, unstepped geom.Vec3) (geom.AABB, geom.Vec3, bool) {
	horizontal := geom.Vec3{X: motion.X, Z: motion.Z}

	rise := geom.Vec3{X: motion.X, Y: height, Z: motion.Z}
	firstBody, firstMotion := applyAxes(boxes, body, body.Stretch(horizontal), rise)
	secondBody, secondMotion := applyAxes(boxes, body, body, rise)

	winnerBody, winnerMotion := secondBody, secondMotion
	if firstMotion.HorizontalLengthSquared() > secondMotion.HorizontalLengthSquared() {
		winnerBody, winnerMotion = firstBody, firstMotion
	}

	// Settle onto whatever was climbed before judging the attempt: a rise that
	// gained height but no ground must lose to the plain move.
	settle := -winnerMotion.Y
	for _, box := range boxes {
		settle = box.ClampY(winnerBody, settle)
	}
	winnerBody = winnerBody.Offset(geom.Vec3{Y: settle})
	winnerMotion.Y += settle

	if unstepped.HorizontalLengthSquared() >= winnerMotion.HorizontalLengthSquared() {
		return body.Offset(unstepped), unstepped, false
	}

	return winnerBody, winnerMotion, true
}
```

Then insert the step decision into `Resolve`, immediately after the `applyAxes` call and before the `Result` is built:

```go
	stepped := false
	verticallyBlocked := move.Motion.Y < 0 && applied.Y != move.Motion.Y
	horizontallyBlocked := applied.X != move.Motion.X || applied.Z != move.Motion.Z
	if move.StepHeight > 0 && (move.OnGround || verticallyBlocked) && horizontallyBlocked {
		stepBoxes, err := Gather(
			view,
			move.Body.Stretch(geom.Vec3{X: move.Motion.X, Y: move.StepHeight, Z: move.Motion.Z}),
			move.CandidateLimit,
		)
		if err != nil {
			return Result{}, err
		}
		if len(stepBoxes.Unknown) != 0 {
			return Result{Body: move.Body, Unknown: stepBoxes.Unknown}, nil
		}

		body, applied, stepped = stepUp(stepBoxes.Boxes, move.Body, move.Motion, move.StepHeight, applied)
	}
```

and set the flags from the final values:

```go
	return Result{
		Body:      body,
		Applied:   applied,
		CollidedX: applied.X != move.Motion.X,
		CollidedY: applied.Y != move.Motion.Y,
		CollidedZ: applied.Z != move.Motion.Z,
		OnGround:  move.Motion.Y < 0 && applied.Y != move.Motion.Y || stepped,
		Stepped:   stepped,
	}, nil
```

An entity that stepped is standing on the surface it climbed, so `OnGround` is true even though its Y motion was not downward.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./collision/ -v`

Expected: PASS, thirteen tests.

- [ ] **Step 7: Run the repository checks**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

- [ ] **Step 8: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add collision/resolve.go collision/resolve_test.go
git commit -m "feat(collision): add step-up over low obstacles"
```

---

## Task 9: Property tests and the milestone record

**Files:**
- Create: `collision/property_test.go`
- Modify: `README.md`
- Modify: `CHANGELOG.md` (create it if the repository has none, using the header shown below)

**Interfaces:**
- Consumes: everything above.
- Produces: no code.

These three properties are M8.2's exit criterion. They are written as randomized tests over a fixed seed list rather than as `testing/quick` calls, so a failure names the exact case and reruns identically on any machine. No test may call `math/rand` without an explicit seed: a nondeterministic failure in a determinism project is worse than no test.

- [ ] **Step 1: Write the property tests**

```go
package collision

import (
	"math/rand/v2"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// seeds are fixed so that a failure reproduces exactly. Add to this list
// rather than randomizing it.
var seeds = []uint64{1, 2, 3, 5, 8, 13, 21, 34, 55, 89}

// solidWorld returns a view whose every cell in the working area is a full
// cube except a hollow corridor the body starts inside.
func solidWorld() *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -6, Y: -6, Z: -6}, geom.BlockPos{X: 6, Y: 6, Z: 6}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -1, Y: 0, Z: -1}, geom.BlockPos{X: 1, Y: 2, Z: 1}, geom.EmptyShape())

	return blocks
}

// TestSweptMotionNeverTunnels is the first exit property: however large the
// motion, the resolved body never ends up overlapping a solid block.
func TestSweptMotionNeverTunnels(t *testing.T) {
	blocks := solidWorld()
	body := geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}

	for _, seed := range seeds {
		random := rand.New(rand.NewPCG(seed, 0))
		for step := 0; step < 100; step++ {
			// Kept inside the described region: a sweep that leaves it
			// reports unknown cells and asserts nothing, so a wider range
			// would just buy slower tests that check less.
			motion := geom.Vec3{
				X: (random.Float64() - 0.5) * 8,
				Y: (random.Float64() - 0.5) * 8,
				Z: (random.Float64() - 0.5) * 8,
			}

			got, err := Resolve(blocks, Move{Body: body, Motion: motion, StepHeight: 0.6})
			if err != nil {
				t.Fatalf("seed %d step %d: Resolve: %v", seed, step, err)
			}
			if len(got.Unknown) != 0 {
				continue // The sweep left the described area; nothing to assert.
			}

			candidates, err := Gather(blocks, got.Body, 0)
			if err != nil {
				t.Fatalf("seed %d step %d: Gather: %v", seed, step, err)
			}
			for _, box := range candidates.Boxes {
				if box.Intersects(got.Body) {
					t.Fatalf("seed %d step %d: motion %+v tunnelled into %+v; body ended at %+v",
						seed, step, motion, box, got.Body)
				}
			}
		}
	}
}

// TestStepUpRespectsItsBound is the second exit property: a body never rises
// by more than its step height in a single resolve.
func TestStepUpRespectsItsBound(t *testing.T) {
	const stepHeight = 0.6

	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -6, Y: -1, Z: -6}, geom.BlockPos{X: 6, Y: -1, Z: 6}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -6, Y: 0, Z: -6}, geom.BlockPos{X: 6, Y: 6, Z: 6}, geom.EmptyShape())
	// Slabs along +X, which a 0.6 step height can actually climb. A staircase
	// of full cubes would prove nothing here: the entity could never step at
	// all, and the bound would hold trivially.
	blocks.Fill(geom.BlockPos{X: 1, Y: 0, Z: -6}, geom.BlockPos{X: 4, Y: 0, Z: 6}, slab())

	body := geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}
	for _, seed := range seeds {
		random := rand.New(rand.NewPCG(seed, 0))
		for step := 0; step < 100; step++ {
			motion := geom.Vec3{
				X: random.Float64() * 2,
				Y: (random.Float64() - 0.75) * 2,
				Z: (random.Float64() - 0.5) * 2,
			}

			got, err := Resolve(blocks, Move{
				Body:       body,
				Motion:     motion,
				OnGround:   true,
				StepHeight: stepHeight,
			})
			if err != nil {
				t.Fatalf("seed %d step %d: Resolve: %v", seed, step, err)
			}
			if len(got.Unknown) != 0 {
				continue
			}
			if rise := got.Body.MinY - body.MinY; rise > stepHeight {
				t.Fatalf("seed %d step %d: rose %v with a step height of %v, motion %+v",
					seed, step, rise, stepHeight, motion)
			}
		}
	}
}

// TestZeroMotionIsAFixedPoint is the third exit property: resolving no motion
// changes nothing, from any starting position, however many times it runs.
func TestZeroMotionIsAFixedPoint(t *testing.T) {
	blocks := solidWorld()

	for _, seed := range seeds {
		random := rand.New(rand.NewPCG(seed, 0))
		for step := 0; step < 100; step++ {
			origin := geom.Vec3{
				X: (random.Float64() - 0.5) * 4,
				Y: random.Float64() * 2,
				Z: (random.Float64() - 0.5) * 4,
			}
			body := geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}.
				Offset(origin)

			got, err := Resolve(blocks, Move{Body: body, OnGround: true, StepHeight: 0.6})
			if err != nil {
				t.Fatalf("seed %d step %d: Resolve: %v", seed, step, err)
			}
			if len(got.Unknown) != 0 {
				continue
			}
			if got.Body != body {
				t.Fatalf("seed %d step %d: zero motion moved the body from %+v to %+v",
					seed, step, body, got.Body)
			}
			if !got.Applied.IsZero() {
				t.Fatalf("seed %d step %d: zero motion applied %+v", seed, step, got.Applied)
			}
			if got.Stepped {
				t.Fatalf("seed %d step %d: zero motion stepped", seed, step)
			}
		}
	}
}

// TestResolveIsDeterministic runs the same move twice and requires identical
// results, including the order of reported unknown cells.
func TestResolveIsDeterministic(t *testing.T) {
	blocks := solidWorld()
	body := geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}
	move := Move{Body: body, Motion: geom.Vec3{X: 3.5, Y: -2.25, Z: 1.75}, StepHeight: 0.6}

	first, err := Resolve(blocks, move)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	second, err := Resolve(blocks, move)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if first.Body != second.Body || first.Applied != second.Applied {
		t.Fatalf("Resolve is not deterministic: %+v vs %+v", first, second)
	}
	if len(first.Unknown) != len(second.Unknown) {
		t.Fatalf("Unknown lengths differ: %d vs %d", len(first.Unknown), len(second.Unknown))
	}
	for index := range first.Unknown {
		if first.Unknown[index] != second.Unknown[index] {
			t.Fatalf("Unknown order differs at %d", index)
		}
	}
}
```

- [ ] **Step 2: Run the property tests**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- go test ./collision/ -v -run 'TestSwept|TestStepUpRespects|TestZeroMotion|TestResolveIsDeterministic'`

Expected: PASS, four tests. A failure here names the seed and step; reproduce it by narrowing `seeds` to that one value.

- [ ] **Step 3: Record the packages in the README**

Add this section to `README.md`, after whatever introduction the file already has:

```markdown
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
```

- [ ] **Step 4: Add the changelog entry**

If `CHANGELOG.md` does not exist, create it with this content. If it exists, add only the `### Added` bullet under `## Unreleased`.

```markdown
# Changelog

This file records notable user-visible changes. It follows [Keep a Changelog](https://keepachangelog.com/en/2.0.0/) and [Semantic Versioning](https://semver.org/).

## Unreleased

### Added

- `geom`, `world`, and `collision`: swept axis-aligned collision reproducing
  Java Edition 1.8.9 axis order and step-up, over a block view that
  distinguishes known air from unknown regions.
```

- [ ] **Step 5: Run the full verification**

Run: `cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation && devbox run -- task fmt && devbox run -- task verify`

Expected: `lint`, `secrets`, `test`, `vuln`, and `build` all pass.

- [ ] **Step 6: Commit**

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
git add collision/property_test.go README.md CHANGELOG.md
git commit -m "test(collision): add the M8.2 exit properties"
```

---

## Definition of done

M8.2 is complete when every statement below is true:

- `geom` provides vectors, block positions with floor semantics, axis-aligned boxes, the three single-axis clamps, and per-block voxel shapes.
- `world.BlockView` distinguishes known air from an unknown region, and `world.Blocks` implements it deterministically.
- `collision.Resolve` applies motion along Y, then X, then Z, translating the body after each axis, and reports which axes were clamped and whether the entity is on the ground.
- A blocked horizontal move retries with a step-up bounded by `Move.StepHeight`, choosing the outcome that travels further in the horizontal plane.
- A move over an unknown region returns the cells it could not resolve and applies no motion.
- Property tests establish that swept motion never tunnels through a solid block, that step-up respects its bound, and that zero motion is a fixed point.
- `devbox run -- task verify` passes.
- No package in this plan imports `minecraft-protocol`, and no decompiled source or game asset is committed.

## Follow-on

M8.3 delivers `sim`, `world` tick contracts, `entity`, `runtime`, and the in-memory store. The design places it after this milestone deliberately: what collision returns, including the contact flags and the step result, should shape the tick contracts rather than the reverse. `Result` is the input to that decision.

M8.4 adds `movement` and the v1_8 player profile. That is where `data.Physics` from M8.1 first meets this package: the profile turns block slipperiness and the motion constants into a motion vector, and this package decides where the body actually ends up. The `float32` product noted in M8.1's motion notes belongs to the profile, not here.
