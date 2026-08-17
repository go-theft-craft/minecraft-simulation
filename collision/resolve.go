package collision

import (
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Move is one entity's intended motion for one tick.
type Move struct {
	// Body is the entity's box before the move.
	Body geom.AABB
	// Motion is the intended displacement.
	Motion geom.Vec3
	// OnGround is the entity's standing state entering the tick. It is one of
	// the two conditions that allow a step-up.
	OnGround bool
	// StepHeight is how far the entity may rise to clear an obstacle. Zero
	// disables stepping.
	StepHeight float64
	// CandidateLimit bounds how many cells the sweep may visit. Zero means no
	// limit.
	CandidateLimit int
}

// Result is the outcome of a move.
type Result struct {
	// Body is the box after the move.
	Body geom.AABB
	// Applied is the displacement that actually happened.
	Applied geom.Vec3
	// CollidedX, CollidedY, and CollidedZ report which axes were clamped.
	CollidedX bool
	CollidedY bool
	CollidedZ bool
	// OnGround is true when downward motion was clamped.
	OnGround bool
	// Stepped is true when the body rose over an obstacle.
	Stepped bool
	// Unknown holds the cells the view could not answer for. When it is
	// non-empty the move did not happen and every other field is the input
	// state.
	Unknown []geom.BlockPos
}

// CollidedHorizontally reports whether either horizontal axis was clamped.
func (r Result) CollidedHorizontally() bool {
	return r.CollidedX || r.CollidedZ
}

// Resolve applies move against view one axis at a time.
//
// The axis order is Y, then X, then Z, and the body is translated after each
// axis, so the X pass tests a body that has already moved vertically. That
// order is vanilla's and is observable: it decides whether an entity rising
// into a ledge is stopped by the ledge or slides under it.
func Resolve(view world.BlockView, move Move) (Result, error) {
	candidates, err := Gather(view, move.Body.Stretch(move.Motion), move.CandidateLimit)
	if err != nil {
		return Result{}, err
	}
	if len(candidates.Unknown) != 0 {
		return Result{Body: move.Body, Unknown: candidates.Unknown}, nil
	}

	body := move.Body
	applied := move.Motion

	for _, box := range candidates.Boxes {
		applied.Y = box.ClampY(body, applied.Y)
	}
	body = body.Offset(geom.Vec3{Y: applied.Y})

	for _, box := range candidates.Boxes {
		applied.X = box.ClampX(body, applied.X)
	}
	body = body.Offset(geom.Vec3{X: applied.X})

	for _, box := range candidates.Boxes {
		applied.Z = box.ClampZ(body, applied.Z)
	}
	body = body.Offset(geom.Vec3{Z: applied.Z})

	return Result{
		Body:      body,
		Applied:   applied,
		CollidedX: applied.X != move.Motion.X,
		CollidedY: applied.Y != move.Motion.Y,
		CollidedZ: applied.Z != move.Motion.Z,
		OnGround:  move.Motion.Y < 0 && applied.Y != move.Motion.Y,
	}, nil
}
