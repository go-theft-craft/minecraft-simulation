// Package geom holds the value types the simulation measures space with:
// vectors, block positions, axis-aligned boxes, and per-block voxel shapes.
//
// Every value in this package is immutable and every operation returns a new
// value. Nothing here reads the clock, allocates shared state, or depends on
// any package outside the standard library.
package geom

import "math"

// Vec3 is a position or a motion in world space.
type Vec3 struct {
	X, Y, Z float64
}

// Add returns the component-wise sum.
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{X: v.X + other.X, Y: v.Y + other.Y, Z: v.Z + other.Z}
}

// Sub returns the component-wise difference.
func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{X: v.X - other.X, Y: v.Y - other.Y, Z: v.Z - other.Z}
}

// Scale returns the vector multiplied by factor.
func (v Vec3) Scale(factor float64) Vec3 {
	return Vec3{X: v.X * factor, Y: v.Y * factor, Z: v.Z * factor}
}

// HorizontalLengthSquared returns the squared length in the XZ plane. Step-up
// picks a winner with this, so it must ignore Y and must not take a root: the
// comparison is exact this way.
func (v Vec3) HorizontalLengthSquared() float64 {
	return v.X*v.X + v.Z*v.Z
}

// IsZero reports whether every component is exactly zero.
func (v Vec3) IsZero() bool {
	return v.X == 0 && v.Y == 0 && v.Z == 0
}

// BlockPos identifies one block cell.
type BlockPos struct {
	X, Y, Z int32
}

// Floor rounds toward negative infinity. A Go conversion truncates toward
// zero, which puts every negative coordinate in the wrong cell.
func Floor(value float64) int32 {
	return int32(math.Floor(value))
}

// BlockPosOf returns the cell containing v.
func BlockPosOf(v Vec3) BlockPos {
	return BlockPos{X: Floor(v.X), Y: Floor(v.Y), Z: Floor(v.Z)}
}
