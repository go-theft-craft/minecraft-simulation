package movement

import "github.com/go-theft-craft/minecraft-simulation/geom"

// ApplyGravity subtracts one tick of gravity from the vertical motion.
//
// The gravity argument is a float64 while the drags beside it are float32, and
// the asymmetry is the point rather than an oversight: the game writes gravity as
// a double literal and the drags as float literals. That is why the dataset
// records a player's gravity as exactly 0.08 and its vertical drag as
// 0.9800000190734863.
func ApplyGravity(motion geom.Vec3, gravity float64) geom.Vec3 {
	motion.Y -= gravity

	return motion
}

// ApplyVerticalDrag multiplies the vertical motion by one tick of drag.
//
// The product is formed at single width and widened once, which is what makes a
// long fall reach the same terminal speed the game reaches.
func ApplyVerticalDrag(motion geom.Vec3, drag float32) geom.Vec3 {
	motion.Y = float64(float32(motion.Y) * drag)

	return motion
}

// ApplyHorizontalDrag multiplies both horizontal components by the tick's
// friction.
//
// The friction is the one computed before the body moved. A player who walks off
// ice keeps ice friction for the tick that leaves it, and recomputing here from
// the post-move position would take that away.
func ApplyHorizontalDrag(motion geom.Vec3, friction float32) geom.Vec3 {
	motion.X = float64(float32(motion.X) * friction)
	motion.Z = float64(float32(motion.Z) * friction)

	return motion
}
