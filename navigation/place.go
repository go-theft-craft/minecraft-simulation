package navigation

import (
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// places returns the bridging edges leaving a cell.
//
// A bridge fills the cell under a neighbour the body would otherwise fall
// through, and then walks into it. It is the smallest edge that needs the
// overlay, which is why it is built before the pillar.
//
// The face to place against is the body's own footing: the cell under the body
// is solid, and the cell being filled is next to it. That is what makes this a
// bridge rather than a block hung in the air, and it is why no separate
// support check is needed for the first block of one.
func (c Capability) places(o oracle, from node) ([]Edge, error) {
	if !c.CanPlace || c.BlockBudget <= 0 {
		return nil, nil
	}

	edges := make([]Edge, 0, len(steps))
	for _, step := range steps {
		to := geom.BlockPos{X: from.Pos.X + step.X, Y: from.Pos.Y, Z: from.Pos.Z + step.Z}

		ok, err := c.canBridgeInto(o, to)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		arr, err := o.arriveAt(to)
		if err != nil {
			return nil, err
		}
		if !arr.ok || arr.posture != PostureStand {
			continue
		}

		edges = append(edges, Edge{
			Kind: EdgePlace, From: from.Pos, To: to,
			Posture: PostureStand, Cost: c.PlaceTicks + c.BlockTicks,
		})
	}

	return edges, nil
}

// canBridgeInto reports whether a neighbouring cell is a hole this body may
// fill and step into.
//
// The cell has to be one the body fits in with nothing holding it up — a hole,
// not a wall — and the cell beneath it has to be empty and described, because a
// block cannot be put where one already is and must not be put where nobody has
// said what is there.
func (c Capability) canBridgeInto(o oracle, to geom.BlockPos) (bool, error) {
	passable, err := o.passable(to)
	if err != nil {
		return false, err
	}
	if passable != terrain.Blocked {
		return false, nil
	}

	admits, err := o.clear(to)
	if err != nil {
		return false, err
	}
	if !admits {
		// A wall, not a hole. Filling it in is not a thing to do.
		return false, nil
	}

	return o.placeable(geom.BlockPos{X: to.X, Y: to.Y - 1, Z: to.Z})
}

// maxValidationRounds bounds the re-run-and-ban loop.
//
// Each round bans one edge, so the loop terminates on any finite set of routes.
// The bound is here for the case that is not finite: an open world where every
// banned bridge has another one beside it. Exhausting it returns the last path
// found rather than an error, because a route the body can start walking beats
// a refusal, which is the same reasoning that returns a partial path elsewhere.
const maxValidationRounds = 64

// validate walks a path in order against an overlay and reports the first edge
// that is not legal by the time the body reaches it.
//
// This is the parent design's validation loop. A search expands nodes without
// any notion of which placements a route has already made, so a winning path can
// be internally inconsistent: one edge's block sits in a cell a later edge has
// to pass through. Nothing during the search can see that, because the two edges
// belong to branches that were never compared.
//
// The check is forward and cumulative, which is the only order in which the
// question means anything. An edge is legal or not against the world as it
// stands at the moment the body takes it.
func (c Capability) validate(
	base world.View,
	facts terrain.Facts,
	path Path,
) (Edge, bool, error) {
	overlay := NewOverlay(base)
	o := directOracle{
		query:      c.query(overlay, facts),
		capability: c,
		crawlQuery: c.crawling().query(overlay, facts),
	}

	placed := 0
	column := make(map[geom.BlockPos]int)

	for _, edge := range path.Edges {
		legal, err := c.edgeHolds(o, edge)
		if err != nil {
			return Edge{}, false, err
		}
		if !legal {
			return edge, true, nil
		}

		block, mutates := placementOf(edge)
		if !mutates {
			continue
		}

		placed++
		if placed > c.BlockBudget {
			// The body has run out of blocks. Banning this edge is what makes
			// the next search find a route that fits in the inventory.
			return edge, true, nil
		}
		if edge.Kind == EdgePillar && c.MaxPillarHeight > 0 {
			foot := geom.BlockPos{X: block.X, Z: block.Z}
			column[foot]++
			if column[foot] > c.MaxPillarHeight {
				return edge, true, nil
			}
		}

		overlay.Place(block, c.PlacedBlock, geom.FullCube())
	}

	return Edge{}, false, nil
}

// placementOf returns the cell an edge fills, and whether it fills one.
//
// The position is derived rather than carried on the edge, because for both
// mutating edges it is fixed by the geometry: a bridge fills the cell under
// where it lands, and a pillar fills the cell it left.
func placementOf(edge Edge) (geom.BlockPos, bool) {
	switch edge.Kind {
	case EdgePlace:
		return geom.BlockPos{X: edge.To.X, Y: edge.To.Y - 1, Z: edge.To.Z}, true
	case EdgePillar:
		return edge.From, true
	case EdgeWalk, EdgeStep, EdgeFall, EdgeSwim, EdgeJumpGap, EdgeWaterDrop, EdgeClimb, EdgeDoor:
		return geom.BlockPos{}, false
	}

	return geom.BlockPos{}, false
}

// edgeHolds re-checks one edge's destination against the world as it now
// stands.
func (c Capability) edgeHolds(o directOracle, edge Edge) (bool, error) {
	switch edge.Kind {
	case EdgeClimb:
		return o.clear(edge.To)
	case EdgeDoor:
		passable, err := o.passableThroughDoor(edge.To)

		return passable == terrain.Clear, err
	case EdgePlace:
		// The cell is a hole until the block goes in, so what has to hold is
		// that it is still a hole this body may fill.
		return c.canBridgeInto(o, edge.To)
	case EdgePillar:
		// The body rises into the cell above, which has to still admit it, and
		// the cell it leaves has to still be fillable.
		admits, err := o.clear(edge.To)
		if err != nil || !admits {
			return false, err
		}

		return o.placeable(edge.From)
	case EdgeWalk, EdgeStep, EdgeFall, EdgeSwim, EdgeJumpGap, EdgeWaterDrop:
		passable, err := o.passable(edge.To)

		return passable == terrain.Clear, err
	}

	return false, nil
}

// mutates reports whether this body can change the world at all, which is what
// decides whether a search needs validating.
//
// A body that cannot place produces no mutating edge, so its path cannot be
// self-inconsistent and the loop is skipped entirely. That is what keeps the
// read-only search exactly as fast and exactly as deterministic as it was.
func (c Capability) mutates() bool { return c.CanPlace && c.BlockBudget > 0 }
