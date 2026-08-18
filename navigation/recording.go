package navigation

import (
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// recordingView is a world.View that logs the cells read through it.
//
// It exists so a cached answer can say what it was computed from without anyone
// deriving that by hand. Passable reads a body's whole column plus the ground
// below it, and a taller body reads more; a hand-written rule about which cells
// an answer depends on would be wrong the first time the body changed. This
// records what the code actually read, so it cannot drift from it.
//
// It is not safe for concurrent use, and neither is the Planner that owns one.
type recordingView struct {
	view world.View
	// seen deduplicates, and cells preserves first-read order. The order is
	// not load-bearing for correctness — invalidation drops a set — but a
	// deterministic dependency list keeps a failing test reproducible.
	seen  map[geom.BlockPos]struct{}
	cells []geom.BlockPos
}

// CollisionShape implements world.BlockView, recording the read.
func (r *recordingView) CollisionShape(pos geom.BlockPos) (geom.Shape, world.Lookup) {
	r.record(pos)

	return r.view.CollisionShape(pos)
}

// BlockState implements world.StateView, recording the read.
func (r *recordingView) BlockState(pos geom.BlockPos) (world.BlockRef, world.Lookup) {
	r.record(pos)

	return r.view.BlockState(pos)
}

// reset clears the log, ready for the next answer.
func (r *recordingView) reset() {
	clear(r.seen)
	r.cells = r.cells[:0]
}

// read returns the cells logged since the last reset, in first-read order.
func (r *recordingView) read() []geom.BlockPos {
	return r.cells
}

// record logs one cell, once.
func (r *recordingView) record(pos geom.BlockPos) {
	if r.seen == nil {
		r.seen = make(map[geom.BlockPos]struct{})
	}
	if _, ok := r.seen[pos]; ok {
		return
	}
	r.seen[pos] = struct{}{}
	r.cells = append(r.cells, pos)
}
