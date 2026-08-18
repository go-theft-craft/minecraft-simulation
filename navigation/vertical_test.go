package navigation

import (
	"context"
	"slices"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

const refLadder world.BlockRef = 65

// verticalFacts answers water for one handle and climbable for another, which
// is the pair the two edges in vertical.go need.
type verticalFacts struct{}

func (verticalFacts) Hazard(world.BlockRef) terrain.Hazard { return terrain.HazardNone }

func (verticalFacts) Fluid(ref world.BlockRef) terrain.Fluid {
	if ref == refWater {
		return terrain.FluidWater
	}

	return terrain.FluidNone
}

// Climbable is the fact no collision shape carries. A ladder's box is empty, so
// a caller reading geometry alone cannot tell this handle from air.
func (verticalFacts) Climbable(ref world.BlockRef) bool { return ref == refLadder }

// swimmer is walker with a swim, a shallow safe fall, and water deep enough to
// land in stated at two blocks.
func swimmer() Capability {
	capability := walker
	capability.CanSwim = true
	capability.SafeFall = 3
	capability.WaterLandingDepth = 2

	return capability
}

// waterShaft returns a floor at the top with a shaft down one column, and depth
// blocks of water at the bottom of it.
//
// The shaft is ten deep, well past the three-block safe fall, so nothing but
// what is at the bottom can make the drop legal.
func waterShaft(depth int32, fill world.BlockRef, shape geom.Shape) *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1, Y: -12, Z: -1}, geom.BlockPos{X: 3, Y: 4, Z: 1}, geom.EmptyShape())
	// The rim the body walks off.
	blocks.Fill(geom.BlockPos{X: -1, Y: -1, Z: -1}, geom.BlockPos{X: 1, Y: -1, Z: 1}, geom.FullCube())
	// The floor of the shaft, ten below.
	blocks.Fill(geom.BlockPos{X: 2, Y: -12, Z: -1}, geom.BlockPos{X: 2, Y: -12, Z: 1}, geom.FullCube())
	for y := int32(0); y < depth; y++ {
		blocks.SetBlock(geom.BlockPos{X: 2, Y: -11 + y, Z: 0}, fill, shape)
	}

	return blocks
}

// TestADeepDropIntoWaterIsTaken pins the edge's whole reason for existing: a
// drop can be survivable because of what is at the bottom rather than because
// of how far it is.
func TestADeepDropIntoWaterIsTaken(t *testing.T) {
	path, err := Find(
		context.Background(), waterShaft(4, refWater, geom.EmptyShape()), verticalFacts{}, swimmer(),
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 2, Y: -8, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("a ten-block drop into four blocks of water was refused: %v", path.Reason)
	}
	if !slices.ContainsFunc(path.Edges, func(e Edge) bool { return e.Kind == EdgeWaterDrop }) {
		t.Fatalf("reached the water without a water-drop edge: %v", path.Edges)
	}
}

// TestTheSameDropOntoStoneIsRefused is the control. Only the block at the
// bottom differs.
func TestTheSameDropOntoStoneIsRefused(t *testing.T) {
	path, err := Find(
		context.Background(), waterShaft(0, refWater, geom.EmptyShape()), verticalFacts{}, swimmer(),
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 2, Y: -11, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("a ten-block drop onto stone was taken with a three-block safe fall")
	}
}

// TestShallowWaterDoesNotBreakTheFall pins the depth rule.
//
// One block of water is not a landing. A body that treated any water as a
// landing would step off every cliff with a puddle at the bottom.
func TestShallowWaterDoesNotBreakTheFall(t *testing.T) {
	path, err := Find(
		context.Background(), waterShaft(1, refWater, geom.EmptyShape()), verticalFacts{}, swimmer(),
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 2, Y: -11, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("one block of water broke a ten-block fall")
	}
}

