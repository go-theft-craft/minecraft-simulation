package collision_test

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/collision"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// player returns a 1.8.9-sized body standing with its feet at a position. The
// dimensions do not matter to collision; only that the box is not a cube, so a
// mistake that swapped an axis shows up.
func player(x, y, z float64) geom.AABB {
	return geom.AABB{
		MinX: x - 0.3, MinY: y, MinZ: z - 0.3,
		MaxX: x + 0.3, MaxY: y + 1.8, MaxZ: z + 0.3,
	}
}

// floored returns a view with a stone floor at y = 0 and air above, plus
// whatever else the caller places.
func floored(t *testing.T, place func(*world.Blocks)) *world.Blocks {
	t.Helper()

	blocks := world.NewBlocks()
	blocks.Fill(
		geom.BlockPos{X: -8, Y: -2, Z: -8},
		geom.BlockPos{X: 8, Y: 8, Z: 8},
		geom.EmptyShape(),
	)
	for x := int32(-8); x <= 8; x++ {
		for z := int32(-8); z <= 8; z++ {
			blocks.Set(geom.BlockPos{X: x, Y: 0, Z: z}, geom.FullCube())
		}
	}
	if place != nil {
		place(blocks)
	}

	return blocks
}

func TestTheAxisOrderFollowsTheMotion(t *testing.T) {
	// A body driven into the inside corner of two walls resolves the larger
	// horizontal axis first. Which one that is decides where it ends up, so the
	// two motions must not produce mirror-image answers.
	blocks := floored(t, func(blocks *world.Blocks) {
		blocks.Set(geom.BlockPos{X: 1, Y: 1, Z: 0}, geom.FullCube())
		blocks.Set(geom.BlockPos{X: 0, Y: 1, Z: 1}, geom.FullCube())
	})

	alongX, err := collision.ResolveVoxel(blocks, collision.Move{
		Body:   player(0.5, 1, 0.5),
		Motion: geom.Vec3{X: 0.6, Z: 0.2},
	})
	if err != nil {
		t.Fatalf("ResolveVoxel: %v", err)
	}
	alongZ, err := collision.ResolveVoxel(blocks, collision.Move{
		Body:   player(0.5, 1, 0.5),
		Motion: geom.Vec3{X: 0.2, Z: 0.6},
	})
	if err != nil {
		t.Fatalf("ResolveVoxel: %v", err)
	}

	// Both are blocked on their major axis by the wall in front of them.
	if alongX.Applied.X == 0.6 {
		t.Errorf("the X-major move was not blocked: %+v", alongX.Applied)
	}
	if alongZ.Applied.Z == 0.6 {
		t.Errorf("the Z-major move was not blocked: %+v", alongZ.Applied)
	}
}

func TestAMotionSmallerThanTheToleranceIsDiscarded(t *testing.T) {
	// The shape clamp reports zero rather than carrying a motion this small,
	// which is what stops a body pressed against a wall creeping by millionths.
	// The 1.8.9 resolve has no such rule.
	blocks := floored(t, func(blocks *world.Blocks) {
		blocks.Set(geom.BlockPos{X: 1, Y: 1, Z: 0}, geom.FullCube())
	})

	// Standing against the wall rather than near it: with nothing in reach the
	// game returns the motion untouched, and so does this. The rule is about
	// what the clamp does when it runs, so the body has to be somewhere it runs.
	got, err := collision.ResolveVoxel(blocks, collision.Move{
		Body:   player(0.7, 1, 0.5),
		Motion: geom.Vec3{X: 1e-9},
	})
	if err != nil {
		t.Fatalf("ResolveVoxel: %v", err)
	}
	if got.Applied.X != 0 {
		t.Fatalf("X = %v, want the sub-tolerance motion discarded", got.Applied.X)
	}
	// And it is not reported as a collision: the shortfall is far below the
	// horizontal tolerance.
	if got.CollidedX {
		t.Error("a motion too small to apply was reported as a collision")
	}
}

func TestAHorizontalShortfallUnderATenThousandthIsNotACollision(t *testing.T) {
	// The vertical flag compares exactly and the horizontal ones do not. A body
	// stopped a millionth short of its intent has collided in the arithmetic and
	// not in the flag, which is what the game reports.
	blocks := floored(t, nil)

	got, err := collision.ResolveVoxel(blocks, collision.Move{
		Body:   player(0.5, 1, 0.5),
		Motion: geom.Vec3{X: 0.25, Y: -0.5},
	})
	if err != nil {
		t.Fatalf("ResolveVoxel: %v", err)
	}

	if got.Applied.Y == -0.5 {
		t.Fatal("the floor did not stop the body")
	}
	if !got.CollidedY || !got.OnGround {
		t.Errorf("landing was not reported: %+v", got)
	}
	if got.CollidedX || got.CollidedZ {
		t.Errorf("an unobstructed horizontal axis was reported as collided: %+v", got)
	}
}

