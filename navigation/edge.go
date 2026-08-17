package navigation

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// EdgeKind names one way of getting from one cell to the next.
//
// The design also specifies JumpGap, Dig, Place, Support, and Collapse. They
// are absent rather than stubbed: each needs work that has not landed, and a
// kind that exists but is never produced is a kind a caller will switch on and
// be wrong about.
type EdgeKind uint8

const (
	// EdgeWalk crosses to an adjacent cell on the level.
	EdgeWalk EdgeKind = iota
	// EdgeStep rises one block into an adjacent cell.
	EdgeStep
	// EdgeFall descends into an adjacent column within the body's safe fall.
	EdgeFall
	// EdgeSwim crosses to an adjacent cell through fluid.
	EdgeSwim
)

// String returns the kind's name.
func (e EdgeKind) String() string {
	switch e {
	case EdgeWalk:
		return "walk"
	case EdgeStep:
		return "step"
	case EdgeFall:
		return "fall"
	case EdgeSwim:
		return "swim"
	default:
		return fmt.Sprintf("EdgeKind(%d)", uint8(e))
	}
}

// Edge is one move.
type Edge struct {
	// Kind names the move.
	Kind EdgeKind
	// From is the cell the body leaves.
	From geom.BlockPos
	// To is the cell the body arrives in.
	To geom.BlockPos
	// Posture is how the body occupies To on arrival.
	Posture Posture
	// Cost is the move's price in ticks.
	Cost float64
}

// Reason says why a search stopped.
type Reason uint8

const (
	// ReasonFound means the goal was reached.
	ReasonFound Reason = iota
	// ReasonBudget means the node budget ran out.
	ReasonBudget
	// ReasonCeiling means every remaining route costs more than the ceiling.
	ReasonCeiling
	// ReasonUnreachable means the frontier emptied without reaching the goal.
	ReasonUnreachable
)

// String returns the reason's name.
func (r Reason) String() string {
	switch r {
	case ReasonFound:
		return "found"
	case ReasonBudget:
		return "budget"
	case ReasonCeiling:
		return "ceiling"
	case ReasonUnreachable:
		return "unreachable"
	default:
		return fmt.Sprintf("Reason(%d)", uint8(r))
	}
}

// Path is a route, complete or not.
//
// An incomplete path is returned rather than an error because a body that
// travels most of the way and searches again beats one that refuses to move.
// Complete says which it is holding; Reason says why.
type Path struct {
	// Edges are the moves in order. It is empty when the body is already at
	// the goal and when nothing was reachable.
	Edges []Edge
	// Cost is the sum of the edge costs, in ticks.
	Cost float64
	// Complete reports that Edges reach the goal.
	Complete bool
	// Reason says why the search stopped.
	Reason Reason
}

// End returns the cell the path arrives at, and the cell the body started in
// when the path is empty.
func (p Path) End(start geom.BlockPos) geom.BlockPos {
	if len(p.Edges) == 0 {
		return start
	}

	return p.Edges[len(p.Edges)-1].To
}
