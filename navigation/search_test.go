package navigation

import (
	"context"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// walker is a two-block body whose costs are distinct primes, so a wrong edge
// kind shows up as a wrong total rather than coinciding with the right one.
var walker = Capability{
	Body:      terrain.Body{HalfWidth: 0.3, Height: 1.8, StepHeight: 0.6},
	SafeFall:  3,
	WalkTicks: 5,
	StepTicks: 7,
	FallTicks: 3,
	SwimTicks: 11,
}

// flat returns a floor at y=-1 spanning the inclusive range, with air above it.
func flat(minX, minZ, maxX, maxZ int32) *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(
		geom.BlockPos{X: minX, Y: -1, Z: minZ},
		geom.BlockPos{X: maxX, Y: -1, Z: maxZ},
		geom.FullCube(),
	)
	blocks.Fill(
		geom.BlockPos{X: minX, Y: 0, Z: minZ},
		geom.BlockPos{X: maxX, Y: 3, Z: maxZ},
		geom.EmptyShape(),
	)

	return blocks
}

var wideBudget = Budget{Nodes: 10_000, Ceiling: 10_000}

func TestFindWalksAStraightLine(t *testing.T) {
	path, err := Find(
		context.Background(), flat(-1, -1, 5, 1), nil, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}
	if len(path.Edges) != 4 {
		t.Fatalf("len(Edges) = %d, want 4", len(path.Edges))
	}
	if path.Cost != 20 {
		t.Fatalf("Cost = %v, want 20", path.Cost)
	}
	for i, edge := range path.Edges {
		if edge.Kind != EdgeWalk {
			t.Fatalf("edge %d kind = %v, want EdgeWalk", i, edge.Kind)
		}
	}
}

func TestFindReturnsAnEmptyCompletePathAtTheGoal(t *testing.T) {
	here := geom.BlockPos{X: 0, Y: 0, Z: 0}

	path, err := Find(context.Background(), flat(-1, -1, 1, 1), nil, walker, here, here, wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete || len(path.Edges) != 0 || path.Cost != 0 {
		t.Fatalf("path = %+v, want an empty complete path", path)
	}
}

func TestFindStepsOverAOneBlockRise(t *testing.T) {
	blocks := flat(-1, -1, 4, 1)
	blocks.Set(geom.BlockPos{X: 2, Y: 0, Z: 0}, geom.FullCube())
	blocks.Set(geom.BlockPos{X: 2, Y: -1, Z: 0}, geom.FullCube())

	path, err := Find(
		context.Background(), blocks, nil, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 2, Y: 1, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}
	last := path.Edges[len(path.Edges)-1]
	if last.Kind != EdgeStep {
		t.Fatalf("last edge kind = %v, want EdgeStep", last.Kind)
	}
	if last.To != (geom.BlockPos{X: 2, Y: 1, Z: 0}) {
		t.Fatalf("last edge To = %v, want {2 1 0}", last.To)
	}
}

func TestFindFallsWithinTheSafeFall(t *testing.T) {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1, Y: -1, Z: -1}, geom.BlockPos{X: 0, Y: -1, Z: 1}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -1, Y: 0, Z: -1}, geom.BlockPos{X: 3, Y: 3, Z: 1}, geom.EmptyShape())
	blocks.Fill(geom.BlockPos{X: 1, Y: -3, Z: -1}, geom.BlockPos{X: 3, Y: -3, Z: 1}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: 1, Y: -2, Z: -1}, geom.BlockPos{X: 3, Y: -1, Z: 1}, geom.EmptyShape())

	path, err := Find(
		context.Background(), blocks, nil, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 1, Y: -2, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}
	if path.Edges[0].Kind != EdgeFall {
		t.Fatalf("first edge kind = %v, want EdgeFall", path.Edges[0].Kind)
	}
}

func TestFindRefusesAFallBeyondTheSafeFall(t *testing.T) {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1, Y: -1, Z: -1}, geom.BlockPos{X: 0, Y: -1, Z: 1}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -1, Y: 0, Z: -1}, geom.BlockPos{X: 3, Y: 3, Z: 1}, geom.EmptyShape())
	blocks.Fill(geom.BlockPos{X: 1, Y: -9, Z: -1}, geom.BlockPos{X: 3, Y: -9, Z: 1}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: 1, Y: -8, Z: -1}, geom.BlockPos{X: 3, Y: -1, Z: 1}, geom.EmptyShape())

	path, err := Find(
		context.Background(), blocks, nil, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 1, Y: -8, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("Find crossed a fall beyond the capability's safe fall")
	}
}

