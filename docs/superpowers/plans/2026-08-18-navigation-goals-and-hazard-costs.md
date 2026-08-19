# Navigation Goals and Hazard Costs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Status: complete, 2026-08-19.** All six tasks done and `task verify` is
> green. Two deviations, each recorded in the commit that made it: Tasks 1 and
> 2 are one commit, because `euclidean` has no caller until the goal shapes
> that use it exist and the lint gate rejects an unused function; and
> `memoOracle` answers `hazardous` uncached rather than growing a fifth cache,
> because the package's stated rule is that a cached answer is one that costs a
> collision sweep and `HazardAt` is a single block lookup. Task 6's consumer
> note stands open: `headless-minecraft` wraps its `Find` call in `GoalBlock`
> when it bumps `go.mod` past the next release.

**Goal:** Replace the exact-`BlockPos` goal in `navigation.Find` and `Planner.Plan` with a `Goal` interface (heuristic + completion test), ship the eight goal shapes the Baritone adoption design names, and add an opt-in soft cost for edges arriving beside a hazard.

**Architecture:** A `Goal` answers two questions — a lower bound in blocks on how far a position is from satisfying it, and whether a position satisfies it. The search scales blocks into ticks with the capability's existing `perBlockFloor()`, so goals stay capability-free and admissibility is preserved. The hazard penalty is a new `Capability` field, zero by default, applied in `expand()` to any edge whose destination has a hazardous horizontal neighbour; the oracle grows one cached question to answer it.

**Tech Stack:** Go, Devbox, Task. Repository: `minecraft-simulation`, package `navigation` (plus one call site in `navigation/reach`).

**Parent spec:** `headless-minecraft/docs/superpowers/specs/2026-08-18-baritone-adoption-design.md`, stage 1.

## Global Constraints

- Work in `/home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation`.
- Run every command as `devbox run -- task <name>`. Scope test runs to the touched package (`devbox run -- task test -- ./navigation/...`); run full `devbox run -- task verify` only before the final commit.
- Commit directly to `main`. Never branch. Never add `Co-Authored-By` or `Claude-Session` lines to a commit message.
- **Do not start until the navigation pillar's pending release tags have landed** — this plan changes `navigation`'s public surface, and the spec orders the tags first.
- `EdgeKind`, `Posture`, and `Reason` values are appended, never inserted (recorded paths carry the numbers). This plan adds none of them.
- Nothing in `navigation` imports `sim`. Goals import only `geom` and `math`.
- Every cost is in ticks; goals answer in blocks and only `Capability.toward` converts.
- With `HazardPenalty` zero (the default), every search must return byte-identical results to today — the determinism gate (`devbox run -- task determinism`) enforces this.

---

### Task 1: Goal interface and GoalBlock

**Files:**
- Create: `navigation/goal.go`
- Create: `navigation/goal_test.go`

**Interfaces:**
- Consumes: `geom.BlockPos` (fields `X, Y, Z int32`).
- Produces: `type Goal interface { Heuristic(pos geom.BlockPos) float64; Reached(pos geom.BlockPos) bool }`, `type GoalBlock struct { Pos geom.BlockPos }`, and unexported helpers `manhattan(a, b geom.BlockPos) float64` and `euclidean(a, b geom.BlockPos) float64`. Tasks 2–5 rely on these exact names.

- [x] **Step 1: Write the failing test**

```go
// navigation/goal_test.go
package navigation

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestGoalBlockIsReachedAtExactlyItsCell(t *testing.T) {
	goal := GoalBlock{Pos: geom.BlockPos{X: 3, Y: 64, Z: -2}}

	if !goal.Reached(geom.BlockPos{X: 3, Y: 64, Z: -2}) {
		t.Fatal("Reached refused the goal cell itself")
	}
	if goal.Reached(geom.BlockPos{X: 3, Y: 65, Z: -2}) {
		t.Fatal("Reached accepted a cell one block above")
	}
}

func TestGoalBlockHeuristicIsManhattanBlocks(t *testing.T) {
	goal := GoalBlock{Pos: geom.BlockPos{X: 3, Y: 64, Z: -2}}

	if got := goal.Heuristic(geom.BlockPos{X: 0, Y: 64, Z: 0}); got != 5 {
		t.Fatalf("Heuristic = %v, want 5 (|3-0| + |64-64| + |-2-0|)", got)
	}
	if got := goal.Heuristic(goal.Pos); got != 0 {
		t.Fatalf("Heuristic at the goal = %v, want 0", got)
	}
}
```

- [x] **Step 2: Run the test to verify it fails**

Run: `devbox run -- task test -- -run 'TestGoalBlock' ./navigation/`
Expected: FAIL — `undefined: GoalBlock`.

