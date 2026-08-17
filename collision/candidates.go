// Package collision resolves swept axis-aligned motion against the blocks of a
// world view, reproducing Java Edition 1.8.9 exactly.
//
// The package knows nothing about entities, profiles, or the protocol. A
// caller supplies a box, a motion, and a view; it receives the motion that
// actually applied and what the box touched on the way.
package collision

import (
	"errors"
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// ErrCandidateLimit reports that a sweep would visit more cells than its
// deterministic budget allows. The budget exists so that a malformed motion
// cannot make one tick walk the world.
var ErrCandidateLimit = errors.New("collision: candidate limit exhausted")

// Candidates is the outcome of a broad-phase sweep.
type Candidates struct {
	// Boxes are the collision boxes overlapping the region, in visit order.
	Boxes []geom.AABB
	// Unknown holds every cell the view could not answer for, in visit order.
	// A caller that finds this non-empty must abandon the tick rather than
	// treat the region as empty.
	Unknown []geom.BlockPos
}

// Gather collects the collision boxes of every cell the region touches.
//
// Cells are visited X outermost, then Y, then Z, so both returned slices have
// an order that does not depend on map iteration. A non-positive limit means
// no limit.
func Gather(view world.BlockView, region geom.AABB, limit int) (Candidates, error) {
	minPos := geom.BlockPos{
		X: geom.Floor(region.MinX),
		Y: geom.Floor(region.MinY),
		Z: geom.Floor(region.MinZ),
	}
	maxPos := geom.BlockPos{
		X: geom.Floor(region.MaxX),
		Y: geom.Floor(region.MaxY),
		Z: geom.Floor(region.MaxZ),
	}

	var candidates Candidates
	visited := 0
	for x := minPos.X; x <= maxPos.X; x++ {
		for y := minPos.Y; y <= maxPos.Y; y++ {
			for z := minPos.Z; z <= maxPos.Z; z++ {
				visited++
				if limit > 0 && visited > limit {
					return Candidates{}, fmt.Errorf("%w: %d cells", ErrCandidateLimit, visited)
				}

				pos := geom.BlockPos{X: x, Y: y, Z: z}
				shape, lookup := view.CollisionShape(pos)
				switch lookup {
				case world.LookupUnknown:
					candidates.Unknown = append(candidates.Unknown, pos)
				case world.LookupAir:
				case world.LookupShape:
					candidates.Boxes = shape.BoxesAt(pos, candidates.Boxes)
				}
			}
		}
	}

	return candidates, nil
}
