package navigation

import (
	"context"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// benchBudget is generous enough that no benchmark measures budget exhaustion.
var benchBudget = Budget{Nodes: 200_000, Ceiling: 200_000}

// corridor returns a flat world spanning the given half-extent in x and z.
func corridor(extent int32) *world.Blocks {
	return flat(-extent, -extent, extent, extent)
}

func BenchmarkFindShort(b *testing.B) {
	blocks := corridor(8)
	goal := geom.BlockPos{X: 4, Y: 0, Z: 0}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := Find(context.Background(), blocks, nil, walker,
			geom.BlockPos{X: 0, Y: 0, Z: 0}, goal, benchBudget); err != nil {
			b.Fatalf("Find returned an error: %v", err)
		}
	}
}

func BenchmarkFindLong(b *testing.B) {
	blocks := corridor(64)
	goal := geom.BlockPos{X: 60, Y: 0, Z: 0}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := Find(context.Background(), blocks, nil, walker,
			geom.BlockPos{X: 0, Y: 0, Z: 0}, goal, benchBudget); err != nil {
			b.Fatalf("Find returned an error: %v", err)
		}
	}
}

// BenchmarkFindMaze uses the property suite's obstacle fixture, so the
// benchmark measures a search that actually branches rather than one walking a
// straight line.
func BenchmarkFindMaze(b *testing.B) {
	blocks := maze(seeds[0])
	goal := geom.BlockPos{X: 11, Y: 0, Z: 11}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := Find(context.Background(), blocks, nil, walker,
			geom.BlockPos{X: 0, Y: 0, Z: 0}, goal, benchBudget); err != nil {
			b.Fatalf("Find returned an error: %v", err)
		}
	}
}

func BenchmarkPlanCold(b *testing.B) {
	blocks := corridor(64)
	goal := geom.BlockPos{X: 60, Y: 0, Z: 0}

	b.ReportAllocs()
	for b.Loop() {
		planner, err := NewPlanner(blocks, nil, walker, Options{})
		if err != nil {
			b.Fatalf("NewPlanner returned an error: %v", err)
		}
		if _, err := planner.Plan(context.Background(),
			geom.BlockPos{X: 0, Y: 0, Z: 0}, goal, benchBudget); err != nil {
			b.Fatalf("Plan returned an error: %v", err)
		}
	}
}

func BenchmarkPlanWarm(b *testing.B) {
	blocks := corridor(64)
	goal := geom.BlockPos{X: 60, Y: 0, Z: 0}
	from := geom.BlockPos{X: 0, Y: 0, Z: 0}

	planner, err := NewPlanner(blocks, nil, walker, Options{})
	if err != nil {
		b.Fatalf("NewPlanner returned an error: %v", err)
	}
	if _, err := planner.Plan(context.Background(), from, goal, benchBudget); err != nil {
		b.Fatalf("warming Plan returned an error: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := planner.Plan(context.Background(), from, goal, benchBudget); err != nil {
			b.Fatalf("Plan returned an error: %v", err)
		}
	}
}

// The replanning case: one block changes, the planner is told, and it routes
// again. This is what a follower does when the world moves under it.
func BenchmarkPlanAfterChange(b *testing.B) {
	blocks := corridor(64)
	goal := geom.BlockPos{X: 60, Y: 0, Z: 0}
	from := geom.BlockPos{X: 0, Y: 0, Z: 0}
	changed := []geom.BlockPos{{X: 30, Y: 0, Z: 1}}

	planner, err := NewPlanner(blocks, nil, walker, Options{})
	if err != nil {
		b.Fatalf("NewPlanner returned an error: %v", err)
	}
	if _, err := planner.Plan(context.Background(), from, goal, benchBudget); err != nil {
		b.Fatalf("warming Plan returned an error: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		planner.Observe(changed)
		if _, err := planner.Plan(context.Background(), from, goal, benchBudget); err != nil {
			b.Fatalf("Plan returned an error: %v", err)
		}
	}
}
