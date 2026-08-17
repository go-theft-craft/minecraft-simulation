// Package navigation searches a route through a world and reports it as typed
// edges.
//
// The route is a value rather than a hidden state machine. A caller can print
// it, test it, and compare it against a recording, which a navigator that only
// answered "what do I press this tick" could not.
//
// Every cost is in ticks. Break time is in ticks and movement is in ticks, so
// a version that adds digging can compare "mine through" against "walk around"
// in one unit rather than through a weighting nobody can justify.
//
// Nothing here imports sim. A rule that needs a version's number receives it
// on Capability, which is what lets 1.8.9 and 26.1.2 share this search.
package navigation

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/terrain"
)

// Posture is how a body occupies a position.
//
// Two postures at one position are distinct nodes, because they differ in the
// box the body needs and in which edges leave them.
type Posture uint8

const (
	// PostureStand is a body standing on ground.
	PostureStand Posture = iota
	// PostureSwim is a body in a fluid it can swim.
	PostureSwim
)

// String returns the posture's name.
func (p Posture) String() string {
	switch p {
	case PostureStand:
		return "stand"
	case PostureSwim:
		return "swim"
	default:
		return fmt.Sprintf("Posture(%d)", uint8(p))
	}
}

// Capability is what one body can do and what each thing costs it.
//
// A mob is this value with CanSwim false; it gets a ground navigator out of
// the same search. Every duration is in ticks and every one is supplied by the
// caller, because 1.8.9 and 26.1.2 disagree about all of them.
type Capability struct {
	// Body is the box the search routes.
	Body terrain.Body
	// SafeFall is how far the body drops without harm, in blocks.
	SafeFall float64
	// CanSwim allows swim edges.
	CanSwim bool
	// WalkTicks is the cost of crossing one block on the level.
	WalkTicks float64
	// StepTicks is the cost of rising one block.
	StepTicks float64
	// FallTicks is the cost of descending one block.
	FallTicks float64
	// SwimTicks is the cost of crossing one block in fluid.
	SwimTicks float64
	// CandidateLimit bounds one terrain query's collision sweep. A
	// non-positive value means no limit.
	CandidateLimit int
}

// cheapest returns the lowest cost of any edge this capability may take. The
// heuristic scales distance by it, and a heuristic that assumed a higher floor
// would overestimate and stop returning shortest paths.
func (c Capability) cheapest() float64 {
	lowest := c.WalkTicks
	for _, cost := range []float64{c.StepTicks, c.FallTicks} {
		if cost < lowest {
			lowest = cost
		}
	}
	if c.CanSwim && c.SwimTicks < lowest {
		lowest = c.SwimTicks
	}

	return lowest
}
