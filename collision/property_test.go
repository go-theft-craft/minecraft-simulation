package collision

import (
	"math/rand/v2"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// seeds are fixed so that a failure reproduces exactly. Add to this list
// rather than randomizing it.
var seeds = []uint64{1, 2, 3, 5, 8, 13, 21, 34, 55, 89}

// solidWorld returns a view whose every cell in the working area is a full
// cube except a hollow corridor the body starts inside.
func solidWorld() *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -6, Y: -6, Z: -6}, geom.BlockPos{X: 6, Y: 6, Z: 6}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -1, Y: 0, Z: -1}, geom.BlockPos{X: 1, Y: 2, Z: 1}, geom.EmptyShape())

	return blocks
}

// TestSweptMotionNeverTunnels is the first exit property: however large the
// motion, the resolved body never ends up overlapping a solid block.
func TestSweptMotionNeverTunnels(t *testing.T) {
	blocks := solidWorld()
	body := geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}

	for _, seed := range seeds {
		random := rand.New(rand.NewPCG(seed, 0))
		for step := 0; step < 100; step++ {
			// Kept inside the described region: a sweep that leaves it
			// reports unknown cells and asserts nothing, so a wider range
			// would just buy slower tests that check less.
			motion := geom.Vec3{
				X: (random.Float64() - 0.5) * 8,
				Y: (random.Float64() - 0.5) * 8,
				Z: (random.Float64() - 0.5) * 8,
			}

			got, err := Resolve(blocks, Move{Body: body, Motion: motion, StepHeight: playerStepHeight})
			if err != nil {
				t.Fatalf("seed %d step %d: Resolve: %v", seed, step, err)
			}
			if len(got.Unknown) != 0 {
				continue // The sweep left the described area; nothing to assert.
			}

			candidates, err := Gather(blocks, got.Body, 0)
			if err != nil {
				t.Fatalf("seed %d step %d: Gather: %v", seed, step, err)
			}
			for _, box := range candidates.Boxes {
				if box.Intersects(got.Body) {
					t.Fatalf("seed %d step %d: motion %+v tunnelled into %+v; body ended at %+v",
						seed, step, motion, box, got.Body)
				}
			}
		}
	}
}

// TestStepUpRespectsItsBound is the second exit property: a body never rises
// by more than its step height in a single resolve.
func TestStepUpRespectsItsBound(t *testing.T) {
	stepHeight := playerStepHeight

	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -6, Y: -1, Z: -6}, geom.BlockPos{X: 6, Y: -1, Z: 6}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -6, Y: 0, Z: -6}, geom.BlockPos{X: 6, Y: 6, Z: 6}, geom.EmptyShape())
	// Slabs along +X, which a 0.6 step height can actually climb. A staircase
	// of full cubes would prove nothing here: the entity could never step at
	// all, and the bound would hold trivially.
	blocks.Fill(geom.BlockPos{X: 1, Y: 0, Z: -6}, geom.BlockPos{X: 4, Y: 0, Z: 6}, slab())

	body := geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}
	for _, seed := range seeds {
		random := rand.New(rand.NewPCG(seed, 0))
		for step := 0; step < 100; step++ {
			motion := geom.Vec3{
				X: random.Float64() * 2,
				Y: (random.Float64() - 0.75) * 2,
				Z: (random.Float64() - 0.5) * 2,
			}

			got, err := Resolve(blocks, Move{
				Body:       body,
				Motion:     motion,
				OnGround:   true,
				StepHeight: stepHeight,
			})
			if err != nil {
				t.Fatalf("seed %d step %d: Resolve: %v", seed, step, err)
			}
			if len(got.Unknown) != 0 {
				continue
			}
			if rise := got.Body.MinY - body.MinY; rise > stepHeight {
				t.Fatalf("seed %d step %d: rose %v with a step height of %v, motion %+v",
					seed, step, rise, stepHeight, motion)
			}
		}
	}
}

// TestZeroMotionIsAFixedPoint is the third exit property: resolving no motion
// changes nothing, from any starting position, however many times it runs.
func TestZeroMotionIsAFixedPoint(t *testing.T) {
	blocks := solidWorld()

	for _, seed := range seeds {
		random := rand.New(rand.NewPCG(seed, 0))
		for step := 0; step < 100; step++ {
			origin := geom.Vec3{
				X: (random.Float64() - 0.5) * 4,
				Y: random.Float64() * 2,
				Z: (random.Float64() - 0.5) * 4,
			}
			body := geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}.
				Offset(origin)

			got, err := Resolve(blocks, Move{Body: body, OnGround: true, StepHeight: playerStepHeight})
			if err != nil {
				t.Fatalf("seed %d step %d: Resolve: %v", seed, step, err)
			}
			if len(got.Unknown) != 0 {
				continue
			}
			if got.Body != body {
				t.Fatalf("seed %d step %d: zero motion moved the body from %+v to %+v",
					seed, step, body, got.Body)
			}
			if !got.Applied.IsZero() {
				t.Fatalf("seed %d step %d: zero motion applied %+v", seed, step, got.Applied)
			}
			if got.Stepped {
				t.Fatalf("seed %d step %d: zero motion stepped", seed, step)
			}
		}
	}
}

// TestResolveIsDeterministic runs the same move twice and requires identical
// results, including the order of reported unknown cells.
func TestResolveIsDeterministic(t *testing.T) {
	blocks := solidWorld()
	body := geom.AABB{MinX: -0.3, MinY: 0, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3}
	move := Move{Body: body, Motion: geom.Vec3{X: 3.5, Y: -2.25, Z: 1.75}, StepHeight: playerStepHeight}

	first, err := Resolve(blocks, move)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	second, err := Resolve(blocks, move)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if first.Body != second.Body || first.Applied != second.Applied {
		t.Fatalf("Resolve is not deterministic: %+v vs %+v", first, second)
	}
	if len(first.Unknown) != len(second.Unknown) {
		t.Fatalf("Unknown lengths differ: %d vs %d", len(first.Unknown), len(second.Unknown))
	}
	for index := range first.Unknown {
		if first.Unknown[index] != second.Unknown[index] {
			t.Fatalf("Unknown order differs at %d", index)
		}
	}
}
