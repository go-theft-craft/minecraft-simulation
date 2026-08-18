package navigation

import (
	"context"
	"slices"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// jumper is walker with a jump. The reach is 3.5 rather than either profile's
// measured value so that a two-block hole is inside it and a three-block hole
// is not, which is what makes the gating assertions here about the gate rather
// than about a number that may move when a kernel constant does.
//
// The measured values are smaller: both profiles clear a little over two blocks
// from a standing start, which crosses a one-block hole. That the real
// measurement reaches the search at all is pinned in navigation/reach, where
// the profiles it needs may be imported.
func jumpingCapability() Capability {
	capability := walker
	capability.JumpTicks = 13
	capability.JumpReach = 3.5
	capability.JumpRise = 1.25

	return capability
}

// gap returns a floor at y=-1 with holes cut clean across it.
//
// Each hole spans the full width of the described floor rather than one cell,
// because flat lays three rows and a hole in the middle one is a hole with a
// detour around it. A test about crossing wants no way round.
func gap(maxX int32, holes ...int32) *world.Blocks {
	blocks := flat(-1, -1, maxX+1, 1)
	for _, x := range holes {
		blocks.Fill(
			geom.BlockPos{X: x, Y: -1, Z: -1},
			geom.BlockPos{X: x, Y: -1, Z: 1},
			geom.EmptyShape(),
		)
	}

	return blocks
}

func hasJump(path Path) bool {
	return slices.ContainsFunc(path.Edges, func(e Edge) bool { return e.Kind == EdgeJumpGap })
}

// TestABodyCrossesATwoBlockGap is the reachability this whole edge exists for.
//
// Nothing in the shipped vocabulary reaches the far side of a hole: EdgeStep
// rises into an adjacent cell and EdgeFall descends into one, and neither
// crosses. Before this edge the goal below was simply unreachable.
func TestABodyCrossesATwoBlockGap(t *testing.T) {
	path, err := Find(
		context.Background(), gap(5, 2, 3), nil, jumpingCapability(),
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}
	if !hasJump(path) {
		t.Fatalf("crossed the gap without a jump edge: %v", path.Edges)
	}
}

// TestACapabilityWithNoJumpReachDoesNotJump pins the switch a mob is given.
//
// A capability with no reach is the ground navigator every non-jumping body
// gets out of this same search, and it must produce no jump edge at all rather
// than a cheap one.
func TestACapabilityWithNoJumpReachDoesNotJump(t *testing.T) {
	capability := jumpingCapability()
	capability.JumpReach = 0

	path, err := Find(
		context.Background(), gap(5, 2, 3), nil, capability,
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if hasJump(path) {
		t.Fatal("a capability with no jump reach produced a jump edge")
	}
	// With the floor gone at x=2 and x=3 and no way across, the far side is
	// genuinely unreachable: this is the detour case with no detour.
	if path.Complete {
		t.Fatal("crossed a two-block hole without a jump")
	}
}

// TestACapabilityRoutesAroundAGapItCannotJump pins that a body which cannot
// jump still gets where it is going when there is another way.
func TestACapabilityRoutesAroundAGapItCannotJump(t *testing.T) {
	// The hole is one cell wide at z=0 only, so the rows either side are
	// intact and the detour is a sidestep, two walks, and a sidestep back.
	blocks := flat(-1, -1, 5, 1)
	blocks.Set(geom.BlockPos{X: 2, Y: -1, Z: 0}, geom.EmptyShape())

	capability := jumpingCapability()
	capability.JumpReach = 0

	path, err := Find(
		context.Background(), blocks, nil, capability,
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 3, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("the detour was not found: %v", path.Reason)
	}
	if hasJump(path) {
		t.Fatal("a capability with no jump reach produced a jump edge")
	}
}

// TestAJumpUnderALowCeilingIsRefused is the test that catches a clearance check
// which only looks at the endpoints.
//
// The arc rises. A block one above the middle of the gap is what the body's
// head meets, and both ends of the jump are perfectly clear while it does.
func TestAJumpUnderALowCeilingIsRefused(t *testing.T) {
	start, goal := geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}

	open := gap(5, 2, 3)
	blocked := gap(5, 2, 3)
	blocked.Fill(
		geom.BlockPos{X: 2, Y: 1, Z: -1},
		geom.BlockPos{X: 2, Y: 1, Z: 1},
		geom.FullCube(),
	)

	openPath, err := Find(context.Background(), open, nil, jumpingCapability(), start, goal, wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	blockedPath, err := Find(context.Background(), blocked, nil, jumpingCapability(), start, goal, wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}

	if !hasJump(openPath) {
		t.Fatal("the gap without a ceiling produced no jump")
	}
	if hasJump(blockedPath) {
		t.Fatal("jumped through a block sitting in the arc")
	}
}

// TestAJumpIsRefusedWhenTheLandingIsNotStandable pins that the far side is
// checked the same way every other arrival is.
func TestAJumpIsRefusedWhenTheLandingIsNotStandable(t *testing.T) {
	// The hole runs to the end of the floor, so there is no far side at all.
	blocks := gap(5, 2, 3, 4)

	path, err := Find(
		context.Background(), blocks, nil, jumpingCapability(),
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if hasJump(path) {
		t.Fatal("jumped to a cell with no floor under it")
	}
}

// TestAJumpCostsItsDistance pins that a longer jump is priced as one, so the
// search prefers the short hop when both reach.
func TestAJumpCostsItsDistance(t *testing.T) {
	capability := jumpingCapability()

	path, err := Find(
		context.Background(), gap(5, 2), nil, capability,
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 3, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}

	index := slices.IndexFunc(path.Edges, func(e Edge) bool { return e.Kind == EdgeJumpGap })
	if index < 0 {
		t.Fatalf("no jump edge over a one-block hole: %v", path.Edges)
	}
	jump := path.Edges[index]
	if want := capability.JumpTicks * 2; jump.Cost != want {
		t.Errorf("a two-block jump cost %v, want %v", jump.Cost, want)
	}
	if jump.Posture != PostureStand {
		t.Errorf("landed in posture %v, want stand", jump.Posture)
	}
}

// TestAFallingBodyDoesNotJump pins the constraint PostureFall exists to encode.
//
// Nothing in this package produces that posture yet — Pillar does, in the
// mutating-edge amendment — so the rule is asserted against the expansion
// directly rather than through a route.
func TestAFallingBodyDoesNotJump(t *testing.T) {
	capability := jumpingCapability()
	view := gap(5, 2, 3)
	o := directOracle{query: capability.query(view, nil), capability: capability}

	grounded, err := capability.jumps(o, node{Pos: geom.BlockPos{X: 1, Y: 0, Z: 0}, Posture: PostureStand})
	if err != nil {
		t.Fatalf("jumps: %v", err)
	}
	if len(grounded) == 0 {
		t.Fatal("a standing body produced no jump, so the airborne case proves nothing")
	}

	airborne, err := capability.jumps(o, node{Pos: geom.BlockPos{X: 1, Y: 0, Z: 0}, Posture: PostureFall})
	if err != nil {
		t.Fatalf("jumps: %v", err)
	}
	if len(airborne) != 0 {
		t.Fatalf("an airborne body produced %d jump edges, want none", len(airborne))
	}
}

// TestAReachBelowOneJumpProducesNothing pins the boundary of the gate.
//
// A reach under two blocks cannot cross anything, because the shortest jump the
// search offers lands two cells away. A reach of one would otherwise produce a
// jump to the adjacent cell, which is a walk wearing a different price.
func TestAReachBelowOneJumpProducesNothing(t *testing.T) {
	capability := jumpingCapability()
	capability.JumpReach = 1.9
	view := gap(5, 2)
	o := directOracle{query: capability.query(view, nil), capability: capability}

	edges, err := capability.jumps(o, node{Pos: geom.BlockPos{X: 1, Y: 0, Z: 0}, Posture: PostureStand})
	if err != nil {
		t.Fatalf("jumps: %v", err)
	}
	if len(edges) != 0 {
		t.Fatalf("a reach of %v produced %d jump edges, want none", capability.JumpReach, len(edges))
	}
}