- [x] **Step 3: Write the implementation**

```go
// navigation/goal.go
package navigation

import (
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// Goal is what a search is trying to reach.
//
// Heuristic answers in blocks, not ticks: a lower bound on how far pos is
// from satisfying the goal. The search scales blocks into ticks with the
// capability's cheapest per-block cost, which is what lets one goal serve
// every body. An overestimate makes the search return routes that are not
// shortest, so a goal that cannot bound tightly answers small rather than
// clever.
//
// Reached reports that pos satisfies the goal. The search stops at the first
// expanded node it holds for.
type Goal interface {
	Heuristic(pos geom.BlockPos) float64
	Reached(pos geom.BlockPos) bool
}

// manhattan is the block distance the search's own former heuristic used:
// admissible for a search whose steps change one coordinate at a time.
func manhattan(a, b geom.BlockPos) float64 {
	return math.Abs(float64(a.X-b.X)) +
		math.Abs(float64(a.Y-b.Y)) +
		math.Abs(float64(a.Z-b.Z))
}

// euclidean never exceeds manhattan, so a goal built on it stays admissible.
func euclidean(a, b geom.BlockPos) float64 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	dz := float64(a.Z - b.Z)

	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// GoalBlock is the goal every search had before goals existed: one exact
// cell, reached there and nowhere else.
type GoalBlock struct {
	Pos geom.BlockPos
}

// Heuristic implements Goal.
func (g GoalBlock) Heuristic(pos geom.BlockPos) float64 { return manhattan(pos, g.Pos) }

// Reached implements Goal.
func (g GoalBlock) Reached(pos geom.BlockPos) bool { return pos == g.Pos }
```

- [x] **Step 4: Run the test to verify it passes**

Run: `devbox run -- task test -- -run 'TestGoalBlock' ./navigation/`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add navigation/goal.go navigation/goal_test.go
git commit -m "feat(navigation): a goal is a heuristic and a completion test"
```

---

### Task 2: The remaining goal shapes

**Files:**
- Modify: `navigation/goal.go` (append)
- Modify: `navigation/goal_test.go` (append)

**Interfaces:**
- Consumes: `Goal`, `manhattan`, `euclidean` from Task 1.
- Produces: `GoalXZ{X, Z int32}`, `GoalYLevel{Y int32}`, `GoalNear{Pos geom.BlockPos; Radius float64}`, `GoalGetToBlock{Pos geom.BlockPos}`, `GoalRunAway{From []geom.BlockPos; Distance float64}`, `GoalComposite []Goal`, `GoalInverted{Goal Goal}`, `GoalAxis{Y int32}` — all satisfying `Goal`.

- [x] **Step 1: Write the failing tests**

Append to `navigation/goal_test.go`:

```go
func TestGoalXZIgnoresHeight(t *testing.T) {
	goal := GoalXZ{X: 10, Z: -4}

	if !goal.Reached(geom.BlockPos{X: 10, Y: 200, Z: -4}) {
		t.Fatal("Reached refused the column at another height")
	}
	if got := goal.Heuristic(geom.BlockPos{X: 0, Y: 63, Z: 0}); got != 14 {
		t.Fatalf("Heuristic = %v, want 14 (|10| + |-4|, no Y term)", got)
	}
}

func TestGoalYLevelIgnoresColumn(t *testing.T) {
	goal := GoalYLevel{Y: 12}

	if !goal.Reached(geom.BlockPos{X: -100, Y: 12, Z: 40}) {
		t.Fatal("Reached refused the level in another column")
	}
	if got := goal.Heuristic(geom.BlockPos{X: 0, Y: 64, Z: 0}); got != 52 {
		t.Fatalf("Heuristic = %v, want 52", got)
	}
}

func TestGoalNearAcceptsTheRadius(t *testing.T) {
	goal := GoalNear{Pos: geom.BlockPos{}, Radius: 3}

	if !goal.Reached(geom.BlockPos{X: 3}) {
		t.Fatal("Reached refused a cell exactly on the radius")
	}
	if goal.Reached(geom.BlockPos{X: 3, Z: 1}) {
		t.Fatal("Reached accepted a cell beyond the radius")
	}
	if got := goal.Heuristic(geom.BlockPos{X: 10}); got != 7 {
		t.Fatalf("Heuristic = %v, want 7 (10 - radius 3)", got)
	}
	if got := goal.Heuristic(geom.BlockPos{X: 1}); got != 0 {
		t.Fatalf("Heuristic inside the radius = %v, want 0", got)
	}
}

