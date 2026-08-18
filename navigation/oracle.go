package navigation

import (
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
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
