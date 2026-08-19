package navigation

import (
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Overlay is a world.View that answers from a set of pending placements first
// and from a base view otherwise.
//
// A search that can place blocks has to search a world in which its own
// placements already exist, and it has to do that without touching the caller's
// snapshot: most of the routes it considers are discarded, and a search that
// wrote through would leave every discarded one behind.
//
// world.View is an interface, so this is a decorator and nothing in world
// changes. That is what the interface is for.
//
// It is not safe for concurrent use. One search owns one overlay, which is the
// same rule the frontier follows.
type Overlay struct {
	base world.View
	// placed is only ever indexed. Nothing iterates it, so no output ordering
	// can depend on Go's map order — which is the rule the determinism gate
	// enforces from a long way away.
	placed map[geom.BlockPos]placement
	// broken is the cells a route has dug out. It is a separate map from
	// placed rather than a placement of air, because the two answer different
	// lookups: a placed block has a handle to report and a hole has none, and
	// folding them together would make a dug cell claim to hold whatever was
	// last placed there.
	//
	// Placing into a cell this route dug is legal and the placement wins, so
	// placed is consulted first.
	broken map[geom.BlockPos]struct{}
}

// placement is one pending block: what it is and what shape it has.
type placement struct {
	ref   world.BlockRef
	shape geom.Shape
}

// NewOverlay returns an overlay with nothing placed in it.
func NewOverlay(base world.View) *Overlay {
	return &Overlay{
		base:   base,
		placed: make(map[geom.BlockPos]placement),
		broken: make(map[geom.BlockPos]struct{}),
	}
}

// Place records a pending block.
func (o *Overlay) Place(pos geom.BlockPos, ref world.BlockRef, shape geom.Shape) {
	o.placed[pos] = placement{ref: ref, shape: shape}
}

// Remove forgets one pending block. It does not hide a block the base view
// holds; Break is what takes a block away.
func (o *Overlay) Remove(pos geom.BlockPos) {
	delete(o.placed, pos)
}

// Break records a cell as dug out, hiding whatever the base view holds there.
//
// It also forgets a pending placement in the same cell, so that a route which
// places a block and then breaks it leaves a hole rather than the block: the
// overlay replays a route in order, and the last thing done to a cell is what
// the cell is.
func (o *Overlay) Break(pos geom.BlockPos) {
	delete(o.placed, pos)
	o.broken[pos] = struct{}{}
}

// Reset forgets every pending block and every hole, so one overlay serves the
// whole validation loop rather than one per round.
func (o *Overlay) Reset() {
	clear(o.placed)
	clear(o.broken)
}

// Broken reports whether a cell is one this route dug out.
func (o *Overlay) Broken(pos geom.BlockPos) bool {
	_, ok := o.broken[pos]

	return ok
}

// Len reports how many blocks are pending, which is what the resource check
// counts against a body's budget.
func (o *Overlay) Len() int { return len(o.placed) }

// CollisionShape implements world.BlockView.
//
// An unknown cell in the base stays unknown unless something was placed there.
// Answering "air" for a cell nobody described would let a route run through
// unloaded chunks, which is the substitution every rule in this package
// refuses.
func (o *Overlay) CollisionShape(pos geom.BlockPos) (geom.Shape, world.Lookup) {
	if pending, ok := o.placed[pos]; ok {
		return pending.shape, world.LookupShape
	}
	// A hole answers air rather than deferring, but only where the base view
	// described something: a cell nobody described is not made known by a
	// route claiming to have dug it.
	if _, dug := o.broken[pos]; dug {
		if _, lookup := o.base.CollisionShape(pos); lookup != world.LookupUnknown {
			return geom.EmptyShape(), world.LookupAir
		}
	}

	return o.base.CollisionShape(pos)
}

// BlockState implements world.StateView.
func (o *Overlay) BlockState(pos geom.BlockPos) (world.BlockRef, world.Lookup) {
	if pending, ok := o.placed[pos]; ok {
		return pending.ref, world.LookupShape
	}
	// A dug cell holds no block. It answers the zero handle, which
	// world.BlockRef documents as carrying no meaning — the handle for a cell
	// whose shape is known without a block behind it. Reporting the handle it
	// used to hold would let a second dig charge for the same break twice.
	if _, dug := o.broken[pos]; dug {
		if _, lookup := o.base.BlockState(pos); lookup != world.LookupUnknown {
			return world.BlockRef(0), world.LookupAir
		}
	}

	return o.base.BlockState(pos)
}
