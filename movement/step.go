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
	return collision.Resolve(view, collision.Move{
		Body:           state.Box,
		Motion:         state.Motion,
		OnGround:       state.OnGround,
		StepHeight:     state.StepHeight,
		CandidateLimit: limit,
	})
}
