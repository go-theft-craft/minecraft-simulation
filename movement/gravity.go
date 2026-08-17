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
// The drag arrives as a float32 because the game's constant is a float literal,
// and the product is formed at double width because the motion is a double and
// Java widens the float to meet it. Narrowing the motion to single width first
// is a different number: the oracle caught it on the first tick it compared, on
// a body that was doing nothing but standing still.
//
// This is the shape of every product in this package that mixes the two widths.
// A float constant times a double motion is a double product. Only products
// whose operands are all floats in the game are formed at single width.
func ApplyVerticalDrag(motion geom.Vec3, drag float32) geom.Vec3 {
	motion.Y *= float64(drag)

	return motion
}

// ApplyHorizontalDrag multiplies both horizontal components by the tick's
// friction.
//
// The friction is the one computed before the body moved. A player who walks off
// ice keeps ice friction for the tick that leaves it, and recomputing here from
// the post-move position would take that away.
//
// The friction is a float and the motion is a double, so the product is a double
// one, as in ApplyVerticalDrag.
func ApplyHorizontalDrag(motion geom.Vec3, friction float32) geom.Vec3 {
	motion.X *= float64(friction)
	motion.Z *= float64(friction)

	return motion
}
