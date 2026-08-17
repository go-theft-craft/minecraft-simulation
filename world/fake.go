package world

import "github.com/go-theft-craft/minecraft-simulation/geom"

// cell is one described block: its profile handle and its collision shape.
type cell struct {
	ref   BlockRef
	shape geom.Shape
}

// Blocks is an in-memory View. Every position starts unknown, so a test that
// means "empty space" has to say so, and a test that forgets to describe a
// region gets the unknown path rather than a silent floor of air.
//
// Blocks is not safe for concurrent modification. Build it, then read it.
type Blocks struct {
	cells map[geom.BlockPos]cell
}

// NewBlocks returns an empty view in which every position is unknown.
func NewBlocks() *Blocks {
	return &Blocks{cells: make(map[geom.BlockPos]cell)}
}

// Set records a block shape under the zero handle. An empty shape records air,
// because a block nothing collides with is indistinguishable from air to a
// caller that only asked about collision.
func (b *Blocks) Set(pos geom.BlockPos, shape geom.Shape) {
	b.SetBlock(pos, 0, shape)
}

// SetBlock records a block state and its collision shape.
func (b *Blocks) SetBlock(pos geom.BlockPos, ref BlockRef, shape geom.Shape) {
	b.cells[pos] = cell{ref: ref, shape: shape}
}

// SetAir records that the position holds nothing collidable.
func (b *Blocks) SetAir(pos geom.BlockPos) {
	b.SetBlock(pos, 0, geom.EmptyShape())
}

// Forget returns the position to unknown.
func (b *Blocks) Forget(pos geom.BlockPos) {
	delete(b.cells, pos)
}

// Fill records the same shape for every cell in the inclusive range.
func (b *Blocks) Fill(from, to geom.BlockPos, shape geom.Shape) {
	for x := from.X; x <= to.X; x++ {
		for y := from.Y; y <= to.Y; y++ {
			for z := from.Z; z <= to.Z; z++ {
				b.Set(geom.BlockPos{X: x, Y: y, Z: z}, shape)
			}
		}
	}
}

// CollisionShape implements BlockView.
func (b *Blocks) CollisionShape(pos geom.BlockPos) (geom.Shape, Lookup) {
	found, ok := b.cells[pos]
	if !ok {
		return geom.EmptyShape(), LookupUnknown
	}
	if found.shape.IsEmpty() {
		return geom.EmptyShape(), LookupAir
	}

	return found.shape, LookupShape
}

// BlockState implements StateView.
func (b *Blocks) BlockState(pos geom.BlockPos) (BlockRef, Lookup) {
	found, ok := b.cells[pos]
	if !ok {
		return 0, LookupUnknown
	}
	if found.shape.IsEmpty() {
		return found.ref, LookupAir
	}

	return found.ref, LookupShape
}

// Clone returns a view that does not alias this one, which is what lets a store
// hand out a snapshot a later tick cannot change underneath a reader.
func (b *Blocks) Clone() *Blocks {
	clone := &Blocks{cells: make(map[geom.BlockPos]cell, len(b.cells))}
	for pos, found := range b.cells {
		clone.cells[pos] = found
	}

	return clone
}
