package navigation

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// terrainView is the view a search reads. It is world.View under another name
// so that this package's signatures say what they need rather than repeating
// the composite everywhere.
type terrainView = world.View

// EdgeKind names one way of getting from one cell to the next.
//
// The design also specifies Support and Collapse. They are absent rather than
// stubbed: each needs a number a milestone still owes — placement legality and
// a falling-column trace — and a kind that exists but is never produced is a
// kind a caller will switch on and be wrong about. Dig was absent for the same
// reason until M9.4 measured break times against both jars; it is here now,
// and it is produced only for a body whose Breaker can say how long a block
// takes. A body that mines at a plausible-looking wrong speed is worse than
// one that refuses to mine.
//
// Values are appended, never inserted. A kind's number reaches a recorded path,
// so renumbering one would make every recording taken before the change
// disagree with the build for a reason nothing in it explains.
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
	// EdgeJumpGap crosses a gap the body's jump arc clears.
	//
	// It is the one edge that reaches past an adjacent cell. Without it
	// nothing in the vocabulary crosses a hole at all: EdgeStep rises into an
	// adjacent cell and EdgeFall descends into an adjacent column, so a body
	// standing at the edge of a two-block hole had no edge reaching the far
	// side and every route around every gap was a detour.
	EdgeJumpGap
	// EdgeWaterDrop descends further than the body's safe fall, into water
	// deep enough to break the landing.
	//
	// EdgeFall is bounded by SafeFall and has no way to say that a drop is
	// survivable because of what is at the bottom of it rather than because of
	// how far it is.
	EdgeWaterDrop
	// EdgeClimb moves one cell vertically inside a climbable column, in either
	// direction.
	EdgeClimb
	// EdgeDoor passes through a door the body opens on the way.
	//
	// The door is the edge's To cell, so a caller reading a path knows which
	// one to work without the edge carrying a second position.
	EdgeDoor
	// EdgePlace bridges into a cell with nothing under it by putting a block
	// there first.
	//
	// The block goes into the cell below the edge's To, which is why the edge
	// carries no second position: a caller reading a path knows where the
	// block goes from where the body ends up.
	EdgePlace
	// EdgePillar rises one block by placing a block into the cell the body is
	// standing in, while the body is above it.
	//
	// It is not a special case of EdgePlace. Place puts a block into a cell the
	// body will walk across; this one puts it into the cell the body just left,
	// and the body arrives one block higher. The preconditions differ, the
	// resulting node differs, and the failure modes differ.
	//
	// The block goes into the edge's From cell.
	EdgePillar
	// EdgeDig enters a blocked cell by breaking what is in the way.
	//
	// It clears the cells the body would occupy standing in To — its whole
	// span, because a two-block body walking into a filled column needs both
	// of them gone. Like the placing edges, the cells are derived from the
	// destination and the body rather than carried: a caller reading a path
	// knows which cells were broken from where the body ended up and how tall
	// it is.
	//
	// It is produced only for a destination the body could not otherwise
	// enter, so it never competes with a plain walk over the same cell.
	EdgeDig
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
	case EdgeJumpGap:
		return "jump-gap"
	case EdgeWaterDrop:
		return "water-drop"
	case EdgeClimb:
		return "climb"
	case EdgeDoor:
		return "door"
	case EdgePlace:
		return "place"
	case EdgePillar:
		return "pillar"
	case EdgeDig:
		return "dig"
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
