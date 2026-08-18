package navigation

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
)

// The memo must answer exactly what the direct oracle answers, warm or cold.
// A cache that changes an answer is a bug, not an optimization.
func TestMemoAnswersAsTheDirectOracle(t *testing.T) {
	blocks := maze(seeds[0])
	direct := directOracle{query: walker.query(blocks, nil), capability: walker}
	memo := newMemoOracle(blocks, nil, walker)

	for x := int32(0); x <= 11; x++ {
		for z := int32(0); z <= 11; z++ {
			cell := geom.BlockPos{X: x, Y: 0, Z: z}

			want, err := direct.passable(cell)
			if err != nil {
				t.Fatalf("direct.passable returned an error: %v", err)
			}
			// Twice, so the second call is served from the cache.
			for range 2 {
				got, err := memo.passable(cell)
				if err != nil {
					t.Fatalf("memo.passable returned an error: %v", err)
				}
				if got != want {
					t.Fatalf("memo.passable(%v) = %v, want %v", cell, got, want)
				}
			}
		}
	}
}

func TestMemoServesTheSecondCallFromCache(t *testing.T) {
	blocks := flat(-1, -1, 1, 1)
	memo := newMemoOracle(blocks, nil, walker)
	cell := geom.BlockPos{X: 0, Y: 0, Z: 0}

	if _, err := memo.passable(cell); err != nil {
		t.Fatalf("passable returned an error: %v", err)
	}
	before := memo.misses

	if _, err := memo.passable(cell); err != nil {
		t.Fatalf("passable returned an error: %v", err)
	}
	if memo.misses != before {
		t.Fatalf("misses went from %d to %d; the second call was not cached", before, memo.misses)
	}
}

// Invalidating a cell the answer depended on must drop that answer, even though
// the cell is not the one the answer is keyed by. Passable(c) reads the ground
// under c, so changing the ground must invalidate c.
func TestMemoInvalidatesByDependency(t *testing.T) {
	blocks := flat(-1, -1, 1, 1)
	memo := newMemoOracle(blocks, nil, walker)
	cell := geom.BlockPos{X: 0, Y: 0, Z: 0}
	ground := geom.BlockPos{X: 0, Y: -1, Z: 0}

	first, err := memo.passable(cell)
	if err != nil {
		t.Fatalf("passable returned an error: %v", err)
	}
	if first != terrain.Clear {
		t.Fatalf("passable = %v, want Clear", first)
	}

	blocks.SetAir(ground)
	memo.invalidate([]geom.BlockPos{ground})

	second, err := memo.passable(cell)
	if err != nil {
		t.Fatalf("passable returned an error: %v", err)
	}
	if second != terrain.Blocked {
		t.Fatalf("passable = %v after the ground was removed, want Blocked", second)
	}
}

// The negative control: without the invalidate call the memo must return the
// stale answer. Without this, the test above would pass on a memo that cached
// nothing at all.
func TestMemoWithoutInvalidationIsStale(t *testing.T) {
	blocks := flat(-1, -1, 1, 1)
	memo := newMemoOracle(blocks, nil, walker)
	cell := geom.BlockPos{X: 0, Y: 0, Z: 0}

	if _, err := memo.passable(cell); err != nil {
		t.Fatalf("passable returned an error: %v", err)
	}

	blocks.SetAir(geom.BlockPos{X: 0, Y: -1, Z: 0})

	stale, err := memo.passable(cell)
	if err != nil {
		t.Fatalf("passable returned an error: %v", err)
	}
	if stale != terrain.Clear {
		t.Fatalf("passable = %v, want the stale Clear — the memo is not caching", stale)
	}
}

func TestMemoResetDropsEverything(t *testing.T) {
	blocks := flat(-1, -1, 1, 1)
	memo := newMemoOracle(blocks, nil, walker)
	cell := geom.BlockPos{X: 0, Y: 0, Z: 0}

	if _, err := memo.passable(cell); err != nil {
		t.Fatalf("passable returned an error: %v", err)
	}
	memo.reset()
	before := memo.misses

	if _, err := memo.passable(cell); err != nil {
		t.Fatalf("passable returned an error: %v", err)
	}
	if memo.misses == before {
		t.Fatal("the call after reset was served from cache")
	}
}

var _ oracle = (*memoOracle)(nil)
