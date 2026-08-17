package movement

import (
	"github.com/go-theft-craft/minecraft-simulation/collision"
	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Step resolves a body's motion against the world.
//
// It is a thin adapter over collision.Resolve and it must stay one. The M8.2
// oracle established that collision.Result reports what the game reports —
// including a step-up recording only the settle as its vertical motion, which
// reads like a bug and is not — so a caller that "corrected" the result here
// would reintroduce exactly the defect that oracle caught.
//
// An unknown region is not an error. It arrives as a non-empty Unknown in the
// result, and the phase that called this must forward those cells to the tick's
// missing set, because the kernel cannot see inside a collision result.
func Step(view world.View, state entity.State, limit int) (collision.Result, error) {
	return StepWith(collision.Resolve, view, state, limit)
}

// Resolver is the collision algorithm a profile's move phase resolves through.
//
// It exists because the algorithm is a version fact, not a constant. Java
// Edition reworked collision around voxel shapes, and the result is a different
// algorithm rather than the same one with different numbers: the axis order
// depends on the motion, every comparison works to a tolerance, and the step-up
// tries whatever heights the obstacle offers instead of a fixed two. A profile
// picks the one its version plays by.
type Resolver func(view world.BlockView, move collision.Move) (collision.Result, error)

// StepWith resolves a body's motion through a chosen collision algorithm.
//
// It is the same thin adapter Step is, and it must stay one for the same reason:
// a caller that "corrected" a result here would reintroduce the defect M8.2's
// oracle caught.
func StepWith(
	resolve Resolver, view world.View, state entity.State, limit int,
) (collision.Result, error) {
	return resolve(view, collision.Move{
		Body:           state.Box,
		Motion:         state.Motion,
		OnGround:       state.OnGround,
		StepHeight:     state.StepHeight,
		CandidateLimit: limit,
	})
}