func TestAStepClimbsTheHeightTheObstacleOffers(t *testing.T) {
	// A slab offers a half-block rise, and a body walking into one takes it.
	blocks := floored(t, func(blocks *world.Blocks) {
		blocks.Set(geom.BlockPos{X: 1, Y: 1, Z: 0}, geom.NewShape(geom.AABB{MaxX: 1, MaxY: 0.5, MaxZ: 1}))
	})

	got, err := collision.ResolveVoxel(blocks, collision.Move{
		Body:       player(0.5, 1, 0.5),
		Motion:     geom.Vec3{X: 0.4},
		OnGround:   true,
		StepHeight: float64(float32(0.6)),
	})
	if err != nil {
		t.Fatalf("ResolveVoxel: %v", err)
	}

	if !got.Stepped {
		t.Fatalf("the body did not step onto the slab: %+v", got)
	}
	if got.Body.MinY <= 1 {
		t.Errorf("the body ended at %v, want it up on the slab", got.Body.MinY)
	}
	if got.Applied.X != 0.4 {
		t.Errorf("the step did not carry the full horizontal motion: %v", got.Applied.X)
	}
}

func TestAWallTooTallToClimbStopsTheBody(t *testing.T) {
	blocks := floored(t, func(blocks *world.Blocks) {
		blocks.Set(geom.BlockPos{X: 1, Y: 1, Z: 0}, geom.FullCube())
	})

	got, err := collision.ResolveVoxel(blocks, collision.Move{
		Body:       player(0.5, 1, 0.5),
		Motion:     geom.Vec3{X: 0.4},
		OnGround:   true,
		StepHeight: float64(float32(0.6)),
	})
	if err != nil {
		t.Fatalf("ResolveVoxel: %v", err)
	}

	if got.Stepped {
		t.Fatalf("the body climbed a whole block with a 0.6 step height: %+v", got)
	}
	if !got.CollidedX {
		t.Errorf("the wall was not reported: %+v", got)
	}
}

func TestAnUndescribedRegionStopsTheMoveRatherThanBeingTreatedAsAir(t *testing.T) {
	blocks := world.NewBlocks()

	got, err := collision.ResolveVoxel(blocks, collision.Move{
		Body:   player(0.5, 1, 0.5),
		Motion: geom.Vec3{X: 0.4},
	})
	if err != nil {
		t.Fatalf("ResolveVoxel: %v", err)
	}
	if len(got.Unknown) == 0 {
		t.Fatal("a move over cells nobody described reported no unknowns")
	}
	if got.Body != player(0.5, 1, 0.5) {
		t.Errorf("the body moved over an undescribed region: %+v", got.Body)
	}
}

func TestTheCandidateLimitReachesTheShapeResolve(t *testing.T) {
	blocks := floored(t, nil)

	_, err := collision.ResolveVoxel(blocks, collision.Move{
		Body:           player(0.5, 1, 0.5),
		Motion:         geom.Vec3{X: 40, Y: 40, Z: 40},
		CandidateLimit: 4,
	})
	if err == nil {
		t.Fatal("a sweep far past its budget returned no error")
	}
}

func TestTheShapeGridIsTheGameShape(t *testing.T) {
	// What a shape offers a step-up is its grid, not its faces. These are the
	// four cases the game distinguishes, and the oracle checks them against a
	// real server; this pins them where the jar is absent.
	cases := []struct {
		name  string
		shape geom.Shape
		want  []float64
	}{
		{"a full cube is its two faces", geom.FullCube(), []float64{0, 1}},
		{
			"a slab is halves",
			geom.NewShape(geom.AABB{MaxX: 1, MaxY: 0.5, MaxZ: 1}),
			[]float64{0, 0.5, 1},
		},
		{
			"a plate an eighth thick offers every eighth",
			geom.NewShape(geom.AABB{MaxX: 1, MaxY: 0.125, MaxZ: 1}),
			[]float64{0, 0.125, 0.25, 0.375, 0.5, 0.625, 0.75, 0.875, 1},
		},
		{
			"a box on no grid line keeps its own faces",
			geom.NewShape(geom.AABB{MinX: 0.0625, MaxX: 0.9375, MaxY: 0.3, MaxZ: 1}),
			[]float64{0, 0.3},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := test.shape.GridY()
			if len(got) != len(test.want) {
				t.Fatalf("GridY = %v, want %v", got, test.want)
			}
			for index := range got {
				if got[index] != test.want[index] {
					t.Fatalf("GridY = %v, want %v", got, test.want)
				}
			}
		})
	}
}

func TestAnEmptyShapeOffersNothing(t *testing.T) {
	if got := geom.EmptyShape().GridY(); len(got) != 0 {
		t.Fatalf("GridY = %v, want nothing", got)
	}
}