// TestACapabilityWithNoLandingDepthDropsNoFurtherThanItsSafeFall pins the
// default, which is the search exactly as it was before this edge.
func TestACapabilityWithNoLandingDepthDropsNoFurtherThanItsSafeFall(t *testing.T) {
	capability := swimmer()
	capability.WaterLandingDepth = 0

	path, err := Find(
		context.Background(), waterShaft(4, refWater, geom.EmptyShape()), verticalFacts{}, capability,
		geom.BlockPos{X: 1, Y: 0, Z: 0}, geom.BlockPos{X: 2, Y: -8, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("a capability with no stated landing depth dropped past its safe fall")
	}
}

// shaft returns a floor with a ladder column rising out of it.
func shaft(height int32) *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1, Y: -1, Z: -1}, geom.BlockPos{X: 1, Y: -1, Z: 1}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -1, Y: 0, Z: -1}, geom.BlockPos{X: 1, Y: height + 2, Z: 1}, geom.EmptyShape())
	// The ladder occupies its cells and blocks nothing: an empty shape is the
	// point, because it is what makes the fact unreadable from geometry.
	for y := int32(0); y <= height; y++ {
		blocks.SetBlock(geom.BlockPos{X: 0, Y: y, Z: 0}, refLadder, geom.EmptyShape())
	}
	// A landing at the top, so the climb ends somewhere the body can stand.
	blocks.Fill(
		geom.BlockPos{X: 1, Y: height - 1, Z: -1},
		geom.BlockPos{X: 1, Y: height - 1, Z: 1},
		geom.FullCube(),
	)

	return blocks
}

// climberCapability is walker with a climb.
func climberCapability() Capability {
	capability := walker
	capability.CanClimb = true
	capability.ClimbTicks = 23

	return capability
}

// TestALadderIsClimbedInBothDirections pins the edge in both directions,
// because a column that can only be gone up is a trap.
func TestALadderIsClimbedInBothDirections(t *testing.T) {
	view := shaft(6)
	bottom, top := geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 0, Y: 6, Z: 0}

	up, err := Find(context.Background(), view, verticalFacts{}, climberCapability(), bottom, top, wideBudget)
	if err != nil {
		t.Fatalf("Find up returned an error: %v", err)
	}
	down, err := Find(context.Background(), view, verticalFacts{}, climberCapability(), top, bottom, wideBudget)
	if err != nil {
		t.Fatalf("Find down returned an error: %v", err)
	}

	if !up.Complete {
		t.Fatalf("the ladder was not climbed up: %v", up.Reason)
	}
	if !down.Complete {
		t.Fatalf("the ladder was not climbed down: %v", down.Reason)
	}
	for _, path := range []Path{up, down} {
		if !slices.ContainsFunc(path.Edges, func(e Edge) bool { return e.Kind == EdgeClimb }) {
			t.Fatalf("reached the other end without a climb edge: %v", path.Edges)
		}
	}
}

// TestACapabilityThatCannotClimbDoesNotClimb is the same shaft with the
// capability taken away.
func TestACapabilityThatCannotClimbDoesNotClimb(t *testing.T) {
	capability := climberCapability()
	capability.CanClimb = false

	path, err := Find(
		context.Background(), shaft(6), verticalFacts{}, capability,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 0, Y: 6, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	for _, edge := range path.Edges {
		if edge.Kind == EdgeClimb {
			t.Fatal("a capability that cannot climb produced a climb edge")
		}
	}
	if path.Complete {
		t.Fatal("reached the top of a ladder shaft without climbing")
	}
}

// TestAClimbCostsItsOwnPrice pins that the edge is priced apart from a step.
func TestAClimbCostsItsOwnPrice(t *testing.T) {
	capability := climberCapability()

	path, err := Find(
		context.Background(), shaft(6), verticalFacts{}, capability,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 0, Y: 6, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	for _, edge := range path.Edges {
		if edge.Kind == EdgeClimb && edge.Cost != capability.ClimbTicks {
			t.Errorf("a climb cost %v, want %v", edge.Cost, capability.ClimbTicks)
		}
	}
}

// TestNothingClimbsAColumnThatIsNotClimbable is the test that fails if the
// climbable fact is being read off geometry.
//
// The world is the same shaft with the ladder handle swapped for one the facts
// do not call climbable. Its shape is empty either way, so a search reading
// collision alone cannot tell the two worlds apart.
func TestNothingClimbsAColumnThatIsNotClimbable(t *testing.T) {
	view := shaft(6)
	for y := int32(0); y <= 6; y++ {
		view.SetBlock(geom.BlockPos{X: 0, Y: y, Z: 0}, refWater+100, geom.EmptyShape())
	}

	path, err := Find(
		context.Background(), view, verticalFacts{}, climberCapability(),
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 0, Y: 6, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	for _, edge := range path.Edges {
		if edge.Kind == EdgeClimb {
			t.Fatal("climbed a column nothing said was climbable")
		}
	}
}
