package navigation

import (
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// waterDrop looks down a neighbouring column for a landing in water deep enough
// to break a fall the body could not otherwise survive.
//
// It walks the same column the ordinary fall walks and stops at the same first
// cell the body does not fall through, because the two are answering the same
// question about the same shaft. What differs is the rule at the bottom: fall
// refuses a drop deeper than SafeFall, and this one admits it when the landing
// is fluid and there is enough of it underneath.
//
// A capability with no landing depth produces nothing, which is what keeps a
// search that never measured its version exactly as deep as it was.
func (c Capability) waterDrop(o oracle, from, neighbour geom.BlockPos) (Edge, bool, error) {
	if !c.CanSwim || c.WaterLandingDepth <= 0 {
		return Edge{}, false, nil
	}

	for drop := int32(1); drop <= maxFallSearch; drop++ {
		landing := geom.BlockPos{X: neighbour.X, Y: neighbour.Y - drop, Z: neighbour.Z}

		fluid, lookup, err := o.fluidAt(landing)
		if err != nil {
			return Edge{}, false, err
		}
		if lookup == world.LookupUnknown {
			// An undescribed cell ends the search down this column. Guessing
			// water below it would be guessing the one fact the whole edge
			// rests on.
			return Edge{}, false, nil
		}

		if fluid != terrain.FluidWater {
			passable, err := o.passable(landing)
			if err != nil {
				return Edge{}, false, err
			}
			if passable == terrain.Blocked {
				// Still falling: nothing holds a body up here and it is not
				// water, so keep descending.
				continue
			}

			// Solid ground, or a cell the body cannot fall through. The
			// ordinary fall edge owns this landing, and owning it twice would
			// give the search two prices for one move.
			return Edge{}, false, nil
		}

		deep, err := c.waterIsDeepEnough(o, landing)
		if err != nil {
			return Edge{}, false, err
		}
		if !deep {
			// Shallow water does not break a fall. The body hits the bottom
			// through it, so this column offers nothing below either.
			return Edge{}, false, nil
		}

		// The hazard gate still applies: water is safe to land in and the
		// things floating in it may not be.
		arr, err := o.arriveAt(landing)
		if err != nil {
			return Edge{}, false, err
		}
		if !arr.ok || arr.posture != PostureSwim {
			return Edge{}, false, nil
		}

		return Edge{
			Kind: EdgeWaterDrop, From: from, To: landing,
			Posture: PostureSwim, Cost: c.FallTicks * float64(drop),
		}, true, nil
	}

	return Edge{}, false, nil
}

// waterIsDeepEnough reports whether a column holds WaterLandingDepth blocks of
// water counting down from the surface cell.
//
// It counts down rather than up because the surface is where the body arrives,
// and what stops it is how much water is under that.
func (c Capability) waterIsDeepEnough(o oracle, surface geom.BlockPos) (bool, error) {
	needed := int32(math.Ceil(c.WaterLandingDepth))
	for depth := int32(0); depth < needed; depth++ {
		cell := geom.BlockPos{X: surface.X, Y: surface.Y - depth, Z: surface.Z}

		fluid, lookup, err := o.fluidAt(cell)
		if err != nil {
			return false, err
		}
		if lookup == world.LookupUnknown || fluid != terrain.FluidWater {
			return false, nil
		}
	}

	return true, nil
}

// climbs returns the edges up and down a climbable column.
//
// It is vertical rather than a neighbour move, which is why it is not in the
// four-step loop. A ladder's collision box is empty, so nothing in collision
// distinguishes one from air and the fact has to be asked for: that is the
// whole reason the climbable property is measured out of a jar rather than
// derived from a shape.
//
// The rule is the game's: a body climbs while it is in a climbable cell. Going
// up needs the cell above to be climbable too, or to be somewhere the body can
// stand — which is how a ladder is stepped off at the top. Going down needs the
// cell below to be climbable, or the body would be falling rather than
// climbing, and falling is an edge that already exists.
func (c Capability) climbs(o oracle, from node) ([]Edge, error) {
	if !c.CanClimb {
		return nil, nil
	}

	here, err := o.climbable(from.Pos)
	if err != nil {
		return nil, err
	}
	if !here {
		return nil, nil
	}

	edges := make([]Edge, 0, 2)
	for _, rise := range [2]int32{1, -1} {
		to := geom.BlockPos{X: from.Pos.X, Y: from.Pos.Y + rise, Z: from.Pos.Z}

		admits, err := o.clear(to)
		if err != nil {
			return nil, err
		}
		if !admits {
			continue
		}

		arr, err := o.arriveAt(to)
		if err != nil {
			return nil, err
		}
		if !arr.ok {
			continue
		}

		// The destination is legal when it is more of the same column, or when
		// it is a cell the body can stand in — the top of the ladder, or the
		// floor at the bottom of it.
		onward, err := o.climbable(to)
		if err != nil {
			return nil, err
		}
		if !onward {
			passable, err := o.passable(to)
			if err != nil {
				return nil, err
			}
			if passable != terrain.Clear {
				continue
			}
		}

		edges = append(edges, Edge{
			Kind: EdgeClimb, From: from.Pos, To: to,
			Posture: arr.posture, Cost: c.ClimbTicks,
		})
	}

	return edges, nil
}
