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

	body, applied := applyAxes(candidates.Boxes, move.Body, move.Body, move.Motion)

	stepped := false
	verticallyBlocked := move.Motion.Y < 0 && applied.Y != move.Motion.Y
	horizontallyBlocked := applied.X != move.Motion.X || applied.Z != move.Motion.Z
	if move.StepHeight > 0 && (move.OnGround || verticallyBlocked) && horizontallyBlocked {
		stepBoxes, err := Gather(
			view,
			move.Body.Stretch(geom.Vec3{X: move.Motion.X, Y: move.StepHeight, Z: move.Motion.Z}),
			move.CandidateLimit,
		)
		if err != nil {
			return Result{}, err
		}
		if len(stepBoxes.Unknown) != 0 {
			return Result{Body: move.Body, Unknown: stepBoxes.Unknown}, nil
		}

		body, applied, stepped = stepUp(stepBoxes.Boxes, move.Body, move.Motion, move.StepHeight, applied)
	}

	return Result{
		Body:      body,
		Applied:   applied,
		CollidedX: applied.X != move.Motion.X,
		CollidedY: applied.Y != move.Motion.Y,
		CollidedZ: applied.Z != move.Motion.Z,
		OnGround:  move.Motion.Y < 0 && applied.Y != move.Motion.Y || stepped,
		Stepped:   stepped,
	}, nil
}

// applyAxes runs the Y, X, Z passes against boxes and returns the moved body
// with the motion that survived.
//
// yProbe is the box the Y clamp tests. An ordinary move passes the body
// itself; step-up passes a horizontally stretched body for one of its two
// attempts, which is the only way the two differ.
func applyAxes(boxes []geom.AABB, body, yProbe geom.AABB, motion geom.Vec3) (geom.AABB, geom.Vec3) {
	for _, box := range boxes {
		motion.Y = box.ClampY(yProbe, motion.Y)
	}
	body = body.Offset(geom.Vec3{Y: motion.Y})

	for _, box := range boxes {
		motion.X = box.ClampX(body, motion.X)
	}
	body = body.Offset(geom.Vec3{X: motion.X})

	for _, box := range boxes {
		motion.Z = box.ClampZ(body, motion.Z)
	}
	body = body.Offset(geom.Vec3{Z: motion.Z})

	return body, motion
}

// stepUp retries a blocked horizontal move by rising up to height.
//
// Both attempts start from the pre-move body and rise by the full step height.
// They differ only in the box their Y clamp tests: the first tests a body
// stretched horizontally by the motion, so it rises for a block it is about to
// move over; the second tests the plain body, so it rises only for what is
// directly above. Neither wins in every geometry, so vanilla computes both and
// keeps the one that travels further horizontally. A tie goes to the second.
//
// The winner settles downward onto whatever it climbed, and only then is it
// weighed against the unstepped result. The bool reports whether it won.
func stepUp(boxes []geom.AABB, body geom.AABB, motion geom.Vec3, height float64, unstepped geom.Vec3) (geom.AABB, geom.Vec3, bool) {
	horizontal := geom.Vec3{X: motion.X, Z: motion.Z}

	rise := geom.Vec3{X: motion.X, Y: height, Z: motion.Z}
	firstBody, firstMotion := applyAxes(boxes, body, body.Stretch(horizontal), rise)
	secondBody, secondMotion := applyAxes(boxes, body, body, rise)

	winnerBody, winnerMotion := secondBody, secondMotion
	if firstMotion.HorizontalLengthSquared() > secondMotion.HorizontalLengthSquared() {
		winnerBody, winnerMotion = firstBody, firstMotion
	}

	// Settle onto whatever was climbed before judging the attempt: a rise that
	// gained height but no ground must lose to the plain move.
	settle := -winnerMotion.Y
	for _, box := range boxes {
		settle = box.ClampY(winnerBody, settle)
	}
	winnerBody = winnerBody.Offset(geom.Vec3{Y: settle})
	winnerMotion.Y += settle

	if unstepped.HorizontalLengthSquared() >= winnerMotion.HorizontalLengthSquared() {
		return body.Offset(unstepped), unstepped, false
	}

	return winnerBody, winnerMotion, true
}
