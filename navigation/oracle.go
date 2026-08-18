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
}

// directOracle asks terrain every time, caching nothing.
type directOracle struct {
	query      terrain.Query
	capability Capability
}

// passable implements oracle.
func (d directOracle) passable(cell geom.BlockPos) (terrain.Passability, error) {
	return d.query.Passable(cell)
}

// arriveAt implements oracle.
func (d directOracle) arriveAt(cell geom.BlockPos) (arrival, error) {
	return d.capability.arrivalAt(d.query, cell)
}
