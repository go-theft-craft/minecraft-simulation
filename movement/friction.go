package movement

import "github.com/go-theft-craft/minecraft-simulation/geom"

// airFriction is the horizontal multiplier an airborne body gets, in place of
// the block's slipperiness.
const airFriction float32 = 0.91

// accelerationNumerator is the constant a ground body's acceleration divides by
// the cube of its friction.
const accelerationNumerator float32 = 0.16277136

// GroundFrictionBlock returns the cell whose slipperiness applies to a body.
//
// The vertical coordinate comes from the box and the horizontal ones from the
// position, which is why both are arguments: the game reads the cell one below
// the box's floor, at the position's own column, and a function given only one of
// the two would have to guess at the other.
func GroundFrictionBlock(box geom.AABB, pos geom.Vec3) geom.BlockPos {
	return geom.BlockPos{
		X: geom.Floor(pos.X),
		Y: geom.Floor(box.MinY) - 1,
		Z: geom.Floor(pos.Z),
	}
}

// Friction returns the horizontal multiplier for one tick.
//
// An airborne body gets the air constant whatever is beneath it. A grounded body
// gets the block's slipperiness times that constant, and the product is formed at
// single width: computing it as a double and narrowing afterwards gives a
// different number in its last bits, and the difference compounds over a hundred
// ticks into a position a server will correct.
func Friction(slipperiness float32, onGround bool) float32 {
	if !onGround {
		return airFriction
	}

	return slipperiness * airFriction
}

// Acceleration returns the ground acceleration for a friction.
//
// The division by the cube is what makes ice accelerate slowly and stop slowly:
// a higher slipperiness raises the friction, which lowers the acceleration, and
// the body reaches its speed over more ticks.
func Acceleration(friction float32) float32 {
	return accelerationNumerator / (friction * friction * friction)
}

// Speed returns the input scale for one tick.
//
// On the ground it is the movement-speed attribute times the acceleration.
// Airborne it is the jump movement factor alone, which is why a body in the air
// steers so much more slowly than one on the ground.
func Speed(friction float32, onGround bool, moveSpeed, jumpFactor float32) float32 {
	if !onGround {
		return jumpFactor
	}

	return moveSpeed * Acceleration(friction)
}
