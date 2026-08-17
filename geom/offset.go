package geom

// ClampX returns how far mover may still travel along X before it would enter
// b. The receiver is the blocking box and mover is the moving box.
//
// The other two axes must strictly overlap for b to block at all. There is no
// tolerance term: Java Edition 1.8.9 compares the far edges directly, and a
// tolerance here would move every contact position by that amount.
func (b AABB) ClampX(mover AABB, motion float64) float64 {
	if mover.MaxY <= b.MinY || mover.MinY >= b.MaxY {
		return motion
	}
	if mover.MaxZ <= b.MinZ || mover.MinZ >= b.MaxZ {
		return motion
	}

	if motion > 0 && mover.MaxX <= b.MinX {
		if gap := b.MinX - mover.MaxX; gap < motion {
			return gap
		}
	}
	if motion < 0 && mover.MinX >= b.MaxX {
		if gap := b.MaxX - mover.MinX; gap > motion {
			return gap
		}
	}

	return motion
}

// ClampY returns how far mover may still travel along Y before it would enter
// b. See ClampX for the shared rules.
func (b AABB) ClampY(mover AABB, motion float64) float64 {
	if mover.MaxX <= b.MinX || mover.MinX >= b.MaxX {
		return motion
	}
	if mover.MaxZ <= b.MinZ || mover.MinZ >= b.MaxZ {
		return motion
	}

	if motion > 0 && mover.MaxY <= b.MinY {
		if gap := b.MinY - mover.MaxY; gap < motion {
			return gap
		}
	}
	if motion < 0 && mover.MinY >= b.MaxY {
		if gap := b.MaxY - mover.MinY; gap > motion {
			return gap
		}
	}

	return motion
}

// ClampZ returns how far mover may still travel along Z before it would enter
// b. See ClampX for the shared rules.
func (b AABB) ClampZ(mover AABB, motion float64) float64 {
	if mover.MaxX <= b.MinX || mover.MinX >= b.MaxX {
		return motion
	}
	if mover.MaxY <= b.MinY || mover.MinY >= b.MaxY {
		return motion
	}

	if motion > 0 && mover.MaxZ <= b.MinZ {
		if gap := b.MinZ - mover.MaxZ; gap < motion {
			return gap
		}
	}
	if motion < 0 && mover.MinZ >= b.MaxZ {
		if gap := b.MaxZ - mover.MinZ; gap > motion {
			return gap
		}
	}

	return motion
}
