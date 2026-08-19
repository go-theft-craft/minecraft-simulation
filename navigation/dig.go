package navigation

import (
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// spanHeight is how many whole cells a body of this height occupies when its
// feet are at a cell's floor.
//
// It rounds up, because a body 1.8 blocks tall standing on a floor reaches
// into the cell above it and a route that left that cell filled would walk the
// body's head through a wall. A body exactly two blocks tall occupies two
// cells and not three: the ceiling is where it ends, not where it starts.
func spanHeight(body terrain.Body) int32 {
	cells := int32(math.Ceil(body.Height))
	if cells < 1 {
		return 1
	}

	return cells
}

// spanOf returns the cells a body occupies standing in one cell, feet first.
//
// The order is fixed and bottom-up so that a dig edge's cost, which sums over
// it, is the same number every time it is computed. Nothing here reads a map.
func spanOf(body terrain.Body, cell geom.BlockPos) []geom.BlockPos {
	height := spanHeight(body)
	span := make([]geom.BlockPos, 0, height)
	for y := range height {
		span = append(span, geom.BlockPos{X: cell.X, Y: cell.Y + y, Z: cell.Z})
	}

	return span
}

// dugView reports a set of cells as empty and defers everything else.
//
// It is openedView's sibling, and exists for the same reason: the search asks
// what a cell would be if the blocks in the way were gone, and gets the answer
// from the ordinary passability rules rather than from a second set written
// for digging.
type dugView struct {
	view world.View
	// span is the cells the dig clears. It is a short fixed slice — two cells
	// for a player-sized body — so a linear scan beats a map, and a map here
	// would put an allocation in the middle of an expansion.
	span []geom.BlockPos
}

// covers reports whether a cell is one the dig clears.
func (d dugView) covers(pos geom.BlockPos) bool {
	for _, cell := range d.span {
		if cell == pos {
			return true
		}
	}

	return false
}

// CollisionShape implements world.BlockView.
//
// An undescribed cell stays undescribed, exactly as openedView leaves one.
// Masking it would turn "nobody has said what is here" into "this is open",
// which is the one substitution every other rule in this package refuses to
// make — and for a dig it would additionally claim a break nobody can price.
func (d dugView) CollisionShape(pos geom.BlockPos) (geom.Shape, world.Lookup) {
	shape, lookup := d.view.CollisionShape(pos)
	if lookup == world.LookupUnknown || !d.covers(pos) {
		return shape, lookup
	}

	return geom.EmptyShape(), world.LookupAir
}

// BlockState implements world.StateView.
//
// The handle is left alone, as openedView leaves a door's. A caller asking
// what block is in a cell this route means to break should be told which block
// that is; only its shape is being answered as if the break had happened.
func (d dugView) BlockState(pos geom.BlockPos) (world.BlockRef, world.Lookup) {
	return d.view.BlockState(pos)
}

// digs returns the dig edges leaving a node, one per horizontal neighbour the
// body cannot otherwise enter.
//
// It runs after every other builder and only for a body with a breaker, so a
// search that cannot dig expands exactly the edges it did before this existed,
// in exactly the order it did.
//
// A dig is offered only where an ordinary move is not. The four cases the
// switch in expand already handles — clear, steppable, and the two ways down —
// each produce their own edge from the same neighbour, and a dig competing
// with them would price a break against a route that needs none.
func (c Capability) digs(o oracle, from node) ([]Edge, error) {
	if c.Breaker == nil {
		return nil, nil
	}

	var edges []Edge

	for _, step := range steps {
		neighbour := geom.BlockPos{X: from.Pos.X + step.X, Y: from.Pos.Y, Z: from.Pos.Z + step.Z}

		passable, err := o.passable(neighbour)
		if err != nil {
			return nil, err
		}
		if passable != terrain.Blocked {
			continue
		}

		edge, ok, err := c.digInto(o, from.Pos, neighbour)
		if err != nil {
			return nil, err
		}
		if ok {
			edges = append(edges, edge)
		}
	}

	return edges, nil
}

// digInto prices and checks one dig, and reports whether it is legal.
func (c Capability) digInto(o oracle, from, to geom.BlockPos) (Edge, bool, error) {
	span := spanOf(c.Body, to)

	cost := c.WalkTicks
	broken := 0

	for _, cell := range span {
		open, err := o.clear(cell)
		if err != nil {
			return Edge{}, false, err
		}
		if open {
			// Already open. A span cell that needs no break is not charged
			// for one, which is what makes digging a doorway through a
			// one-block wall cost one break rather than the body's height.
			continue
		}

		ticks, breakable, err := o.breakTicks(cell)
		if err != nil {
			return Edge{}, false, err
		}
		if !breakable {
			// Bedrock, or a cell nobody described, or a block this body's
			// tool cannot touch. One refusal refuses the whole edge: a body
			// cannot walk through half a wall.
			return Edge{}, false, nil
		}

		cost += ticks
		broken++
	}

	if broken == 0 {
		// Nothing in the span to break, so whatever refused this cell was not
		// something digging fixes — no floor under it, most likely, which is
		// the fall builder's business and not this one's.
		return Edge{}, false, nil
	}
	if c.DigBudget > 0 && broken > c.DigBudget {
		return Edge{}, false, nil
	}

	// The cells are still filled: this asks what the destination would be once
	// they are not. A hole with no floor is still refused here, by the
	// ordinary rules, which is why nothing above checks for one.
	passable, err := o.passableAfterDig(to, span)
	if err != nil || passable != terrain.Clear {
		return Edge{}, false, err
	}

	arrival, err := o.arriveAfterDig(to, span)
	if err != nil || !arrival.ok {
		return Edge{}, false, err
	}

	return Edge{
		Kind: EdgeDig, From: from, To: to,
		Posture: arrival.posture, Cost: cost,
	}, true, nil
}
