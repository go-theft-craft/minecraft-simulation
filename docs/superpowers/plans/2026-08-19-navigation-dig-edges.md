# Navigation Dig Edges Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `subagent-driven-development` (recommended) or `executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Status: complete, 2026-08-19.** `task verify` is green and the benchmarks
> are unchanged to the allocation on the nil-breaker path. Two deviations:
> Tasks 1 through 4 landed as one commit, because the exhaustive lint rejects
> an `EdgeKind` whose switches do not handle it and so the kind, its cases, and
> its producer cannot land apart; and the oracle grew a third question,
> `arriveAfterDig`, that the plan did not name — `passableAfterDig` says the
> body fits and is held up, not that the cell is somewhere it may rest, and a
> dug doorway over lava is passable. Task 5's consumer note stands open.

**Goal:** Add `EdgeDig` to the navigation vocabulary: a body that carries a tool routes through a blocked cell by breaking what is in its way, at the break time the `mining` package computes, and the winning route is validated against the holes it made.

**Parent spec:** `headless-minecraft/docs/superpowers/specs/2026-08-18-baritone-adoption-design.md`, stage 2. It follows stage 1 (goals and hazard costs), which is complete.

## The debt this pays

`edge.go`'s own comment says Dig is absent rather than stubbed because it needs
a number a milestone still owes: break times. M9.4 paid that debt — `mining`
computes `BreakTicks` from a hardness and a `Conditions`, and each version's
profile resolves the conditions behind `mining.Classifier`, gated on a corpus
asked of each version's own jar. The number exists. This plan spends it.

## Architecture

**Navigation does not import `mining`.** `mining` imports `sim`, and nothing in
`navigation` imports `sim` — that separation is what keeps the search testable
without a tick loop. The seam is an interface `navigation` declares and a
caller implements:

```go
type Breaker interface {
    BreakTicks(ref world.BlockRef) (float64, bool)
}
```

A caller with a held tool, a set of effects, and a version profile closes over
all three and answers per block handle. `mining.BreakTicks` returns an `int`
and an error; the seam returns ticks and a `bool` because the search has one
question — how long, or never — and `mining.ErrUnbreakable` is a "never", not a
failure. Consumers translate.

**Digging is on when `Breaker` is non-nil.** No `CanDig` flag beside it: a nil
breaker cannot answer, and a non-nil one is a caller saying it holds a tool.
This differs deliberately from `CanPlace`/`BlockBudget`, and the reason is
worth stating — a placed block is consumed from a finite stack, so "zero blocks
left" is a real state the search must model, while a pickaxe does not run out
part-way along a route in any way the search can see. `DigBudget` bounds how
many cells one route may break, with a non-positive value meaning no bound,
because a caller that does not want a tunnel through a mountain needs a way to
say so and cost alone does not say it.

**A dig edge clears the body's span, not one block.** A two-block body walking
into a blocked column needs both its cells emptied. The removed cells are
derived from the destination and the body's height exactly as `placementOf`
derives a placement's cell, so nothing new is carried on `Edge`.

**Passability after a dig reuses the door machinery.** `openedView` already
answers "what would this cell be if the door in it had swung" by masking cells
to air and running the ordinary rules over the result. `dugView` is the same
decorator over the span cells, and `passableAfterDig` is the same question
`passableThroughDoor` asks. An undescribed cell stays undescribed: masking one
would turn "nobody said what is here" into "this is open", which is the
substitution every rule in this package refuses.

**The overlay learns to hold a hole.** `Overlay.Remove` today forgets a pending
placement and says so in its comment: it deliberately does not hide a base
block, because "taking blocks away is a dig edge, which is not built." It is
built now. `Overlay.Break` records a cell as air over the base view, and
validation replays a route's holes forward the same way it replays its
placements.

**Cost.** A dig edge costs the move it replaces plus the sum of the break times
of the cells it clears. It is produced only for a destination the body cannot
otherwise enter, so it never competes with a plain walk over the same cell.

## Tech Stack

Go, Devbox, Task. Repository: `minecraft-simulation`, package `navigation`.

## Global Constraints

- Work in `/home/ocharnyshevich/pet.projects/go-theft-craft/minecraft-simulation`.
- Run every command as `devbox run -- task <name>`. Scope test runs to the touched package; run full `devbox run -- task verify` only before the final commit.
- Commit directly to `main`. Never branch. Never add `Co-Authored-By` or `Claude-Session` lines.
- `EdgeKind`, `Posture`, and `Reason` values are appended, never inserted. `EdgeDig` is appended after `EdgePillar`.
- Nothing in `navigation` imports `sim` or `mining`.
- With `Breaker` nil (the default), every search must return byte-identical results to today — `devbox run -- task determinism` enforces it.

---

### Task 1: The Breaker seam and EdgeDig

**Files:**
- Modify: `navigation/edge.go` (`EdgeDig` and its `String`)
- Modify: `navigation/navigation.go` (`Breaker`, `Capability.Breaker`, `Capability.DigBudget`)
- Create: `navigation/dig_test.go`

- [x] **Step 1:** Test that `EdgeDig.String()` is `"dig"` and that it is numbered after `EdgePillar`, in the style of `edge_test.go`'s existing kind test.
- [x] **Step 2:** Run, watch it fail on `undefined: EdgeDig`.
- [x] **Step 3:** Append `EdgeDig` to the `EdgeKind` block with a doc comment saying what it clears and why the cells are derived; add the `String` case; delete the sentence in the `EdgeKind` comment that says Dig is absent, and leave Support and Collapse named there. Declare `Breaker` and the two `Capability` fields.
- [x] **Step 4:** Run, watch it pass.
- [x] **Step 5:** Commit.

