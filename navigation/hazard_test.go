package navigation

import (
	"context"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// corridorBesideLava is three walkable lanes at z = 0, 1, and 2 running from
// x = 0 to x = 8, with fire filling the row at z = -1. Only the z = 0 lane has
// a hazardous neighbour, so a route from one end of it to the other either
// hugs the fire or steps a lane over and pays two lateral walks instead.
func corridorBesideLava() *world.Blocks {
	blocks := flat(-1, -1, 9, 3)
	for x := int32(0); x <= 8; x++ {
		blocks.SetBlock(geom.BlockPos{X: x, Y: 0, Z: -1}, refFire, geom.EmptyShape())
	}

	return blocks
}

func TestHazardPenaltySteersAwayFromTheLavaEdge(t *testing.T) {
	timid := walker
	timid.HazardPenalty = 50

	path, err := Find(context.Background(), corridorBesideLava(), burningFacts{}, timid,
		geom.BlockPos{}, GoalBlock{Pos: geom.BlockPos{X: 8}}, wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}

	// The goal itself is beside the fire, so one penalised arrival is the
	// floor. Anything more means the route walked the rim rather than
	// stepping off it.
	beside := 0
	for _, edge := range path.Edges {
		if edge.To.Z == 0 {
			beside++
		}
	}
	if beside > 1 {
		t.Fatalf("route spent %d arrivals in the lane beside the fire, want 1 (the goal)", beside)
	}
}

func TestZeroPenaltyLeavesEveryCostAlone(t *testing.T) {
	path, err := Find(context.Background(), corridorBesideLava(), burningFacts{}, walker,
		geom.BlockPos{}, GoalBlock{Pos: geom.BlockPos{X: 8}}, wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("path incomplete, reason %v", path.Reason)
	}

	// walker.HazardPenalty is zero: the lane beside the fire is the straight
	// line and must still be taken at its plain cost.
	for _, edge := range path.Edges {
		if edge.To.Z != 0 {
			t.Fatalf("zero penalty still detoured via %v", edge.To)
		}
	}
}
