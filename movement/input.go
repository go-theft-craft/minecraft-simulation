// Package movement implements the movement rules a profile's phases call.
//
// Nothing here knows a game version. Every constant a rule needs arrives as an
// argument, and the profile that supplies it owns the width that constant is
// computed at. That division is the reason this package can be shared between
// 1.8.9 and 26.1.2 while the two disagree on almost every number.
//
// Arithmetic in this package is float32 where the game computes in float, and
// float64 where the game computes in double. A signature that takes float32 is
// not a mistake or an optimization: it is the width the product is formed at,
// and widening it early changes the result in its last bits. See the profile's
// documentation for where the widening happens.
package movement

import "github.com/go-theft-craft/minecraft-simulation/entity"

// Locomotion is the movement state that persists between ticks.
//
// It is separate from entity.State because none of it is geometry, and
// entity.State is deliberately geometry and motion alone so that it stays
// comparable. Every field here is a scalar for the same reason: a change set
// carries this by value and the digest encodes it.
type Locomotion struct {
	// JumpTicks counts down the delay before a body may jump again. A jump sets
	// it to ten and releasing the jump input zeroes it.
	JumpTicks int32
	// Yaw is the horizontal facing in degrees.
	//
	// It is float32 because the game stores it as a float and because it indexes
	// the sine table: holding it wider and narrowing per read would be a second
	// chance to pick a different table entry.
	Yaw float32
	// Pitch is the vertical facing in degrees. Land movement does not read it;
	// it is carried because the body has one and a consumer reports it.
	Pitch float32
	// Sprinting, Sneaking, and Jumping are the input flags as of the last tick.
	Sprinting bool
	Sneaking  bool
	Jumping   bool
	// MoveSpeed is the movement-speed attribute, which the ground speed is the
	// product of. The game reads it from an attribute map rather than a field.
	MoveSpeed float32
	// JumpFactor is the airborne movement factor, 0.02 for a player with no
	// modifiers.
	JumpFactor float32
}

// Input is what a controller asked for on one tick.
//
// It is a sim.Command: a semantic intent with no packet in it. The kernel hands
// it to the phases, which decide whether the body's state permits it.
type Input struct {
	// Entity is the body the intent is for.
	//
	// A command carries its subject because a tick may simulate several bodies:
	// a server steps every player it holds from one tick's worth of queued
	// input, and an intent that did not name its body could only ever be applied
	// to one of them.
	Entity entity.ID
	// Strafe and Forward are the raw input axes, before the decay a tick applies
	// to them. Positive forward is the direction the body faces.
	Strafe  float32
	Forward float32
	// Yaw and Pitch are the facing this tick.
	Yaw   float32
	Pitch float32
	// Jump, Sprint, and Sneak are held-this-tick, not pressed-this-tick. The
	// game reads them as states, and the jump counter is what turns a held key
	// into one jump.
	Jump   bool
	Sprint bool
	Sneak  bool
}

// CommandKind implements sim.Command.
func (Input) CommandKind() string { return "movement.input" }

// LocomotionView is everything the kernel reads about locomotion in one tick.
//
// It is separate from entity.View because a consumer may hold bodies without
// holding locomotion: a client tracks every entity a server tells it about, and
// simulates one.
type LocomotionView interface {
	// Locomotion returns the state for a body, or false when the view holds
	// none. Absent is not the same as zero: a zero Locomotion is a body with no
	// movement speed, which would stand still forever.
	Locomotion(id entity.ID) (Locomotion, bool)
}
