package terrain

import "github.com/go-theft-craft/minecraft-simulation/geom"

// Body is the box a thing occupies, measured from its feet.
//
// A position names the feet rather than the centre, because that is where a
// block cell puts a standing entity and it is what every caller here has.
type Body struct {
	// HalfWidth is half the footprint, in blocks.
	HalfWidth float64
	// Height is how tall the body is, in blocks.
	Height float64
	// StepHeight is how far the body rises to clear an obstacle without
	// leaving the ground. It arrives from the profile's MotionConstants.
	StepHeight float64
}

// BoxAt returns the box the body occupies with its feet at the position.
func (b Body) BoxAt(feet geom.Vec3) geom.AABB {
	return geom.AABB{
		MinX: feet.X - b.HalfWidth,
		MinY: feet.Y,
		MinZ: feet.Z - b.HalfWidth,
		MaxX: feet.X + b.HalfWidth,
		MaxY: feet.Y + b.Height,
		MaxZ: feet.Z + b.HalfWidth,
	}
}

// FeetOf returns the feet position of a body standing in the middle of a cell.
func FeetOf(cell geom.BlockPos) geom.Vec3 {
	return geom.Vec3{
		X: float64(cell.X) + 0.5,
		Y: float64(cell.Y),
		Z: float64(cell.Z) + 0.5,
	}
}
