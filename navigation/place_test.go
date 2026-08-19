package navigation

import (
	"context"
	"testing"
	"time"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// builder is walker with an inventory and a placement price.
//
// The prices are distinct from every other edge's so a wrong kind shows up as a
// wrong total rather than coinciding with the right one.
func builder() Capability {
	capability := walker
	capability.CanPlace = true
	capability.PlaceTicks = 31
	capability.BlockTicks = 2
	capability.BlockBudget = 16
	capability.PlacedBlock = refStone

	return capability
}

// wideGap returns a floor with a run of columns cut clean out of it, wider than
// any jump reach, so the only crossing is a bridge.
func wideGap(width int32) *world.Blocks {
	blocks := flat(-1, -1, width+3, 1)
	for x := int32(1); x <= width; x++ {
		blocks.Fill(
			geom.BlockPos{X: x, Y: -1, Z: -1},
			geom.BlockPos{X: x, Y: -1, Z: 1},
			geom.EmptyShape(),
		)
	}

	return blocks
}

func countKind(path Path, kind EdgeKind) int {
	var found int
	for _, edge := range path.Edges {
		if edge.Kind == kind {
			found++
		}
	}

	return found
}

// TestABodyBridgesAGapItCannotJump is the edge's reason for existing.
//
// Six blocks of air is further than any measured jump reach, so nothing in the
// read-only vocabulary crosses it. A body with blocks walks over.
func TestABodyBridgesAGapItCannotJump(t *testing.T) {
	path, err := Find(
		context.Background(), wideGap(6), nil, builder(),
		geom.BlockPos{X: 0, Y: 0, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 7, Y: 0, Z: 0}},
		wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}
	if placed := countKind(path, EdgePlace); placed == 0 {
		t.Fatalf("crossed a six-block gap with no placement: %v", path.Edges)
	}
}

// TestABodyWithNoBlocksDoesNotBridge pins the inventory gate.
func TestABodyWithNoBlocksDoesNotBridge(t *testing.T) {
	capability := builder()
	capability.BlockBudget = 0

	path, err := Find(
		context.Background(), wideGap(6), nil, capability,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 7, Y: 0, Z: 0}},
		wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if countKind(path, EdgePlace) != 0 {
		t.Fatal("an empty inventory produced a placement")
	}
	if path.Complete {
		t.Fatal("crossed a six-block gap with an empty inventory")
	}
}

// TestABodyThatCannotPlaceRoutesAsItDidBefore is the additivity claim.
//
// Everything in this amendment is additive: a capability with placement off
// must produce exactly the path it produced before the overlay existed, and it
// must skip the validation loop entirely.
func TestABodyThatCannotPlaceRoutesAsItDidBefore(t *testing.T) {
	if walker.mutates() {
		t.Fatal("the shipped body reports that it changes the world")
	}

	view := wideGap(6)
	plain, err := Find(
		context.Background(), view, nil, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 7, Y: 0, Z: 0}},
		wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	for _, edge := range plain.Edges {
		if edge.Kind == EdgePlace || edge.Kind == EdgePillar {
			t.Fatalf("a body that cannot place produced a %v edge", edge.Kind)
		}
	}
}

// TestTheInventoryBoundsTheBridge pins the resource accounting.
//
// The body carries fewer blocks than the gap is wide, so no bridge it can pay
// for reaches the far side.
func TestTheInventoryBoundsTheBridge(t *testing.T) {
	capability := builder()
	capability.BlockBudget = 2

	path, err := Find(
		context.Background(), wideGap(6), nil, capability,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 7, Y: 0, Z: 0}},
		wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if placed := countKind(path, EdgePlace); placed > capability.BlockBudget {
		t.Fatalf("placed %d blocks with %d in the inventory", placed, capability.BlockBudget)
	}
	if path.Complete {
		t.Fatal("bridged six blocks with two in the inventory")
	}
}

// TestAReturnedPathIsSelfConsistent is the validation loop's own gate.
//
// A winning path can be internally inconsistent because the search compares
// branches that never saw each other's placements. Replaying the winner forward
// against an overlay is how that is caught, and this asserts that what Find
// hands back has already been through it.
func TestAReturnedPathIsSelfConsistent(t *testing.T) {
	capability := builder()

	for width := int32(1); width <= 6; width++ {
		view := wideGap(width)
		path, err := Find(
			context.Background(), view, nil, capability,
			geom.BlockPos{X: 0, Y: 0, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: width + 1, Y: 0, Z: 0}},
			wideBudget)
		if err != nil {
			t.Fatalf("width %d: Find returned an error: %v", width, err)
		}

		offender, conflicted, err := capability.validate(view, nil, path)
		if err != nil {
			t.Fatalf("width %d: validate: %v", width, err)
		}
		if conflicted {
			t.Fatalf("width %d: Find returned a self-inconsistent path; %v %v -> %v does not hold",
				width, offender.Kind, offender.From, offender.To)
		}
	}
}

// TestTheValidationLoopTerminates pins the bound.
//
// Each round bans one edge, so the loop makes progress on any finite set of
// routes. This runs it against a world with a great many equivalent bridges,
// which is the shape that would spin if a round ever banned nothing.
func TestTheValidationLoopTerminates(t *testing.T) {
	capability := builder()
	capability.BlockBudget = 64

	blocks := flat(-1, -4, 20, 4)
	for x := int32(1); x <= 18; x++ {
		blocks.Fill(
			geom.BlockPos{X: x, Y: -1, Z: -4},
			geom.BlockPos{X: x, Y: -1, Z: 4},
			geom.EmptyShape(),
		)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := Find(
			context.Background(), blocks, nil, capability,
			geom.BlockPos{X: 0, Y: 0, Z: 0}, GoalBlock{Pos: geom.BlockPos{X: 19, Y: 0, Z: 0}},

			Budget{Nodes: 20_000, Ceiling: 20_000}); err != nil {
			t.Errorf("Find returned an error: %v", err)
		}
	}()

	select {
	case <-done:
	case <-timeAfterSeconds(30):
		t.Fatal("the validation loop did not terminate within thirty seconds")
	}
}

// timeAfterSeconds is a thin wrapper so the termination test reads as a bound
// rather than as a timer.
func timeAfterSeconds(seconds int) <-chan time.Time {
	return time.After(time.Duration(seconds) * time.Second)
}
