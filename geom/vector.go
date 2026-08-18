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

// HorizontalDistance reports the distance to other ignoring height.
//
// A great many bounds are horizontal: a mob one block below a body is in reach,
// and a mob thirty blocks below it on a cliff is not somewhere to chase.
func (v Vec3) HorizontalDistance(other Vec3) float64 {
	return math.Hypot(v.X-other.X, v.Z-other.Z)
}

// Toward returns the position reached by moving at most limit blocks from v
// toward other, horizontally.
//
// The height is v's. This chooses no Y, because it has no physics and a step
// that changed one would be claiming to fall or fly rather than to walk.
//
// It stops exactly on the target rather than overshooting, which is what makes
// arrival a stable condition instead of a point a body oscillates around.
func (v Vec3) Toward(other Vec3, limit float64) Vec3 {
	distance := v.HorizontalDistance(other)
	if distance <= limit || distance == 0 {
		return Vec3{X: other.X, Y: v.Y, Z: other.Z}
	}

	scale := limit / distance

	return Vec3{
		X: v.X + (other.X-v.X)*scale,
		Y: v.Y,
		Z: v.Z + (other.Z-v.Z)*scale,
	}
}

// Yaw returns the heading from v to other in degrees, as the wire carries it.
//
// Minecraft measures yaw from south, which is +Z, and increases it toward west,
// which is -X. That is neither the mathematical convention nor a compass
// bearing, so the arguments to Atan2 are the way they are on purpose.
func (v Vec3) Yaw(other Vec3) float32 {
	return float32(math.Atan2(-(other.X-v.X), other.Z-v.Z) * 180 / math.Pi)
}

// Pitch returns the downward angle from v to other in degrees, as the wire
// carries it.
//
// Positive is down. That is the game's convention and not the intuitive one:
// looking at the sky is a negative pitch. A client that flips the sign aims at
// the ceiling every time it means to mine the floor, and the mistake is
// invisible on flat ground, which is where most of a bot's testing happens.
//
// The angle is measured against horizontal distance rather than against a
// straight-line one, because that is what makes straight down exactly ninety
// degrees rather than something asymptotic to it.
func (v Vec3) Pitch(other Vec3) float32 {
	horizontal := v.HorizontalDistance(other)
	rise := other.Y - v.Y

	return float32(-math.Atan2(rise, horizontal) * 180 / math.Pi)
}

// Look returns both angles at once.
//
// Every caller that needs one needs the other, and computing them separately
// walks the same vector twice. It is not a convenience: an aim assembled from
// two calls has two chances to be given different endpoints.
func (v Vec3) Look(other Vec3) (yaw, pitch float32) {
	return v.Yaw(other), v.Pitch(other)
}
