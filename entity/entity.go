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
	// Motion is the body's velocity, in blocks per tick.
	Motion geom.Vec3
	// OnGround is the standing state the last tick left behind.
	OnGround bool
	// StepHeight is how far the body may rise to clear an obstacle.
	//
	// Java Edition stores this as a float and widens it where the step-up
	// applies it, so a player's value is float64(float32(0.6)). A profile is
	// responsible for putting the widened value here; see the note on
	// collision.Move.StepHeight.
	StepHeight float64
}
