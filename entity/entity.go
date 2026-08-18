// Package entity owns entity identity and the body state a simulation moves.
//
// A body is geometry and motion, not a game object: there is no health here, no
// inventory, and no behaviour. Rules live in a profile, and the kernel reads
// bodies through the views in this package.
package entity

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// ID identifies one entity.
//
// It is int32 because that is the width Java Edition assigns entity
// identifiers. Widening it here would let a store hold identifiers no server
// could send, and the mismatch would surface as a lost entity rather than as an
// error.
type ID int32

// Family is the physics family a body belongs to. Movement constants are per
// family, not per entity type: every dropped item falls the same way.
type Family uint8

const (
	// FamilyUnknown is a body whose family nobody has set. A profile treats it
	// as an error rather than guessing at constants for it.
	FamilyUnknown Family = iota
	// FamilyPlayer is a player body.
	FamilyPlayer
)

// String returns the family's name.
func (f Family) String() string {
	switch f {
	case FamilyUnknown:
		return "unknown"
	case FamilyPlayer:
		return "player"
	default:
		return fmt.Sprintf("Family(%d)", uint8(f))
	}
}

// State is one body at one instant.
//
// Every field is comparable, so a change set can hold a state by value and a
// store can tell whether an operation changed anything.
type State struct {
	// Family selects the movement constants a profile applies.
	Family Family
	// Box is the body's collision box in world space.
	Box geom.AABB
	// Position is the point the body stands at, when its version keeps one.
	//
	// The two versions this module carries disagree about which of these is the
	// original. Java 1.8.9 moves the box and derives the position from it, so a
	// 1.8.9 profile leaves this zero and nothing reads it. Java 26.1.2 moves the
	// position and rebuilds the box from it and the body's dimensions, and the
	// two orders do not round the same way: a box offset by a motion differs in
	// its last bits from a box rebuilt around a position that was. A profile
	// whose version derives the box carries the position here so that it can
	// rebuild rather than offset.
	Position geom.Vec3
	// Motion is the body's velocity, in blocks per tick.
	Motion geom.Vec3
	// OnGround is the standing state the last tick left behind.
	OnGround bool
	// Support is what the last move recorded about the block holding the body
	// up, for a version that keeps such a record.
	//
	// Java 1.8.9 keeps none and leaves this zero. Java 26.1.2 records it during
	// every move and reads it on the next tick to decide which block's friction
	// applies, so a body that walks off the edge of a slab keeps the slab's
	// column for the tick that leaves it. Deriving it afresh from the body's own
	// box cannot reproduce that: the record survives a tick in which nothing is
	// under the body at all.
	Support Support
	// StepHeight is how far the body may rise to clear an obstacle.
	//
	// Java Edition stores this as a float and widens it where the step-up
	// applies it, so a player's value is float64(float32(0.6)). A profile is
	// responsible for putting the widened value here; see the note on
	// collision.Move.StepHeight.
	StepHeight float64
}

// Support is what a move recorded about the block a body stands on.
//
// The three fields are one rule between them. Present says a block was found
// under the body; Block names it; and NoBlocks remembers that the last look
// found nothing, which is what decides whether the next one may fall back to
// probing where the body came from. A body that is not on the ground carries
// none of it.
type Support struct {
	// Block is the cell the move found under the body.
	Block geom.BlockPos
	// Present says Block means anything.
	Present bool
	// NoBlocks says the last look found nothing under the body.
	NoBlocks bool
}
