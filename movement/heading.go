package movement

import (
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// inputThreshold is the squared input magnitude below which a tick applies no
// heading at all. The comparison is strict, as the game's is: an input exactly at
// the threshold does move the body.
const inputThreshold float32 = 1e-4

// degreesToRadians is the pre-divided conversion, narrowed once. The jump
// impulse uses it, because the game writes that conversion as a single constant
// there.
const degreesToRadians float32 = math.Pi / 180

// pi is the game's float pi. The heading multiplies by it and divides by 180 in
// two float steps rather than by a pre-divided constant, because that is how the
// game writes that expression — and the two disagree for some angles.
//
// The oracle found the difference. A body walking with a yaw of about 656
// degrees drifted four millionths of a block per tick along one axis and matched
// exactly along the other, which is the signature of one table entry being read
// instead of its neighbour.
const pi float32 = math.Pi

// radians converts a yaw the way the heading's own expression does: multiply,
// then divide, both at single width.
func radians(yaw float32) float32 { return yaw * pi / 180 }

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

	angle := radians(yaw)
	sin := table.Sin(angle)
	cos := table.Cos(angle)

	motion.X += float64(strafe*cos - forward*sin)
	motion.Z += float64(forward*cos + strafe*sin)

	return motion
}
