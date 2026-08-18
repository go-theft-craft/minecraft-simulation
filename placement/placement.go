// Package placement decides where a block placement lands and whether it may
// happen.
//
// It answers the two questions that fail differently. *Where* is arithmetic on
// a cell and a face, and it is the same on every version. *May it* is a
// predicate over the target cell, the block already there, and the bodies
// standing in the way, and it is nearly the same on every version. What is not
// here is *what the block becomes*: a placed block's state depends on the face
// clicked, where the player stood, and where the player looked, and the two
// editions address a state so differently that the rule belongs to each
// version's own profile behind Placer.
//
// Nothing here reads a registry. Whether the clicked block is replaceable and
// what shape the placed block has are both version facts, and both arrive as
// arguments — which is what makes this package a table of cases rather than a
// second copy of a version's block data.
package placement

import (
	"math"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mining"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Target is where a placement will actually land.
//
// The clicked cell and the placed cell are usually different: clicking the top
// of a stone block places into the cell above it. They are the same when the
// clicked block is replaceable — grass, snow, water — because those are
// replaced in place. Getting this backwards puts every placement one cell off,
// which reads as an aim bug and is not one.
type Target struct {
	// Clicked is the cell the player's cursor was on.
	Clicked geom.BlockPos
	// Placed is where the block goes.
	Placed geom.BlockPos
	// Replacing reports that the placement replaces the clicked block rather
	// than sitting against its face.
	Replacing bool
}

// Resolve decides which cell a click lands in.
//
// replaceable is supplied rather than looked up because which blocks are
// replaceable is version data, and this package holds no version data.
//
// The face is mining.Face rather than a fourth spelling of the same six
// numbers. A dig and a placement name the same sides of the same cell with the
// same wire numbering, and a package with its own copy of that enum is a
// package that can disagree with the one the wire uses.
func Resolve(clicked geom.BlockPos, face mining.Face, replaceable bool) Target {
	if replaceable {
		return Target{Clicked: clicked, Placed: clicked, Replacing: true}
	}

	return Target{Clicked: clicked, Placed: offsetBy(clicked, face)}
}

// offsetBy steps one cell along a face's outward normal.
func offsetBy(pos geom.BlockPos, face mining.Face) geom.BlockPos {
	switch face {
	case mining.FaceBottom:
		pos.Y--
	case mining.FaceTop:
		pos.Y++
	case mining.FaceNorth:
		pos.Z--
	case mining.FaceSouth:
		pos.Z++
	case mining.FaceWest:
		pos.X--
	case mining.FaceEast:
		pos.X++
	}

	return pos
}

// Legality is why a placement may or may not happen.
//
// It carries a reason on refusal rather than a bare false, because a caller
// that cannot distinguish "a mob is standing there" from "that cell is out of
// reach" cannot do anything useful about either.
type Legality struct {
	Allowed bool
	Reason  string
}

// The reasons a placement is refused, as constants so a test asserts on one
// rather than on prose.
const (
	// ReasonOccupied is a target cell that already holds something.
	ReasonOccupied = "the target cell holds a non-replaceable block"
	// ReasonEntity is a body standing where the block would be.
	ReasonEntity = "an entity's collision box overlaps the placed block"
	// ReasonOutOfReach is a target further away than the player can touch.
	ReasonOutOfReach = "the target is beyond the player's reach"
)

// Replaceable reports whether the block a handle names may be placed into.
//
// It is a function rather than a bool on the target because the cell it is
// asked about is the one Check resolves: a caller that answered in advance
// would have to read the world a second time to know which cell to answer for,
// and the two reads could disagree. Which blocks are replaceable is version
// data, and a version's profile is what supplies this.
type Replaceable func(ref world.BlockRef) bool

// Check decides whether a placement is legal.
//
// shape is the placed block's collision shape, supplied by the profile, and the
// entity check runs against that shape rather than against a cube. A bottom
// slab under a standing mob is legal and a full block is not, and a check that
// assumed a cube would refuse both.
//
// A cell nobody has described is not a cell that refuses a placement. The
// answer is incomplete, and the caller is expected to load the named cell and
// ask again — guessing air here places blocks into walls, which is what the
// three-way view exists to prevent.
//
// Reach is measured from the eye to the nearest point of the placed cell, not
// to its centre. The game measures to the block, and a centre-to-eye distance
// refuses a placement a player can make at the edge of their reach.
func Check(
	view world.View, entities entity.View, target Target, replaceable Replaceable,
	shape geom.Shape, eye geom.Vec3, reach float64,
) (Legality, sim.Completeness) {
	if distance := distanceToCell(eye, target.Placed); distance > reach {
		return refused(ReasonOutOfReach), sim.Completeness{Complete: true}
	}

	// Occupancy is asked of the state view and answered by the version, not
	// inferred from the collision shape. Both editions place into air, into
	// water, and into tall grass, and refuse everything else — and none of
	// those three is a question about a shape.
	//
	// The known limit: a view that reports air for a shapeless block it does
	// hold — a torch, a lever — allows a placement the game refuses, because
	// the tri-state lookup is about collision rather than about the block.
	// world.Blocks is such a view, which is why the predicate exists: a caller
	// with a state view that mints a handle for every block passes one that
	// answers for the handle, and gets the game's answer.
	ref, lookup := view.BlockState(target.Placed)
	if lookup == world.LookupUnknown {
		return Legality{}, incomplete(target.Placed)
	}
	if lookup != world.LookupAir && !replaces(replaceable, ref) {
		return refused(ReasonOccupied), sim.Completeness{Complete: true}
	}

	if overlapsAnEntity(entities, target.Placed, shape) {
		return refused(ReasonEntity), sim.Completeness{Complete: true}
	}

	return Legality{Allowed: true}, sim.Completeness{Complete: true}
}

// replaces asks the version whether a handle may be placed into, treating an
// absent predicate as "only air is replaceable" rather than as "everything is".
// A caller that supplies none is a caller that has not said, and guessing
// permissively here places blocks into walls.
func replaces(replaceable Replaceable, ref world.BlockRef) bool {
	return replaceable != nil && replaceable(ref)
}

// overlapsAnEntity reports whether any body's box intersects the placed shape.
//
// The boxes come from geom.Shape.BoxesAt, which is what the collision path
// builds its candidates from, so a shape that does not collide with a body
// during a move does not refuse a placement either.
func overlapsAnEntity(entities entity.View, pos geom.BlockPos, shape geom.Shape) bool {
	if entities == nil || shape.IsEmpty() {
		return false
	}

	boxes := shape.BoxesAt(pos, nil)
	for _, id := range entities.IDs() {
		body, ok := entities.Entity(id)
		if !ok {
			continue
		}
		for _, box := range boxes {
			if box.Intersects(body.Box) {
				return true
			}
		}
	}

	return false
}

// distanceToCell measures from the eye to the nearest point of a cell.
func distanceToCell(eye geom.Vec3, pos geom.BlockPos) float64 {
	cell := geom.BlockAABB(pos)

	gap := geom.Vec3{
		X: axisGap(eye.X, cell.MinX, cell.MaxX),
		Y: axisGap(eye.Y, cell.MinY, cell.MaxY),
		Z: axisGap(eye.Z, cell.MinZ, cell.MaxZ),
	}

	return math.Sqrt(gap.X*gap.X + gap.Y*gap.Y + gap.Z*gap.Z)
}

// axisGap is how far a coordinate lies outside a span, or zero inside it.
func axisGap(at, minimum, maximum float64) float64 {
	switch {
	case at < minimum:
		return minimum - at
	case at > maximum:
		return at - maximum
	default:
		return 0
	}
}

// refused builds a refusal with its reason.
func refused(reason string) Legality {
	return Legality{Reason: reason}
}

// incomplete names the cell a decision needed and did not have.
func incomplete(pos geom.BlockPos) sim.Completeness {
	return sim.Completeness{Missing: []sim.Dependency{{
		Kind:  sim.DependencyBlock,
		Block: pos,
	}}}
}
