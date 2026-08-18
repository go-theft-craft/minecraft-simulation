package navigation

import (
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// oracle answers the only two questions the search asks about a cell.
//
// It exists so a caching implementation can stand in for the direct one without
// the search knowing. Find uses the direct oracle and behaves exactly as it did
// before this seam; Planner supplies a memoizing one.
type oracle interface {
	// passable classifies a cell for the body.
	passable(cell geom.BlockPos) (terrain.Passability, error)
	// arriveAt reports whether the body may come to rest in a cell, and how.
	arriveAt(cell geom.BlockPos) (arrival, error)
	// clear reports whether the body's box fits in a cell, with no question of
	// what holds it up.
	//
	// It is separate from passable because it asks half of what passable asks.
	// A body mid-jump is not standing on anything and must not be: a cell the
	// arc passes through is legal precisely when the box fits and nothing
	// supports it. Asking passable there would refuse every cell over a gap,
	// which is every cell a jump is for.
	clear(cell geom.BlockPos) (bool, error)
	// passableCrawling classifies a cell for the body while prone.
	//
	// It is a second question rather than an argument on the first because the
	// answers are cached per cell, and a body of a different height reads a
	// different span and gets a different answer. Two questions with two caches
	// is the shape that already works here; one question with a body in it
	// would need the cache keyed by both.
	passableCrawling(cell geom.BlockPos) (terrain.Passability, error)
	// fluidAt reports the fluid filling a cell, and whether the view knew.
	fluidAt(cell geom.BlockPos) (terrain.Fluid, world.Lookup, error)
	// climbable reports whether a body can climb the cell.
	//
	// An undescribed cell is not climbable, which is the cautious reading: a
	// body that climbed into an unloaded chunk would be climbing a ladder
	// nobody has said is there.
	climbable(cell geom.BlockPos) (bool, error)
	// doorAt reports whether a cell holds a door and whether it can be worked.
	doorAt(cell geom.BlockPos) (terrain.Door, error)
	// passableThroughDoor classifies a cell as it would be with the door in it
	// swung open.
	passableThroughDoor(cell geom.BlockPos) (terrain.Passability, error)
}

// directOracle asks terrain every time, caching nothing.
type directOracle struct {
	query      terrain.Query
	capability Capability
	// crawlQuery is the same view read with the prone body. It is built once
	// beside the standing one rather than per call, because building it per
	// call put a heap allocation in the middle of an expansion.
	crawlQuery terrain.Query
}

// passable implements oracle.
func (d directOracle) passable(cell geom.BlockPos) (terrain.Passability, error) {
	return d.query.Passable(cell)
}

// arriveAt implements oracle.
func (d directOracle) arriveAt(cell geom.BlockPos) (arrival, error) {
	return d.capability.arrivalAt(d.query, cell)
}

// clear implements oracle.
//
// An unknown fit is not clear. A body that treated a cell nobody described as
// room to fly through would jump into unloaded chunks, which is the same
// mistake as treating one as a wall and lands harder.
func (d directOracle) clear(cell geom.BlockPos) (bool, error) {
	fit, err := d.query.Fits(terrain.FeetOf(cell))
	if err != nil {
		return false, err
	}

	return fit == terrain.FitClear, nil
}

// passableCrawling implements oracle.
func (d directOracle) passableCrawling(cell geom.BlockPos) (terrain.Passability, error) {
	return d.crawlQuery.Passable(cell)
}

// fluidAt implements oracle.
func (d directOracle) fluidAt(cell geom.BlockPos) (terrain.Fluid, world.Lookup, error) {
	return d.query.FluidAt(cell)
}

// doorAt implements oracle.
func (d directOracle) doorAt(cell geom.BlockPos) (terrain.Door, error) {
	door, lookup, err := d.query.DoorAt(cell)
	if err != nil || lookup == world.LookupUnknown {
		return terrain.DoorNone, err
	}

	return door, nil
}

// passableThroughDoor implements oracle.
//
// The masked query is built per call rather than kept, because a door is rare
// and the mask names one column: keeping one would mean keeping one per door
// the search meets.
func (d directOracle) passableThroughDoor(cell geom.BlockPos) (terrain.Passability, error) {
	query := d.query
	query.View = openedView{view: d.query.View, door: cell}

	return query.Passable(cell)
}

// climbable implements oracle.
func (d directOracle) climbable(cell geom.BlockPos) (bool, error) {
	climbable, lookup, err := d.query.ClimbableAt(cell)
	if err != nil || lookup == world.LookupUnknown {
		return false, err
	}

	return climbable, nil
}
