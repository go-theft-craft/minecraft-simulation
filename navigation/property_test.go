package navigation

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// seeds are fixed so a failure reproduces exactly. Add to this list rather
// than randomizing it, matching collision/property_test.go.
var seeds = []uint64{1, 2, 3, 5, 8, 13, 21, 34, 55, 89}

// terrainClear is spelled out because the property loops compare against it
// repeatedly and the qualified name buries the assertion.
const terrainClear = terrain.Clear

// maze returns a flat world with pillars knocked into it, deterministic for a
// seed. The start and goal cells are always left open.
func maze(seed uint64) *world.Blocks {
	blocks := flat(-1, -1, 12, 12)
	random := rand.New(rand.NewPCG(seed, 0))

	for x := int32(0); x <= 11; x++ {
		for z := int32(0); z <= 11; z++ {
			if x == 0 && z == 0 {
				continue
			}
			if x == 11 && z == 11 {
				continue
			}
			if random.Float64() < 0.25 {
				blocks.Set(geom.BlockPos{X: x, Y: 0, Z: z}, geom.FullCube())
				blocks.Set(geom.BlockPos{X: x, Y: 1, Z: z}, geom.FullCube())
			}
		}
	}

	return blocks
}

// TestPathsAreContiguous is the first exit property: every edge leaves where
// the previous one arrived. A search that lost a parent link returns a path
// that teleports, and a caller following it walks into a wall.
func TestPathsAreContiguous(t *testing.T) {
	for _, seed := range seeds {
		path := search(t, maze(seed))
		for i := 1; i < len(path.Edges); i++ {
			if path.Edges[i].From != path.Edges[i-1].To {
				t.Fatalf(
					"seed %d: edge %d leaves %v but edge %d arrived at %v",
					seed, i, path.Edges[i].From, i-1, path.Edges[i-1].To,
				)
			}
		}
	}
}

// TestPathsStartAtTheOrigin guards the partial-path case: a bounded search
// must still hand back something the caller can start walking.
func TestPathsStartAtTheOrigin(t *testing.T) {
	for _, seed := range seeds {
		path := search(t, maze(seed))
		if len(path.Edges) == 0 {
			continue
		}
		if path.Edges[0].From != (geom.BlockPos{X: 0, Y: 0, Z: 0}) {
			t.Fatalf("seed %d: path starts at %v, want the origin", seed, path.Edges[0].From)
		}
	}
}

// TestPathCostIsTheSumOfItsEdges guards against a cost that drifts from the
// route it describes, which would make one path compare wrongly against
// another.
func TestPathCostIsTheSumOfItsEdges(t *testing.T) {
	for _, seed := range seeds {
		path := search(t, maze(seed))
		if !path.Complete {
			continue
		}

		var total float64
		for _, edge := range path.Edges {
			total += edge.Cost
		}
		if total != path.Cost {
			t.Fatalf("seed %d: Cost = %v, edges sum to %v", seed, path.Cost, total)
		}
	}
}

// TestEveryEdgeLandsSomewhereStandable checks the property a caller actually
// depends on: each arrival is a cell the body can occupy.
func TestEveryEdgeLandsSomewhereStandable(t *testing.T) {
	for _, seed := range seeds {
		blocks := maze(seed)
		path := search(t, blocks)
		query := walker.query(blocks, nil)

		for i, edge := range path.Edges {
			passable, err := query.Passable(edge.To)
			if err != nil {
				t.Fatalf("seed %d: Passable returned an error: %v", seed, err)
			}
			if passable != terrainClear {
				t.Fatalf("seed %d: edge %d lands on a %v cell", seed, i, passable)
			}
		}
	}
}

// TestSearchesAreReproducible is the determinism gate. Go randomizes map
// iteration, so a search that let map order reach an output would return a
// different path on some runs and fail the replay comparison for a reason
// nothing in the recording explains.
func TestSearchesAreReproducible(t *testing.T) {
	for _, seed := range seeds {
		blocks := maze(seed)
		first := search(t, blocks)

		for run := 0; run < 100; run++ {
			again := search(t, blocks)
			if again.Cost != first.Cost || again.Reason != first.Reason {
				t.Fatalf("seed %d run %d: path summary changed", seed, run)
			}
			if len(again.Edges) != len(first.Edges) {
				t.Fatalf("seed %d run %d: edge count changed", seed, run)
			}
			for i := range first.Edges {
				if again.Edges[i] != first.Edges[i] {
					t.Fatalf("seed %d run %d: edge %d changed", seed, run, i)
				}
			}
		}
	}
}

// search runs one fixed query against a world.
func search(t *testing.T, blocks *world.Blocks) Path {
	t.Helper()

	path, err := Find(
		context.Background(), blocks, nil, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 11, Y: 0, Z: 11},
		Budget{Nodes: 5_000, Ceiling: 5_000},
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}

	return path
}
