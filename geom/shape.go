package geom

import "slices"

// Shape is a block's collision volume in block-local coordinates, where the
// cell spans zero to one on every axis. Most blocks are one box; fences,
// slabs, and stairs are several.
//
// A Shape is immutable and safe to share across every cell of a block type.
type Shape struct {
	boxes []AABB
}

// NewShape returns a shape holding a copy of boxes, in the order given. The
// order is preserved because collision resolution visits boxes in order and
// the result must not depend on how the caller built the slice.
func NewShape(boxes ...AABB) Shape {
	return Shape{boxes: slices.Clone(boxes)}
}

// FullCube returns the shape of an ordinary solid block.
func FullCube() Shape {
	return Shape{boxes: []AABB{{MaxX: 1, MaxY: 1, MaxZ: 1}}}
}

// EmptyShape returns the shape of a block nothing collides with.
func EmptyShape() Shape {
	return Shape{}
}

// IsEmpty reports whether the shape has no boxes.
func (s Shape) IsEmpty() bool {
	return len(s.boxes) == 0
}

// Len returns the number of boxes.
func (s Shape) Len() int {
	return len(s.boxes)
}

// BoxesAt appends the shape's boxes, translated into the cell at pos, to dst
// and returns the extended slice. Passing a reused buffer as dst lets a broad
// phase walk thousands of cells without allocating per cell.
func (s Shape) BoxesAt(pos BlockPos, dst []AABB) []AABB {
	origin := Vec3{X: float64(pos.X), Y: float64(pos.Y), Z: float64(pos.Z)}
	for _, box := range s.boxes {
		dst = append(dst, box.Offset(origin))
	}

	return dst
}
