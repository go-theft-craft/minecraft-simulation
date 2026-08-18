package navigation

import (
	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// pillars returns the edge that rises one block by placing a block into the
// cell the body is standing in.
//
// It is the edge the parent design's list has no member for. Place bridges
// horizontally and Support holds a column from below before a dig; nothing in
// that list gains height. A body that can place blocks but cannot pillar up is
// bounded by its step height and its jump arc, which is the same ceiling a body
// with no blocks at all has — and "pillar up to the surface" is the ordinary
// answer to being underground.
//
// # Descending is not the inverse
//
// A pillar cannot be walked back down, and this returns no downward edge. Coming
// down is a fall within the safe fall, or a dig beneath the body otherwise,
// which is a different edge with a different cost and a tool requirement. A
// symmetric edge is the natural thing to write here and it would produce routes
// that strand the body on top of a tower it has no way off.
func (c Capability) pillars(o oracle, from node) ([]Edge, error) {
	if !c.CanPlace || c.BlockBudget <= 0 {
		return nil, nil
	}
	// A body already in the air has nothing to place from and nothing to stand
	// on while it does.
	if from.Posture != PostureStand {
		return nil, nil
	}

	// The cell the block goes into is the one the body occupies, so it has to
	// be empty of everything but the body. If it is not, the body is standing
	// in something and this is not a pillar.
	fillable, err := o.placeable(from.Pos)
	if err != nil || !fillable {
		return nil, err
	}

	above := geom.BlockPos{X: from.Pos.X, Y: from.Pos.Y + 1, Z: from.Pos.Z}

	// The body ends up standing in the cell above, so its whole box has to fit
	// there. This is the ceiling check: a body with a block directly over its
	// head produces no pillar edge.
	admits, err := o.clear(above)
	if err != nil || !admits {
		return nil, err
	}

	arr, err := o.arriveAt(above)
	if err != nil {
		return nil, err
	}
	if !arr.ok || arr.posture != PostureStand {
		return nil, nil
	}

	return []Edge{{
		Kind: EdgePillar, From: from.Pos, To: above,
		Posture: PostureStand, Cost: c.PlaceTicks + c.BlockTicks,
	}}, nil
}

// withinEnvelope reports whether a cell is inside the vertical band a search may
// expand into.
//
// Placing makes every Y coordinate reachable from every position, and digging
// would make the same true downward. The node budget already stops a search
// running forever; it does not stop one spending its whole budget climbing to
// reach a horizontal detour it should have walked. This is the bound that does.
//
// A zero envelope means no bound, which is what every read-only capability
// carries and is why the shipped search is untouched by it.
func (c Capability) withinEnvelope(origin, cell geom.BlockPos) bool {
	if c.VerticalEnvelope <= 0 {
		return true
	}

	rise := cell.Y - origin.Y
	if rise < 0 {
		rise = -rise
	}

	return rise <= c.VerticalEnvelope
}