---

### Task 2: dugView and the oracle question

**Files:**
- Create: `navigation/dig.go` (`dugView`, `spanOf`)
- Modify: `navigation/oracle.go` (interface, `directOracle`)
- Modify: `navigation/memo.go` (`memoOracle`)
- Modify: `navigation/dig_test.go`

**Interfaces:**
- Produces: `func spanOf(body terrain.Body, cell geom.BlockPos) []geom.BlockPos`; `type dugView struct{ view world.View; span []geom.BlockPos }`; oracle methods `passableAfterDig(cell geom.BlockPos, span []geom.BlockPos) (terrain.Passability, error)` and `breakTicks(cell geom.BlockPos) (float64, bool, error)`.

- [x] **Step 1:** Test that `spanOf` returns two cells for a 1.8-tall body and three for a 2.5-tall one, and that `dugView` answers air for a span cell and defers for every other — including leaving an undescribed cell undescribed.
- [x] **Step 2:** Run, watch it fail.
- [x] **Step 3:** Implement. `spanOf` is `cell.Y` through `cell.Y + ceil(Height) - 1`. `dugView` mirrors `openedView`: `CollisionShape` masks a covered, described cell to `geom.EmptyShape()`/`world.LookupAir`, `BlockState` tells the truth. `passableAfterDig` builds the masked query per call, as `passableThroughDoor` does and for the same reason. `breakTicks` reads `BlockState`, refuses an unknown lookup, and asks the capability's `Breaker`. Both are uncached on `memoOracle`, for the reason `fluidAt` is: a single block lookup, not a collision sweep.
- [x] **Step 4:** Run, watch it pass.
- [x] **Step 5:** Commit.

---

### Task 3: The dig edge builder

**Files:**
- Modify: `navigation/dig.go` (`digs`)
- Modify: `navigation/search.go` (`expand` calls it)
- Modify: `navigation/dig_test.go`

- [x] **Step 1:** Tests: a wall of one breakable block across a corridor is routed through by a body with a breaker and around by one without; the dig edge's cost is the walk cost plus the break time of every cell it clears; an unbreakable wall (the breaker answers false) produces no dig edge; a body whose `DigBudget` is smaller than the wall is thick still finds no route through it.
- [x] **Step 2:** Run, watch them fail.
- [x] **Step 3:** Implement `digs(o, from)`: for each of the four horizontal neighbours, when `passable` is not `Clear`, compute the span, ask `breakTicks` for each span cell that is not already clear, and refuse the edge if any answers "never". Then ask `passableAfterDig` for the destination and produce an `EdgeDig` at `WalkTicks` plus the sum when it says `Clear`. `expand` calls `digs` only when `c.Breaker != nil`, so the read-only search pays nothing.
- [x] **Step 4:** Run, watch them pass. Run the benchmarks and confirm the nil-breaker path is unchanged.
- [x] **Step 5:** Commit.

---

### Task 4: Validation replays the holes

**Files:**
- Modify: `navigation/overlay.go` (`Break`, and the `Remove` comment it falsifies)
- Modify: `navigation/place.go` (`removalOf`, `validate`, `edgeHolds`, `mutates`)
- Modify: `navigation/dig_test.go`

- [x] **Step 1:** Tests: a route that digs through a wall and then walks over the cell it dug is caught by validation and re-searched (the ban loop already exists — this asserts a dig participates in it); `mutates` is true for a breaker-only body; the count of broken cells is charged against `DigBudget` across the whole route rather than per edge.
- [x] **Step 2:** Run, watch them fail.
- [x] **Step 3:** Implement. `Overlay.Break(pos)` records a hole in a second map that `CollisionShape` and `BlockState` consult; `Remove`'s comment loses the sentence about dig edges not being built. `removalOf(edge)` returns the span for `EdgeDig` and nothing for every other kind, in the shape `placementOf` already has — including its exhaustive `case` list, so a new kind fails to compile rather than falling through. `validate` counts broken cells against `DigBudget` and replays them into the overlay. `edgeHolds` gains an `EdgeDig` case asking `passableAfterDig`. `mutates` becomes `(CanPlace && BlockBudget > 0) || Breaker != nil`.
- [x] **Step 4:** Run the package suite and `task determinism`.
- [x] **Step 5:** Commit.

---

### Task 5: Changelog, verify, consumer note

**Files:**
- Modify: `CHANGELOG.md` (Unreleased)
- Modify: this plan (status header, checkboxes)

- [x] **Step 1:** Changelog entry under the existing Unreleased section, in the house style: `EdgeDig`, the `Breaker` seam and why it is an interface rather than a `mining` import, and `DigBudget`.
- [x] **Step 2:** `devbox run -- task verify`.
- [x] **Step 3:** Commit.
- [x] **Step 4:** Consumer note, recorded and **not acted on here**: a caller that wants digging builds a `Breaker` from `mining.BreakTicks` and its version's `mining.Classifier`. That adapter belongs in the consumer, next to where the held item and the effects live, and `headless-minecraft` owns writing it. Nothing in this repository ships one, so nothing in this repository proves the two halves meet — say so when tagging.