// An unloaded cell is refused, never guessed. This is the property that keeps
// a bot from walking into a wall it could not see.
func TestFindRefusesToRouteThroughUnknownCells(t *testing.T) {
	blocks := flat(-1, -1, 5, 1)
	blocks.Forget(geom.BlockPos{X: 2, Y: 0, Z: 0})
	blocks.Forget(geom.BlockPos{X: 2, Y: 0, Z: 1})
	blocks.Forget(geom.BlockPos{X: 2, Y: 0, Z: -1})

	path, err := Find(
		context.Background(), blocks, nil, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("Find routed through an undescribed region")
	}
	if path.Reason != ReasonUnreachable {
		t.Fatalf("Reason = %v, want ReasonUnreachable", path.Reason)
	}
}

// A bounded search reports the best it found rather than nothing, so a body
// that cannot see the whole route still makes progress.
func TestFindReturnsAPartialPathWhenTheBudgetRunsOut(t *testing.T) {
	path, err := Find(
		context.Background(), flat(-1, -1, 40, 1), nil, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 39, Y: 0, Z: 0},
		Budget{Nodes: 12, Ceiling: 10_000},
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("Find completed a route its budget could not cover")
	}
	if path.Reason != ReasonBudget {
		t.Fatalf("Reason = %v, want ReasonBudget", path.Reason)
	}
	if len(path.Edges) == 0 {
		t.Fatal("a partial path with no edges makes no progress")
	}
	if path.Edges[0].From != (geom.BlockPos{X: 0, Y: 0, Z: 0}) {
		t.Fatalf("partial path starts at %v, want the origin", path.Edges[0].From)
	}
}

func TestFindHonoursACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Find(
		ctx, flat(-1, -1, 40, 1), nil, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 39, Y: 0, Z: 0}, wideBudget,
	)
	if err == nil {
		t.Fatal("Find ignored a cancelled context")
	}
}

const refWater world.BlockRef = 9

// waterFacts answers water for one handle and nothing for the rest.
type waterFacts struct{}

func (waterFacts) Hazard(world.BlockRef) terrain.Hazard { return terrain.HazardNone }

func (waterFacts) Fluid(ref world.BlockRef) terrain.Fluid {
	if ref == refWater {
		return terrain.FluidWater
	}

	return terrain.FluidNone
}

// pool returns a flat world whose x=2 column is water to head height.
func pool() *world.Blocks {
	blocks := flat(-1, -1, 5, 1)
	for y := int32(0); y <= 1; y++ {
		blocks.SetBlock(geom.BlockPos{X: 2, Y: y, Z: 0}, refWater, geom.EmptyShape())
	}

	return blocks
}

func TestFindCrossesWaterWhenTheBodyCanSwim(t *testing.T) {
	swimmer := walker
	swimmer.CanSwim = true

	path, err := Find(
		context.Background(), pool(), waterFacts{}, swimmer,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}

	var swam bool
	for _, edge := range path.Edges {
		if edge.Kind == EdgeSwim {
			swam = true
			if edge.Posture != PostureSwim {
				t.Fatalf("swim edge posture = %v, want PostureSwim", edge.Posture)
			}
		}
	}
	if !swam {
		t.Fatal("a route through water contains no swim edge")
	}
}

// The same world, the same goal, a body that cannot swim: it must route around
// rather than through. This is the mob case the design promises.
func TestFindRoutesAroundWaterWhenTheBodyCannotSwim(t *testing.T) {
	path, err := Find(
		context.Background(), pool(), waterFacts{}, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}
	for _, edge := range path.Edges {
		if edge.Kind == EdgeSwim {
			t.Fatal("a body that cannot swim took a swim edge")
		}
		if edge.To == (geom.BlockPos{X: 2, Y: 0, Z: 0}) {
			t.Fatal("a body that cannot swim entered the water")
		}
	}
}

const refFire world.BlockRef = 51

// burningFacts answers HazardBurn for one handle. Fire carries no collision
// shape and no fluid, so nothing but the hazard lookup can find it.
type burningFacts struct{}

func (burningFacts) Fluid(world.BlockRef) terrain.Fluid { return terrain.FluidNone }

func (burningFacts) Hazard(ref world.BlockRef) terrain.Hazard {
	if ref == refFire {
		return terrain.HazardBurn
	}

	return terrain.HazardNone
}

func TestFindRoutesAroundFire(t *testing.T) {
	blocks := flat(-1, -1, 5, 1)
	blocks.SetBlock(geom.BlockPos{X: 2, Y: 0, Z: 0}, refFire, geom.EmptyShape())

	path, err := Find(
		context.Background(), blocks, burningFacts{}, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 4, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}
	for _, edge := range path.Edges {
		if edge.To == (geom.BlockPos{X: 2, Y: 0, Z: 0}) {
			t.Fatal("the route walks through fire")
		}
	}
}
