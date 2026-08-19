package navigation

import (
	"context"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// The property every cache in this design rests on: where Plan runs the
// concrete search it must return byte-identically what Find returns.
func TestPlanEqualsFind(t *testing.T) {
	for _, seed := range seeds {
		blocks := maze(seed)
		from := geom.BlockPos{X: 0, Y: 0, Z: 0}
		goal := geom.BlockPos{X: 11, Y: 0, Z: 11}
		budget := Budget{Nodes: 5_000, Ceiling: 5_000}

		want, err := Find(context.Background(), blocks, nil, walker, from, GoalBlock{Pos: goal}, budget)
		if err != nil {
			t.Fatalf("seed %d: Find returned an error: %v", seed, err)
		}

		planner, err := NewPlanner(blocks, nil, walker, Options{})
		if err != nil {
			t.Fatalf("seed %d: NewPlanner returned an error: %v", seed, err)
		}

		// Cold, then warm, then warm after a different route has filled the
		// cache with answers this route did not ask for.
		for run := range 3 {
			if run == 2 {
				if _, err := planner.Plan(context.Background(), goal, GoalBlock{Pos: from}, budget); err != nil {
					t.Fatalf("seed %d: warming Plan returned an error: %v", seed, err)
				}
			}

			got, err := planner.Plan(context.Background(), from, GoalBlock{Pos: goal}, budget)
			if err != nil {
				t.Fatalf("seed %d run %d: Plan returned an error: %v", seed, run, err)
			}
			assertSamePath(t, seed, run, got, want)
		}
	}
}

// assertSamePath compares two paths field by field.
func assertSamePath(t *testing.T, seed uint64, run int, got, want Path) {
	t.Helper()

	if got.Complete != want.Complete || got.Reason != want.Reason || got.Cost != want.Cost {
		t.Fatalf("seed %d run %d: summary %v/%v/%v, want %v/%v/%v",
			seed, run, got.Complete, got.Reason, got.Cost, want.Complete, want.Reason, want.Cost)
	}
	if len(got.Edges) != len(want.Edges) {
		t.Fatalf("seed %d run %d: %d edges, want %d", seed, run, len(got.Edges), len(want.Edges))
	}
	for i := range want.Edges {
		if got.Edges[i] != want.Edges[i] {
			t.Fatalf("seed %d run %d: edge %d = %+v, want %+v", seed, run, i, got.Edges[i], want.Edges[i])
		}
	}
}

// Observe must make a stale plan correct again, matching what a fresh Find says
// about the changed world.
func TestObserveRestoresCorrectness(t *testing.T) {
	blocks := flat(-1, -1, 5, 1)
	from := geom.BlockPos{X: 0, Y: 0, Z: 0}
	goal := geom.BlockPos{X: 4, Y: 0, Z: 0}

	planner, err := NewPlanner(blocks, nil, walker, Options{})
	if err != nil {
		t.Fatalf("NewPlanner returned an error: %v", err)
	}
	if _, err := planner.Plan(context.Background(), from, GoalBlock{Pos: goal}, wideBudget); err != nil {
		t.Fatalf("Plan returned an error: %v", err)
	}

	// Wall off the straight line at x=2, z=0.
	changed := []geom.BlockPos{{X: 2, Y: 0, Z: 0}, {X: 2, Y: 1, Z: 0}}
	for _, cell := range changed {
		blocks.Set(cell, geom.FullCube())
	}
	planner.Observe(changed)

	got, err := planner.Plan(context.Background(), from, GoalBlock{Pos: goal}, wideBudget)
	if err != nil {
		t.Fatalf("Plan returned an error: %v", err)
	}
	want, err := Find(context.Background(), blocks, nil, walker, from, GoalBlock{Pos: goal}, wideBudget)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	assertSamePath(t, 0, 0, got, want)

	for _, edge := range got.Edges {
		if edge.To == (geom.BlockPos{X: 2, Y: 0, Z: 0}) {
			t.Fatal("the replanned route walks through the new wall")
		}
	}
}

// The negative control. Without the Observe call the plan must stay stale —
// otherwise the test above would pass on a planner that caches nothing.
func TestWithoutObserveThePlanIsStale(t *testing.T) {
	blocks := flat(-1, -1, 5, 1)
	from := geom.BlockPos{X: 0, Y: 0, Z: 0}
	goal := geom.BlockPos{X: 4, Y: 0, Z: 0}

	planner, err := NewPlanner(blocks, nil, walker, Options{})
	if err != nil {
		t.Fatalf("NewPlanner returned an error: %v", err)
	}
	if _, err := planner.Plan(context.Background(), from, GoalBlock{Pos: goal}, wideBudget); err != nil {
		t.Fatalf("Plan returned an error: %v", err)
	}

	for _, cell := range []geom.BlockPos{{X: 2, Y: 0, Z: 0}, {X: 2, Y: 1, Z: 0}} {
		blocks.Set(cell, geom.FullCube())
	}
	// Deliberately no Observe call — that is what this test is about.

	stale, err := planner.Plan(context.Background(), from, GoalBlock{Pos: goal}, wideBudget)
	if err != nil {
		t.Fatalf("Plan returned an error: %v", err)
	}

	var through bool
	for _, edge := range stale.Edges {
		if edge.To == (geom.BlockPos{X: 2, Y: 0, Z: 0}) {
			through = true
		}
	}
	if !through {
		t.Fatal("the un-Observed plan avoided the new wall; the planner is not caching")
	}
}

func TestNewPlannerRefusesABodilessCapability(t *testing.T) {
	if _, err := NewPlanner(flat(-1, -1, 1, 1), nil, Capability{}, Options{}); err == nil {
		t.Fatal("NewPlanner accepted a capability with no body")
	}
}

// Reset must return the planner to a cold cache.
func TestResetDropsTheCache(t *testing.T) {
	blocks := flat(-1, -1, 5, 1)
	planner, err := NewPlanner(blocks, nil, walker, Options{})
	if err != nil {
		t.Fatalf("NewPlanner returned an error: %v", err)
	}
	from := geom.BlockPos{X: 0, Y: 0, Z: 0}
	goal := geom.BlockPos{X: 4, Y: 0, Z: 0}

	if _, err := planner.Plan(context.Background(), from, GoalBlock{Pos: goal}, wideBudget); err != nil {
		t.Fatalf("Plan returned an error: %v", err)
	}
	planner.Reset()
	before := planner.memo.misses

	if _, err := planner.Plan(context.Background(), from, GoalBlock{Pos: goal}, wideBudget); err != nil {
		t.Fatalf("Plan returned an error: %v", err)
	}
	if planner.memo.misses == before {
		t.Fatal("the plan after Reset was served from cache")
	}
}
