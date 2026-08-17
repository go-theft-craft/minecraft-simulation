package movement

import (
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// inputThreshold is the squared input magnitude below which a tick applies no
// heading at all. The comparison is strict, as the game's is: an input exactly at
// the threshold does move the body.
const inputThreshold float32 = 1e-4

// degreesToRadians is the conversion the heading applies to yaw before reading
// the table. It is float32 because the game's multiply is a float multiply.
const degreesToRadians float32 = math.Pi / 180

// ApplyHeading adds one tick's input to a body's motion.
//
// This is the most width-sensitive rule in the module, and its order of
// operations is fixed:
//
//  1. The squared magnitude is formed at single width, and an input below the
//     threshold returns the motion untouched.
//  2. The square root is taken at double width and narrowed, which is what the
//     game does and what makes it portable: an IEEE square root is exactly
//     rounded, so every platform agrees on it.
//  3. A magnitude below one becomes one, so that a light touch is not normalized
//     up to a full stride. This is why walking slowly is possible at all.
//  4. The scale, the two products, and each bracket are formed at single width.
//     Only the finished bracket is widened, once, and added to the double-width
//     motion.
//
// Widening earlier than step four changes the result in its last bits, and a
// hundred ticks of that is a position a server corrects.
func ApplyHeading(table Table, motion geom.Vec3, strafe, forward, speed, yaw float32) geom.Vec3 {
	magnitude := strafe*strafe + forward*forward
	if magnitude < inputThreshold {
		return motion
	}

	magnitude = float32(math.Sqrt(float64(magnitude)))
	if magnitude < 1 {
		magnitude = 1
	}

	scale := speed / magnitude
	strafe *= scale
	forward *= scale

	radians := yaw * degreesToRadians
	sin := table.Sin(radians)
	cos := table.Cos(radians)

	motion.X += float64(strafe*cos - forward*sin)
	motion.Z += float64(forward*cos + strafe*sin)

	return motion
}
