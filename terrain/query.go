package terrain

import (
	"github.com/go-theft-craft/minecraft-simulation/collision"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Fit reports whether a body occupies a position.
type Fit uint8

const (
	// FitUnknown means the view could not answer for at least one cell the
	// body would occupy. It is the zero value so that a caller who forgets to
	// switch on it gets the cautious answer.
	FitUnknown Fit = iota
	// FitClear means the body occupies the position without overlapping
	// anything.
	FitClear
	// FitBlocked means something is in the way.
	FitBlocked
)

// Ground reports what is under a body's feet.
type Ground uint8

const (
	// GroundUnknown means the view could not answer.
	GroundUnknown Ground = iota
	// GroundSolid means something holds the body up.
	GroundSolid
	// GroundOpen means nothing does.
	GroundOpen
)

// groundProbe is how far below the feet to look for support. It has to be a
// volume rather than a plane because geom.AABB.Intersects excludes shared
// faces, so a probe of zero height would touch the floor and report nothing.
const groundProbe = 1e-4

// Query is one body asking about one view.
//
// Facts may be nil. Limit is the collision candidate budget; a non-positive
// value means no limit, matching collision.Gather.
type Query struct {
	View  world.View
	Facts Facts
	Body  Body
	Limit int
}

// Fits reports whether the body occupies the position.
func (q Query) Fits(feet geom.Vec3) (Fit, error) {
	box := q.Body.BoxAt(feet)

	candidates, err := collision.Gather(q.View, box, q.Limit)
	if err != nil {
		return FitUnknown, err
	}
	if len(candidates.Unknown) != 0 {
		return FitUnknown, nil
	}

	for _, other := range candidates.Boxes {
		if box.Intersects(other) {
			return FitBlocked, nil
		}
	}

	return FitClear, nil
}

// Ground reports whether anything holds the body up.
//
// The probe is a thin slab under the footprint rather than the cell below,
// because a body wider than one block stands on more than one cell and a
// half-slab holds it up from less than a full one.
func (q Query) Ground(feet geom.Vec3) (Ground, error) {
	box := q.Body.BoxAt(feet)
	probe := geom.AABB{
		MinX: box.MinX,
		MinY: feet.Y - groundProbe,
		MinZ: box.MinZ,
		MaxX: box.MaxX,
		MaxY: feet.Y,
		MaxZ: box.MaxZ,
	}

	candidates, err := collision.Gather(q.View, probe, q.Limit)
	if err != nil {
		return GroundUnknown, err
	}
	if len(candidates.Unknown) != 0 {
		return GroundUnknown, nil
	}

	for _, other := range candidates.Boxes {
		if probe.Intersects(other) {
			return GroundSolid, nil
		}
	}

	return GroundOpen, nil
}

// HazardAt reports what a cell would do to a body occupying it.
//
// The lookup is returned because "no hazard" and "nobody described this cell"
// are different answers, and a body that confuses them walks into lava it
// could not see. Lava carries no collision shape, so geometry alone never
// finds it.
func (q Query) HazardAt(cell geom.BlockPos) (Hazard, world.Lookup, error) {
	ref, lookup := q.View.BlockState(cell)
	if lookup == world.LookupUnknown || q.Facts == nil {
		return HazardNone, lookup, nil
	}

	return q.Facts.Hazard(ref), lookup, nil
}

// FluidAt reports the fluid filling a cell.
func (q Query) FluidAt(cell geom.BlockPos) (Fluid, world.Lookup, error) {
	ref, lookup := q.View.BlockState(cell)
	if lookup == world.LookupUnknown || q.Facts == nil {
		return FluidNone, lookup, nil
	}

	return q.Facts.Fluid(ref), lookup, nil
}

// ClimbableAt reports whether a body can climb the cell.
//
// The lookup is returned for the reason HazardAt returns one: "not climbable"
// and "nobody described this cell" are different answers, and a body that
// confused them would climb into an unloaded chunk.
func (q Query) ClimbableAt(cell geom.BlockPos) (bool, world.Lookup, error) {
	ref, lookup := q.View.BlockState(cell)
	if lookup == world.LookupUnknown || q.Facts == nil {
		return false, lookup, nil
	}

	return q.Facts.Climbable(ref), lookup, nil
}
