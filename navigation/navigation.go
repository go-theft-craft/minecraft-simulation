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
	// PostureFall is a body in the air, holding no ground.
	//
	// It is a posture rather than a flag because the edges leaving it are
	// different: a body already airborne cannot start a jump, which is the
	// constraint this encodes and the reason the jump expansion refuses it.
	//
	// Nothing in this package produces it yet. The edge that does is Pillar,
	// in the mutating-edge amendment, where the body is airborne at the moment
	// it places the block under itself — that amendment names this posture as
	// its one dependency on this one. It is defined here rather than there so
	// that the posture set is settled before an edge needs it, and so the jump
	// guard is written once.
	PostureFall
)

// Postures are appended, never inserted, for the reason EdgeKind values are: a
// posture's number reaches a recorded path.

// String returns the posture's name.
func (p Posture) String() string {
	switch p {
	case PostureStand:
		return "stand"
	case PostureSwim:
		return "swim"
	case PostureFall:
		return "fall"
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
	// JumpTicks is the cost of crossing one block by jump, in ticks.
	JumpTicks float64
	// JumpReach is how far the body's jump carries it horizontally, in blocks.
	// Zero produces no jump edges, which is how a mob keeps getting a ground
	// navigator out of the same search.
	//
	// It comes from navigation/reach, which measures it by running the
	// profile's own movement kernel over a flat world. A hand-written value
	// here is a number this repository cannot verify, and the 2026-08-17
	// navigation plan deferred this edge rather than ship one.
	//
	// It is the distance between cell centres the arc covers, so a jump to the
	// cell two away needs a reach of two. Measured from a standing start both
	// supported profiles clear a little over two, which crosses a one-block
	// hole; a body already running clears more, and a caller that measured its
	// own running start may say so.
	JumpReach float64
	// JumpRise is how high the arc's peak is above the take-off, in blocks.
	// The clearance check needs it: a jump passes over the cells between, and
	// what it passes through is what stops it.
	JumpRise float64
	// CandidateLimit bounds one terrain query's collision sweep. A
	// non-positive value means no limit.
	CandidateLimit int
}

// perBlockFloor returns the lowest cost this capability can pay for one block
// of Manhattan distance closed.
//
// It is deliberately not the cheapest edge. A step closes two blocks — one
// across, one up — for one step's cost, and a fall of depth D closes 1+D blocks
// for FallTicks*D, which is cheapest per block at D=1. Scaling distance by the
// cheapest edge instead overestimates on both, and an overestimating heuristic
// lets the search settle a goal on a route that is not shortest.
//
// Every edge the capability can produce has to enter this, which is why adding
// one means coming back here. A jump of distance D costs JumpTicks*D and closes
// D blocks, so its rate is JumpTicks flat, and a capability whose jump is
// cheaper per block than its walk would otherwise be searched with a heuristic
// that overestimates every route containing one.
func (c Capability) perBlockFloor() float64 {
	lowest := c.WalkTicks
	for _, cost := range []float64{c.StepTicks / 2, c.FallTicks / 2} {
		if cost < lowest {
			lowest = cost
		}
	}
	if c.CanSwim && c.SwimTicks < lowest {
		lowest = c.SwimTicks
	}
	if c.JumpReach > 0 && c.JumpTicks < lowest {
		lowest = c.JumpTicks
	}

	return lowest
}

// query returns the terrain query this capability asks with.
func (c Capability) query(view terrainView, facts terrain.Facts) terrain.Query {
	return terrain.Query{View: view, Facts: facts, Body: c.Body, Limit: c.CandidateLimit}
}
