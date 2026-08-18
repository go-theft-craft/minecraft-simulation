package movement

import "github.com/go-theft-craft/minecraft-simulation/geom"

// Box returns the collision box a body of these dimensions stands in at a
// position.
//
// This is one of the few rules the two versions this module carries provably
// share, and it is shared for a reason worth stating: it is not that the numbers
// happen to match, but that both versions build a box the same way out of two
// floats. The width is halved at single width and the halves are added to a
// double position, so a body 0.6 wide does not stand in a box 0.6 wide — it
// stands in one a sixteenth of a millionth wider, and every collision it has is
// decided by those edges. M8.2's oracle caught that in 1.8.9 before a single
// rule had run, and M8.7's caught it again in 26.1.2.
//
// The dimensions stay with the profile that owns them. What is shared is the
// arithmetic, because both versions were measured doing exactly this.
func Box(pos geom.Vec3, width, height float32) geom.AABB {
	half := float64(width / 2)

	return geom.AABB{
		MinX: pos.X - half, MinY: pos.Y, MinZ: pos.Z - half,
		MaxX: pos.X + half, MaxY: pos.Y + float64(height), MaxZ: pos.Z + half,
	}
}
