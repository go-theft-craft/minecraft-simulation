package geom

import "math"

// Package-level rather than methods: none of these is a question about one
// vector. Behind takes a target and a facing, Lead takes a target and a
// velocity, Tangent takes a centre and a position, and Away takes a position
// and a threat. Hanging them off a receiver would make one of the two arguments
// look privileged when neither is.

// Behind returns the position distance blocks behind a target, given the
// direction it faces.
//
// The facing's magnitude is ignored, so a caller may hand over a velocity, a
// look vector, or a difference of positions without normalising first. A facing
// of zero returns the target unchanged: a body that faces nowhere has no behind,
// and picking a direction for it would be inventing one.
func Behind(target, facing Vec3, distance float64) Vec3 {
	length := math.Hypot(facing.X, facing.Z)
	if length == 0 {
		return target
	}

	return Vec3{
		X: target.X - facing.X/length*distance,
		Y: target.Y,
		Z: target.Z - facing.Z/length*distance,
	}
}

// Lead returns where a mover will be in ticks, at its current velocity.
//
// The velocity is the caller's. Nothing here tracks entities, and a leading
// shot aimed from a velocity this package guessed would be a guess wearing
// arithmetic.
//
// It is a straight projection: no drag, no gravity, no collision. Over the few
// ticks a swing or an arrow is aimed across, those corrections are smaller than
// the target's own next decision, and a caller that needs them has the movement
// kernel to run.
func Lead(target, velocity Vec3, ticks float64) Vec3 {
	return target.Add(velocity.Scale(ticks))
}

// Tangent returns the unit heading that circles a point, so a caller strafes
// without materialising a ring of waypoints.
//
// It is horizontal. Circling through the vertical is orbiting, which is not
// something a body standing on the ground does.
//
// At the centre it returns the zero vector rather than dividing by zero. There
// is no tangent to a circle of radius nothing, and a caller reading a zero
// heading as "hold still" is right.
func Tangent(centre, here Vec3, clockwise bool) Vec3 {
	dx, dz := here.X-centre.X, here.Z-centre.Z

	length := math.Hypot(dx, dz)
	if length == 0 {
		return Vec3{}
	}

	// Rotating the radius a quarter turn in the horizontal plane. Which
	// quarter turn is which direction follows the game's yaw convention, where
	// the angle runs from +Z toward -X.
	if clockwise {
		return Vec3{X: -dz / length, Z: dx / length}
	}

	return Vec3{X: dz / length, Z: -dx / length}
}

// Away returns a point distance blocks from here, directly away from a threat.
//
// It aims at a point rather than returning a direction because that is what an
// actuator takes, and it aims the full distance rather than one step: the step
// is clamped on the way out, and a target that moved one step at a time would
// need recomputing against a threat that has also moved.
func Away(here, threat Vec3, distance float64) Vec3 {
	dx, dz := here.X-threat.X, here.Z-threat.Z

	length := math.Hypot(dx, dz)
	if length == 0 {
		// Standing exactly on it, which a mob that has walked into a body
		// manages. Any direction beats returning here and standing still while
		// something hits it, so this picks one.
		dx, dz, length = 1, 0, 1
	}

	return Vec3{
		X: here.X + dx/length*distance,
		Y: here.Y,
		Z: here.Z + dz/length*distance,
	}
}