func TestGoalGetToBlockIsReachedBesideNotInside(t *testing.T) {
	goal := GoalGetToBlock{Pos: geom.BlockPos{X: 5, Y: 10, Z: 5}}

	if !goal.Reached(geom.BlockPos{X: 4, Y: 10, Z: 5}) {
		t.Fatal("Reached refused a face neighbour")
	}
	if !goal.Reached(geom.BlockPos{X: 5, Y: 11, Z: 5}) {
		t.Fatal("Reached refused the cell above")
	}
	if goal.Reached(goal.Pos) {
		t.Fatal("Reached accepted the block's own cell")
	}
	if goal.Reached(geom.BlockPos{X: 4, Y: 10, Z: 4}) {
		t.Fatal("Reached accepted a diagonal neighbour")
	}
	if got := goal.Heuristic(geom.BlockPos{X: 5, Y: 10, Z: 8}); got != 2 {
		t.Fatalf("Heuristic = %v, want 2 (3 blocks away, satisfied 1 out)", got)
	}
}

func TestGoalRunAwayIsReachedWhenEverySourceIsFar(t *testing.T) {
	goal := GoalRunAway{
		From:     []geom.BlockPos{{X: 0}, {X: 10}},
		Distance: 5,
	}

	if goal.Reached(geom.BlockPos{X: 4}) {
		t.Fatal("Reached accepted a cell only 4 from the nearer source")
	}
	if !goal.Reached(geom.BlockPos{X: 5}) {
		t.Fatal("Reached refused a cell exactly at Distance from both; the boundary counts, as GoalNear's does")
	}
	if !goal.Reached(geom.BlockPos{X: 20}) {
		t.Fatal("Reached refused a cell far from every source")
	}
	// From X=4: source at 0 is 4 away (needs 1 more), source at 10 is 6 away
	// (satisfied). The bound is the worst violation.
	if got := goal.Heuristic(geom.BlockPos{X: 4}); got != 1 {
		t.Fatalf("Heuristic = %v, want 1", got)
	}
}

func TestGoalCompositeTakesAnyMemberAndTheNearestBound(t *testing.T) {
	goal := GoalComposite{
		GoalBlock{Pos: geom.BlockPos{X: 10}},
		GoalBlock{Pos: geom.BlockPos{X: -2}},
	}

	if !goal.Reached(geom.BlockPos{X: -2}) {
		t.Fatal("Reached refused a member's cell")
	}
	if got := goal.Heuristic(geom.BlockPos{}); got != 2 {
		t.Fatalf("Heuristic = %v, want 2 (the nearer member)", got)
	}
}

func TestGoalInvertedIsNeverReached(t *testing.T) {
	goal := GoalInverted{Goal: GoalBlock{Pos: geom.BlockPos{}}}

	if goal.Reached(geom.BlockPos{X: 1000}) {
		t.Fatal("an inverted goal must never be reached")
	}
	if got := goal.Heuristic(geom.BlockPos{X: 4}); got != -4 {
		t.Fatalf("Heuristic = %v, want -4 (the inner bound, negated)", got)
	}
}

