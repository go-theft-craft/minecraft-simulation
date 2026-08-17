package movement

import "github.com/go-theft-craft/minecraft-simulation/geom"

// jumpDelay is what a jump sets the counter to. It is why holding the jump key
// does not jump every tick.
const jumpDelay int32 = 10

// sprintJumpImpulse is the horizontal boost a sprinting jump adds, in the
// direction the body faces.
const sprintJumpImpulse float32 = 0.2

// Countdown decrements the jump delay toward zero.
//
// It is a phase of its own even though it reads nothing else, because the tick
// has this step and a phase list that skipped it would stop describing the tick.
func Countdown(state Locomotion) Locomotion {
	if state.JumpTicks > 0 {
		state.JumpTicks--
	}

	return state
}

// Jump applies the jump impulse when this tick may jump, and reports whether it
// did.
//
// The conditions are all three of: the jump input held, the body on the ground,
// and the counter at zero. Reporting the outcome is what lets the phase set the
// counter without restating the conditions and risking them drifting apart.
//
// A body that is not holding jump has its counter zeroed rather than left to run
// down, which is what makes tapping the key jump immediately.
func Jump(
	table Table,
	state Locomotion,
	motion geom.Vec3,
	onGround bool,
	upwards float32,
) (Locomotion, geom.Vec3, bool) {
	if !state.Jumping {
		state.JumpTicks = 0

		return state, motion, false
	}
	if !onGround || state.JumpTicks > 0 {
		return state, motion, false
	}

	motion.Y = float64(upwards)
	if state.Sprinting {
		// The impulse reads the table rather than computing a sine, for the same
		// reason every other angle in this package does.
		radians := state.Yaw * degreesToRadians
		motion.X -= float64(table.Sin(radians) * sprintJumpImpulse)
		motion.Z += float64(table.Cos(radians) * sprintJumpImpulse)
	}
	state.JumpTicks = jumpDelay

	return state, motion, true
}
