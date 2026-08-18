package geom

// AABB is an axis-aligned box in world space. A box is valid when each Min is
// no greater than its Max; the package does not normalize, because vanilla
// does not, and silently reordering faces would hide a caller's bug.
type AABB struct {
	MinX, MinY, MinZ float64
	MaxX, MaxY, MaxZ float64
}

// BlockAABB returns the full-cube box of one block cell.
func BlockAABB(pos BlockPos) AABB {
	return AABB{
		MinX: float64(pos.X),
		MinY: float64(pos.Y),
		MinZ: float64(pos.Z),
		MaxX: float64(pos.X) + 1,
		MaxY: float64(pos.Y) + 1,
		MaxZ: float64(pos.Z) + 1,
	}
}

// Offset translates the box.
func (b AABB) Offset(delta Vec3) AABB {
	return AABB{
		MinX: b.MinX + delta.X,
		MinY: b.MinY + delta.Y,
		MinZ: b.MinZ + delta.Z,
		MaxX: b.MaxX + delta.X,
		MaxY: b.MaxY + delta.Y,
		MaxZ: b.MaxZ + delta.Z,
	}
}

// Stretch extends the box along delta without moving the opposite faces, so
// the result covers every point the box passes through while travelling delta.
func (b AABB) Stretch(delta Vec3) AABB {
	stretched := b
	if delta.X < 0 {
		stretched.MinX += delta.X
	} else {
		stretched.MaxX += delta.X
	}
	if delta.Y < 0 {
		stretched.MinY += delta.Y
	} else {
		stretched.MaxY += delta.Y
	}
	if delta.Z < 0 {
		stretched.MinZ += delta.Z
	} else {
		stretched.MaxZ += delta.Z
	}

	return stretched
}

// Intersects reports whether the boxes overlap in a volume. Boxes that only
// share a face do not intersect: an entity resting on the ground touches it
// every tick, and treating that as an overlap would report a standing
// collision forever.
func (b AABB) Intersects(other AABB) bool {
	return b.MinX < other.MaxX && b.MaxX > other.MinX &&
		b.MinY < other.MaxY && b.MaxY > other.MinY &&
		b.MinZ < other.MaxZ && b.MaxZ > other.MinZ
}

// Nearest returns the point of b closest to p, which is p itself when p is
// inside b.
//
// It is the point every reach check measures to. The game measures reach to a
// target's box rather than to its centre, so a client that measures to the
// centre refuses attacks the server would have accepted, and the taller the
// target the wider the disagreement.
func (b AABB) Nearest(p Vec3) Vec3 {
	return Vec3{
		X: clamp(p.X, b.MinX, b.MaxX),
		Y: clamp(p.Y, b.MinY, b.MaxY),
		Z: clamp(p.Z, b.MinZ, b.MaxZ),
	}
}

// Reaches reports whether eye is within reach of b's nearest point.
//
// The eye is a position rather than a body and an offset because eye height is
// a per-version, per-posture number: 1.62 standing in 1.8.9, and something the
// profile supplies in 26.1.2 where a crouched body is shorter. The caller that
// knows which is which passes the point.
//
// The reach distance is an argument for the same reason. The two versions
// disagree about it and it differs by what is being reached for; nothing here
// asserts a value.
//
// The comparison is squared on both sides, so it never takes a root and never
// disagrees with itself about a target exactly on the limit.
func (b AABB) Reaches(eye Vec3, reach float64) bool {
	d := b.Nearest(eye).Sub(eye)

	return d.X*d.X+d.Y*d.Y+d.Z*d.Z <= reach*reach
}

// clamp confines a value to a range. It is not exported: geom's callers want
// Nearest, and a bare clamp is the kind of helper that ends up duplicated in
// three packages.
func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}

	return value
}