func TestGoalAxisMeasuresTheNearestAxisOrDiagonal(t *testing.T) {
	goal := GoalAxis{Y: 64}

	if !goal.Reached(geom.BlockPos{X: 7, Y: 64, Z: 7}) {
		t.Fatal("Reached refused the x=z diagonal at the right height")
	}
	if goal.Reached(geom.BlockPos{X: 7, Y: 63, Z: 7}) {
		t.Fatal("Reached accepted the diagonal at the wrong height")
	}
	// (6, 64, 8): the x=z diagonal is |6-8| = 2 coordinate changes away,
	// nearer than the x axis (8) or the z axis (6).
	if got := goal.Heuristic(geom.BlockPos{X: 6, Y: 64, Z: 8}); got != 2 {
		t.Fatalf("Heuristic = %v, want 2", got)
	}
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `devbox run -- task test -- -run 'TestGoal' ./navigation/`
Expected: FAIL — `undefined: GoalXZ` (and the rest).

- [x] **Step 3: Write the implementation**

Append to `navigation/goal.go`:

```go
// GoalXZ is a column: reached at any height. It is the long-distance travel
// goal — a caller crossing the world does not know the terrain height where
// it is going.
type GoalXZ struct {
	X, Z int32
}

// Heuristic implements Goal.
func (g GoalXZ) Heuristic(pos geom.BlockPos) float64 {
	return math.Abs(float64(pos.X-g.X)) + math.Abs(float64(pos.Z-g.Z))
}

// Reached implements Goal.
func (g GoalXZ) Reached(pos geom.BlockPos) bool { return pos.X == g.X && pos.Z == g.Z }

// GoalYLevel is a height: reached in any column. A body that wants the
// surface, or a mining level, states the height and nothing else.
type GoalYLevel struct {
	Y int32
}

// Heuristic implements Goal.
func (g GoalYLevel) Heuristic(pos geom.BlockPos) float64 {
	return math.Abs(float64(pos.Y - g.Y))
}

// Reached implements Goal.
func (g GoalYLevel) Reached(pos geom.BlockPos) bool { return pos.Y == g.Y }

// GoalNear is a sphere around a cell: close enough, by straight-line
// distance. It is the follower's goal — standing inside the radius is the
// point, and which cell inside does not matter.
type GoalNear struct {
	Pos    geom.BlockPos
	Radius float64
}

// Heuristic implements Goal. Straight-line distance never exceeds the block
// steps a route needs, so the bound is admissible.
func (g GoalNear) Heuristic(pos geom.BlockPos) float64 {
	return math.Max(0, euclidean(pos, g.Pos)-g.Radius)
}

// Reached implements Goal.
func (g GoalNear) Reached(pos geom.BlockPos) bool {
	return euclidean(pos, g.Pos) <= g.Radius
}

// GoalGetToBlock is a cell's six face neighbours: beside it, above it, or
// below it, never inside it. It is the goal for working a block — digging
// it, opening it, reading it — where standing in its cell is exactly wrong.
type GoalGetToBlock struct {
	Pos geom.BlockPos
}

// Heuristic implements Goal.
func (g GoalGetToBlock) Heuristic(pos geom.BlockPos) float64 {
	return math.Max(0, manhattan(pos, g.Pos)-1)
}

// Reached implements Goal.
func (g GoalGetToBlock) Reached(pos geom.BlockPos) bool {
	return manhattan(pos, g.Pos) == 1
}

// GoalRunAway is distance from every source at once: reached when the
// nearest one is at least Distance away. Flee is this goal with the threats
// as sources.
type GoalRunAway struct {
	From     []geom.BlockPos
	Distance float64
}

// Heuristic implements Goal. Each source demands the body make up what that
// source still lacks; the worst violation is a floor on the travel left.
func (g GoalRunAway) Heuristic(pos geom.BlockPos) float64 {
	var worst float64
	for _, source := range g.From {
		if need := g.Distance - euclidean(pos, source); need > worst {
			worst = need
		}
	}

	return worst
}

// Reached implements Goal. No sources means nothing to escape.
func (g GoalRunAway) Reached(pos geom.BlockPos) bool {
	for _, source := range g.From {
		if euclidean(pos, source) < g.Distance {
			return false
		}
	}

	return true
}

// GoalComposite is any of its members: reached when one is, guided by the
// nearest. A miner with six known veins states all six and takes whichever
// the terrain favours. An empty composite is never reached and guides
// nothing, so a search holding one runs to its budget.
type GoalComposite []Goal

// Heuristic implements Goal.
func (g GoalComposite) Heuristic(pos geom.BlockPos) float64 {
	if len(g) == 0 {
		return 0
	}
	best := g[0].Heuristic(pos)
	for _, member := range g[1:] {
		if h := member.Heuristic(pos); h < best {
			best = h
		}
	}

	return best
}

// Reached implements Goal.
func (g GoalComposite) Reached(pos geom.BlockPos) bool {
	for _, member := range g {
		if member.Reached(pos) {
			return true
		}
	}

	return false
}

// GoalInverted runs from its inner goal rather than toward it. It is never
// reached, so a search holding it always spends its whole node budget and
// returns the partial path that got farthest; the budget is how a caller
// says how far to flee. A negated bound is not admissible in the shortest-
// path sense, and does not need to be: there is no shortest path away.
type GoalInverted struct {
	Goal Goal
}

// Heuristic implements Goal.
func (g GoalInverted) Heuristic(pos geom.BlockPos) float64 {
	return -g.Goal.Heuristic(pos)
}

// Reached implements Goal.
func (g GoalInverted) Reached(geom.BlockPos) bool { return false }

// GoalAxis is the nearest of the four lines x=0, z=0, x=z, and x=-z, at one
// height. Each step changes one coordinate by one, so reaching x=z costs
// |x-z| changes and reaching x=-z costs |x+z|.
type GoalAxis struct {
	Y int32
}

// Heuristic implements Goal.
func (g GoalAxis) Heuristic(pos geom.BlockPos) float64 {
	x := math.Abs(float64(pos.X))
	z := math.Abs(float64(pos.Z))
	toAxis := math.Min(x, z)
	toDiagonal := math.Min(
		math.Abs(float64(pos.X-pos.Z)),
		math.Abs(float64(pos.X+pos.Z)),
	)

	return math.Min(toAxis, toDiagonal) + math.Abs(float64(pos.Y-g.Y))
}

// Reached implements Goal.
func (g GoalAxis) Reached(pos geom.BlockPos) bool {
	if pos.Y != g.Y {
		return false
	}

	return pos.X == 0 || pos.Z == 0 || pos.X == pos.Z || pos.X == -pos.Z
}
```

- [x] **Step 4: Run the tests to verify they pass**

Run: `devbox run -- task test -- -run 'TestGoal' ./navigation/`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add navigation/goal.go navigation/goal_test.go
git commit -m "feat(navigation): the eight goal shapes the adoption design names"
```

---

### Task 3: The search takes a Goal

**Files:**
- Modify: `navigation/search.go` (`Find` ~line 51, `plan` ~line 82, `search` ~line 131, `heuristic` ~line 259)
- Modify: `navigation/planner.go` (`Plan`, line 64)
- Modify: `navigation/navigation.go` (only if `heuristic` docs reference lands there; otherwise untouched)
- Modify: every `_test.go` in `navigation/` and `navigation/reach/reach_test.go` (mechanical rewrite; ~63 `Find` and ~13 `Plan` call sites)

**Interfaces:**
- Consumes: `Goal`, `GoalBlock` from Task 1.
- Produces: `Find(ctx context.Context, view terrainView, facts terrain.Facts, capability Capability, from geom.BlockPos, goal Goal, budget Budget) (Path, error)`; `(*Planner) Plan(ctx context.Context, from geom.BlockPos, goal Goal, budget Budget) (Path, error)`; `var ErrNoGoal = errors.New("navigation: a search needs a goal")`; unexported `func (c Capability) toward(goal Goal, pos geom.BlockPos) float64`. Tasks 4–5 and every later stage rely on these signatures.

- [x] **Step 1: Write the failing test**

Append to `navigation/goal_test.go` (uses the `flat` world and `walker` capability from `search_test.go`, and `benchBudget`-style bounds):

```go
func TestFindRefusesANilGoal(t *testing.T) {
	blocks := flat(-2, -2, 2, 2)

	_, err := Find(context.Background(), blocks, nil, walker,
		geom.BlockPos{}, nil, Budget{Nodes: 100})
	if !errors.Is(err, ErrNoGoal) {
		t.Fatalf("err = %v, want ErrNoGoal", err)
	}
}

func TestFindReachesAGoalBlockExactlyAsBefore(t *testing.T) {
	blocks := flat(-8, -8, 8, 8)
	goal := geom.BlockPos{X: 4, Y: 0, Z: 0}

	path, err := Find(context.Background(), blocks, nil, walker,
		geom.BlockPos{}, GoalBlock{Pos: goal}, Budget{Nodes: 10_000})
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}
	if end := path.End(geom.BlockPos{}); end != goal {
		t.Fatalf("path ends at %v, want %v", end, goal)
	}
}
```

Add `"context"` and `"errors"` to `goal_test.go` imports.

- [x] **Step 2: Run the test to verify it fails**

Run: `devbox run -- task test -- -run 'TestFind(RefusesANilGoal|ReachesAGoalBlock)' ./navigation/`
Expected: FAIL to compile — `Find` does not accept a `Goal`.

- [x] **Step 3: Change the signatures and the search**

In `navigation/search.go`:

1. Add next to `ErrNoBody`:

```go
// ErrNoGoal reports a nil goal. There is no default destination.
var ErrNoGoal = errors.New("navigation: a search needs a goal")
```

2. `Find`: parameter `from, goal geom.BlockPos` becomes `from geom.BlockPos, goal Goal`; after the `ErrNoBody` check add:

```go
	if goal == nil {
		return Path{}, ErrNoGoal
	}
