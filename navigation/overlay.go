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
}

// placement is one pending block: what it is and what shape it has.
type placement struct {
	ref   world.BlockRef
	shape geom.Shape
}

// NewOverlay returns an overlay with nothing placed in it.
func NewOverlay(base world.View) *Overlay {
	return &Overlay{base: base, placed: make(map[geom.BlockPos]placement)}
}

// Place records a pending block.
func (o *Overlay) Place(pos geom.BlockPos, ref world.BlockRef, shape geom.Shape) {
	o.placed[pos] = placement{ref: ref, shape: shape}
}

// Remove forgets one pending block. It does not hide a block the base view
// holds: this overlay models what a route adds, and taking blocks away is a dig
// edge, which is not built.
func (o *Overlay) Remove(pos geom.BlockPos) {
	delete(o.placed, pos)
}

// Reset forgets every pending block, so one overlay serves the whole
// validation loop rather than one per round.
func (o *Overlay) Reset() {
	clear(o.placed)
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

	return o.base.CollisionShape(pos)
}

// BlockState implements world.StateView.
func (o *Overlay) BlockState(pos geom.BlockPos) (world.BlockRef, world.Lookup) {
	if pending, ok := o.placed[pos]; ok {
		return pending.ref, world.LookupShape
	}

	return o.base.BlockState(pos)
}
