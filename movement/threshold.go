package movement

import (
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// ClampSmallMotion zeroes any component of a body's motion smaller than a
// threshold.
//
// It is why a walking body stops the tick after its input does rather than
// creeping for a dozen ticks while the drag grinds its motion down, and why a
// body barely pressed against a wall reports no motion at all. The threshold is
// a double in the game and the comparison is on the magnitude, so a motion of
// exactly the threshold survives.
//
// The oracle found this rule. Neither the sequencing design nor the M8.4 plan
// described the tick as having it, and every scenario that walked at an angle
// disagreed with the game within four ticks: the component nearest zero is the
// one the game had already discarded.
func ClampSmallMotion(motion geom.Vec3, threshold float64) geom.Vec3 {
	if math.Abs(motion.X) < threshold {
		motion.X = 0
	}
	if math.Abs(motion.Y) < threshold {
		motion.Y = 0
	}
	if math.Abs(motion.Z) < threshold {
		motion.Z = 0
	}

	return motion
}