```

3. `plan`: same parameter change (`from geom.BlockPos, goal Goal`); the body passes `goal` through to both `search` calls unchanged.

4. `search`: same parameter change, then three call-site edits in its body:
   - `open.push(start, capability.heuristic(from, goal))` → `open.push(start, capability.toward(goal, from))`
   - `best, bestScore := start, capability.heuristic(from, goal)` → `best, bestScore := start, capability.toward(goal, from)`
   - `if current.Pos == goal {` → `if goal.Reached(current.Pos) {`
   - `if score := capability.heuristic(current.Pos, goal); score < bestScore {` → `if score := capability.toward(goal, current.Pos); score < bestScore {`
   - `open.push(next, through+capability.heuristic(next.Pos, goal))` → `open.push(next, through+capability.toward(goal, next.Pos))`

5. Replace the `heuristic` method (~line 259) with `toward`, keeping its doc comment's per-block reasoning:

```go
// toward scales a goal's bound, in blocks, into ticks by the cheapest cost
// this capability pays to close one block. The scale is per block closed
// rather than per edge because a step and a fall each close two blocks at
// once. It never overestimates, which is what keeps the search returning
// shortest paths.
func (c Capability) toward(goal Goal, pos geom.BlockPos) float64 {
	return goal.Heuristic(pos) * c.perBlockFloor()
}
```

In `navigation/planner.go`, `Plan` (line 64): `from, goal geom.BlockPos` becomes `from geom.BlockPos, goal Goal`; the body already passes through to `plan`.

- [x] **Step 4: Rewrite the call sites mechanically**

Run each rewrite ONCE (a second run would double-wrap):

```bash
cd /home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation
gofmt -r 'Find(a, b, c, d, e, f, g) -> Find(a, b, c, d, e, GoalBlock{Pos: f}, g)' -w navigation/*_test.go
gofmt -r 'search(a, b, c, d, e, f, g) -> search(a, b, c, d, GoalBlock{Pos: e}, f, g)' -w navigation/*_test.go
gofmt -r 'x.Plan(a, b, c, d) -> x.Plan(a, b, GoalBlock{Pos: c}, d)' -w navigation/*_test.go
gofmt -r 'navigation.Find(a, b, c, d, e, f, g) -> navigation.Find(a, b, c, d, e, navigation.GoalBlock{Pos: f}, g)' -w navigation/reach/reach_test.go
```

Then build and fix the stragglers by hand until clean (any call the patterns missed — different arity, wrapped arguments — the compiler lists them):

```bash
devbox run -- task build
```

Note: edge_test.go's `perBlockFloor` tests are untouched — the method survives; only `heuristic` is replaced by `toward`. If anything still references `capability.heuristic`, rewrite it to `toward` with the argument order swapped.

- [x] **Step 5: Run the package suite**

Run: `devbox run -- task test -- ./navigation/...`
Expected: PASS — behaviour with `GoalBlock` is exactly the old behaviour, so no existing expectation moves.

Run: `devbox run -- task determinism`
Expected: PASS — same searches, same digests.

- [x] **Step 6: Commit**

```bash
git add navigation/ 
git commit -m "feat(navigation): Find and Plan take a Goal, and a cell is the simplest one"
```

---

### Task 4: Goal behaviour through the search

**Files:**
- Modify: `navigation/goal_test.go` (append)

**Interfaces:**
- Consumes: `Find`, goal types, and the `flat`/`walker` test fixtures.

- [x] **Step 1: Write the tests**

```go
func TestFindStopsAnywhereInsideGoalNear(t *testing.T) {
	blocks := flat(-8, -8, 8, 8)
	center := geom.BlockPos{X: 6, Y: 0, Z: 0}

	path, err := Find(context.Background(), blocks, nil, walker,
		geom.BlockPos{}, GoalNear{Pos: center, Radius: 2}, Budget{Nodes: 10_000})
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}
	end := path.End(geom.BlockPos{})
	if euclidean(end, center) > 2 {
		t.Fatalf("path ends at %v, outside the radius", end)
	}
	if end == center {
		t.Fatal("path walked all the way to the center; the radius is the point")
	}
}

func TestFindFleesEverySourceOfGoalRunAway(t *testing.T) {
	blocks := flat(-16, -16, 16, 16)
	sources := []geom.BlockPos{{X: 2}, {X: -2}}

	path, err := Find(context.Background(), blocks, nil, walker,
		geom.BlockPos{}, GoalRunAway{From: sources, Distance: 6}, Budget{Nodes: 10_000})
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}
	end := path.End(geom.BlockPos{})
	for _, source := range sources {
		if euclidean(end, source) < 6 {
			t.Fatalf("path ends at %v, still within 6 of %v", end, source)
		}
	}
}

func TestFindSpendsItsBudgetFleeingAnInvertedGoal(t *testing.T) {
	blocks := flat(-16, -16, 16, 16)
	home := geom.BlockPos{}

	path, err := Find(context.Background(), blocks, nil, walker,
		home, GoalInverted{Goal: GoalBlock{Pos: home}}, Budget{Nodes: 500})
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("an inverted goal can never complete")
	}
	if path.Reason != ReasonBudget {
		t.Fatalf("reason = %v, want budget: fleeing runs until stopped", path.Reason)
	}
	if end := path.End(home); manhattan(end, home) == 0 {
		t.Fatal("the partial path went nowhere")
	}
}
```

- [x] **Step 2: Run the tests**

Run: `devbox run -- task test -- -run 'TestFind(Stops|Flees|Spends)' ./navigation/`
Expected: PASS with the Task 3 search. If `TestFindStopsAnywhereInsideGoalNear` fails on `end == center`, the fallback scoring is walking past satisfied nodes — check that `goal.Reached` is tested on `current` at pop, before expansion.

- [x] **Step 3: Commit**

```bash
git add navigation/goal_test.go
git commit -m "test(navigation): goals steer the search, not just the arithmetic"
```

---

### Task 5: Hazard-adjacency penalty

**Files:**
- Modify: `navigation/navigation.go` (`Capability` gains `HazardPenalty`)
- Modify: `navigation/oracle.go` (interface + `directOracle`)
- Modify: `navigation/memo.go` (`memoOracle` caches the new question)
- Modify: `navigation/search.go` (`expand` applies the penalty)
- Create: `navigation/hazard_test.go`

**Interfaces:**
- Consumes: `terrain.Query.HazardAt(cell) (Hazard, world.Lookup, error)`; the `steps` neighbour table in `search.go`; the memo cache pattern (`claim`, per-question map + insertion-order slice + `dependentSet` field, as `passable` in `memo.go` does).
- Produces: `Capability.HazardPenalty float64`; oracle method `hazardous(cell geom.BlockPos) (bool, error)` on both implementations.

- [x] **Step 1: Write the failing test**

```go
// navigation/hazard_test.go
package navigation

import (
	"context"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// Build a 3-lane corridor from x=0 to x=8 at z ∈ {0,1,2}, with lava filling
// z=-1 along the way, using the same fixture helpers the search tests use.
// The exact fixture construction follows flat()/the lava helper already used
// by the arrival tests in search_test.go — reuse those, do not invent new
// world plumbing. The lane at z=0 is lava-adjacent; z=2 is not.

func TestHazardPenaltySteersAwayFromTheLavaEdge(t *testing.T) {
	blocks, facts := corridorBesideLava(t) // helper built from existing fixtures

	timid := walker
	timid.HazardPenalty = 50

	path, err := Find(context.Background(), blocks, facts, timid,
		geom.BlockPos{Z: 1}, GoalBlock{Pos: geom.BlockPos{X: 8, Z: 1}}, Budget{Nodes: 10_000})
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	for _, edge := range path.Edges {
		if edge.To.Z == 0 {
			t.Fatalf("route hugs the lava at %v despite the penalty", edge.To)
		}
	}
}

func TestZeroPenaltyLeavesEveryCostAlone(t *testing.T) {
	blocks, facts := corridorBesideLava(t)

	before, err := Find(context.Background(), blocks, facts, walker,
		geom.BlockPos{Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 8, Z: 0}}, Budget{Nodes: 10_000})
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	// walker.HazardPenalty is zero: the lava-adjacent lane is the straight
	// line and must still be taken at its plain cost.
	if !before.Complete {
		t.Fatalf("path incomplete, reason %v", before.Reason)
	}
	for _, edge := range before.Edges {
		if edge.To.Z != 0 {
			t.Fatalf("zero penalty still detoured via %v", edge.To)
		}
	}
}
```

Write `corridorBesideLava` with the fixtures the package already has: the `flat` world builder from `search_test.go`, and `burningFacts` (`search_test.go:287`), which answers `terrain.HazardBurn` for one block handle. Build the flat corridor, set that handle's block along z=-1 for x 0..8, and return the blocks plus a `burningFacts` naming the handle. No new fixture machinery.

- [x] **Step 2: Run the test to verify it fails**

Run: `devbox run -- task test -- -run 'TestHazardPenalty|TestZeroPenalty' ./navigation/`
Expected: FAIL — `HazardPenalty` undefined.

- [x] **Step 3: Implement**

1. `navigation/navigation.go`, on `Capability`:

```go
	// HazardPenalty is added to the cost of any edge arriving in a cell with
	// a hazardous horizontal neighbour, in ticks. Zero means hazards beside
	// the route cost nothing, which is exactly the search before this field
	// existed. It is a penalty rather than a refusal: the cell itself is
	// legal — arrivalAt already refuses hazardous cells outright — but a body
	// walking the rim of a lava lake pays for every step it spends there,
	// so a route one lane over wins whenever one exists.
	HazardPenalty float64
```

2. `navigation/oracle.go`, add to the `oracle` interface:

```go
	// hazardous reports whether the cell itself holds a hazard. An
	// undescribed cell is not hazardous: geometry already refuses to route
	// through what nobody has described, and charging a penalty for the
	// unknown would double-count that caution.
	hazardous(cell geom.BlockPos) (bool, error)
```

and on `directOracle`:

```go
func (d directOracle) hazardous(cell geom.BlockPos) (bool, error) {
	hazard, _, err := d.query.HazardAt(cell)
	if err != nil {
		return false, err
	}

	return hazard != terrain.HazardNone, nil
}
```

3. `navigation/memo.go`: mirror the `passable` pattern exactly — a `haz map[geom.BlockPos]hazEntry` field (`type hazEntry struct { value bool; deps []geom.BlockPos }` beside the other entry types), a `hazOrder []geom.BlockPos`, a `haz` set in `dependentSet`, a new answer kind in the enum `claim` switches on, initialization in `newMemoOracle`, eviction (`forgetHaz`) and invalidation wherever the other four questions have theirs (follow every place `crawl`/`passOrder` appears and add the fifth question alongside).

4. `navigation/search.go`, at the end of `expand` before `return edges, nil`:

```go
	if c.HazardPenalty > 0 {
		for i := range edges {
			near, err := hazardBeside(o, edges[i].To)
			if err != nil {
				return nil, err
			}
			if near {
				edges[i].Cost += c.HazardPenalty
			}
		}
	}
```

and the helper beside `expand`:

```go
// hazardBeside reports whether any horizontal neighbour of a cell holds a
// hazard. Only the four flat neighbours are asked: the cell itself is
// already refused by arrival when hazardous, and the cell below is already
// refused as support when it burns.
func hazardBeside(o oracle, cell geom.BlockPos) (bool, error) {
	for _, step := range steps {
		neighbour := geom.BlockPos{X: cell.X + step.X, Y: cell.Y, Z: cell.Z + step.Z}
		hazard, err := o.hazardous(neighbour)
		if err != nil {
			return false, err
		}
		if hazard {
			return true, nil
		}
	}

	return false, nil
}
```

- [x] **Step 4: Run the tests**

Run: `devbox run -- task test -- ./navigation/...`
Expected: PASS — both new tests, and the whole existing suite untouched because every existing capability has `HazardPenalty` zero and the guard skips the lookups entirely.

Run: `devbox run -- task determinism`
Expected: PASS.

- [x] **Step 5: Benchmark guard**

Run: `devbox run -- task test -- -bench 'BenchmarkFind' -run '^$' ./navigation/`
Expected: allocations and ns/op within noise of the numbers before this task (the penalty path is behind `HazardPenalty > 0`, which no benchmark sets). If BenchmarkFindLong regresses, the guard is not short-circuiting — check that `hazardBeside` is never called when the penalty is zero.

- [x] **Step 6: Commit**

```bash
git add navigation/
git commit -m "feat(navigation): walking beside a hazard costs extra when the caller says so"
```

---

### Task 6: Changelog, verify, release note

**Files:**
- Modify: `CHANGELOG.md` (Unreleased section)

- [x] **Step 1: Write the changelog entry**

Under `## Unreleased`, following the house style of the 0.2.0 entry:

```markdown
### Changed

- `navigation`: `Find` and `Planner.Plan` take a `Goal` — a heuristic and a
  completion test — instead of one exact cell. `GoalBlock` is the old
  behaviour under the new signature; `GoalXZ`, `GoalYLevel`, `GoalNear`,
  `GoalGetToBlock`, `GoalRunAway`, `GoalComposite`, `GoalInverted`, and
  `GoalAxis` are the shapes a destination actually takes. Callers wrap their
  cell in `GoalBlock{Pos: cell}` and behave exactly as before.

### Added

- `navigation`: `Capability.HazardPenalty` charges edges that arrive beside a
  hazard, in ticks. Zero — the default — is the search exactly as it was;
  a positive value makes a route one lane away from lava beat the rim walk.
```

- [x] **Step 2: Full verification**

Run: `devbox run -- task verify`
Expected: PASS — fmt, lint, tests with race, determinism, build.

- [x] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: record the goal vocabulary and the hazard penalty"
```

- [x] **Step 4: Consumer note (do not do the work here)**

The next `minecraft-simulation` release is a breaking minor for `navigation`.
Two consumers exist:

- `headless-minecraft/examples/orbit/route.go:136` calls `navigation.Find`
  with a `geom.BlockPos` goal — a one-line wrap in `navigation.GoalBlock`
  when that repository bumps `go.mod`. Its `go.work` also `use`s this
  repository directly, so local builds of orbit break the moment Task 3
  lands; the orbit files currently carry uncommitted edits (the user's and
  another session's), so coordinate before touching that file — do not
  commit it from this plan.
- `navigation/reach` is in-repo and already updated in Task 3.

Record the wrap in the release notes when tagging, per `RELEASING.md`.
