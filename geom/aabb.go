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
