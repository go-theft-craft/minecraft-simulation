package movement

import (
	"errors"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/collision"
	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// floor describes a stone plane at y = 0 with air above it, over a region wide
// enough for the moves below.
func floor(t *testing.T) *world.Blocks {
	t.Helper()

	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -8, Y: 0, Z: -8}, geom.BlockPos{X: 8, Y: 0, Z: 8}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -8, Y: 1, Z: -8}, geom.BlockPos{X: 8, Y: 8, Z: 8}, geom.EmptyShape())

	return blocks
}

// standing returns a player body resting on the floor.
func standing(motion geom.Vec3) entity.State {
	return entity.State{
		Family: entity.FamilyPlayer,
		Box: geom.AABB{
			MinX: -0.3, MinY: 1, MinZ: -0.3,
			MaxX: 0.3, MaxY: 2.8, MaxZ: 0.3,
		},
		Motion:     motion,
		OnGround:   true,
		StepHeight: float64(float32(0.6)),
	}
}

func TestAFreeMoveAppliesInFull(t *testing.T) {
	got, err := Step(floor(t), standing(geom.Vec3{X: 0.1}), 0)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if got.Applied.X != 0.1 {
		t.Fatalf("Applied.X = %v, want the whole motion 0.1", got.Applied.X)
	}
	if got.CollidedX {
		t.Error("a free move reported a horizontal collision")
	}
}

func TestAnUnknownRegionSurfacesInTheResultRatherThanAsAnError(t *testing.T) {
	// The tick is incomplete, not broken. The phase that calls this is the one
	// that has to tell the kernel, which is why the cells come back in the result.
	blocks := world.NewBlocks()

	got, err := Step(blocks, standing(geom.Vec3{X: 0.1}), 0)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if len(got.Unknown) == 0 {
		t.Fatal("a sweep over an undescribed region reported no unknown cells")
	}
	if got.Applied != (geom.Vec3{}) {
		t.Fatalf("an incomplete sweep applied %+v, want nothing", got.Applied)
	}
}

func TestTheCandidateBudgetReachesCollision(t *testing.T) {
	// A huge motion over a described region needs many cells; a budget of one
	// cannot pay for them, and the failure has to come back rather than be
	// silently truncated into a shorter move.
	state := standing(geom.Vec3{X: 500, Z: 500})

	if _, err := Step(floor(t), state, 1); !errors.Is(err, collision.ErrCandidateLimit) {
		t.Fatalf("Step error = %v, want ErrCandidateLimit", err)
	}
}

func TestStepDoesNotAdjustWhatCollisionReported(t *testing.T) {
	// Step is an adapter. Comparing it against a direct Resolve with the same
	// move is what keeps it one: a future "fix" applied here would show up as a
	// difference from the package it wraps.
	blocks := floor(t)
	state := standing(geom.Vec3{X: 0.2, Y: -0.5, Z: 0.1})

	direct, err := collision.Resolve(blocks, collision.Move{
		Body:       state.Box,
		Motion:     state.Motion,
		OnGround:   state.OnGround,
		StepHeight: state.StepHeight,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	got, err := Step(blocks, state, 0)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got.Body != direct.Body || got.Applied != direct.Applied {
		t.Fatalf("Step returned %+v, want the untouched %+v", got, direct)
	}
	if got.OnGround != direct.OnGround || got.Stepped != direct.Stepped {
		t.Fatalf("Step changed the flags: %+v vs %+v", got, direct)
	}
}
