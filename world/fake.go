package world

import "github.com/go-theft-craft/minecraft-simulation/geom"

// Blocks is an in-memory BlockView. Every position starts unknown, so a test
// that means "empty space" has to say so, and a test that forgets to describe
// a region gets the unknown path rather than a silent floor of air.
//
// Blocks is not safe for concurrent modification. Build it, then read it.
type Blocks struct {
	shapes map[geom.BlockPos]geom.Shape
}

// NewBlocks returns an empty view in which every position is unknown.
func NewBlocks() *Blocks {
	return &Blocks{shapes: make(map[geom.BlockPos]geom.Shape)}
}

// Set records a block shape. An empty shape records air, because a block
// nothing collides with is indistinguishable from air to this package.
func (b *Blocks) Set(pos geom.BlockPos, shape geom.Shape) {
	b.shapes[pos] = shape
}

// SetAir records that the position holds nothing collidable.
func (b *Blocks) SetAir(pos geom.BlockPos) {
	b.shapes[pos] = geom.EmptyShape()
}

// Forget returns the position to unknown.
func (b *Blocks) Forget(pos geom.BlockPos) {
	delete(b.shapes, pos)
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
	shape, ok := b.shapes[pos]
	if !ok {
		return geom.EmptyShape(), LookupUnknown
	}
	if shape.IsEmpty() {
		return geom.EmptyShape(), LookupAir
	}

	return shape, LookupShape
}
