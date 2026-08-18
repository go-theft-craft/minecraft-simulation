package placement

import (
	"math"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mining"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Placer is a profile that can say what block state results from a placement.
//
// It is version-owned because the addressing is. 1.8.9 names a state as a block
// id plus four bits of metadata; 26.1.2 names it as a flat id inside the
// block's own range, with the properties in a published order. There is no
// shared representation that is not a lie about one of them.
//
// It is optional for the reason sim.BlockNames is: nothing inside a tick calls
// it — the phase is handed a resolved state — and a profile assembled in a test
// has no block table to place from. A caller asserts for it and reports a
// profile that cannot answer, rather than every profile being obliged to carry
// placement rules.
//
// It is declared here rather than in sim because placement imports sim, and an
// interface in sim returning a placement.Target would be an import cycle.
type Placer interface {
	// PlacedState returns the state one placement produces.
	//
	// cursor is where in the clicked cell the click landed, in block-local
	// coordinates. The plan for this stage passed the player's eye instead; the
	// eye cannot answer, because a slab's half is decided by the click's own
	// height within the face — BlockSlab.onBlockPlaced reads hitY, and two
	// clicks from one eye position land in different halves.
	PlacedState(
		item data.ItemID, target Target, face mining.Face, yaw float32, cursor geom.Vec3,
	) (world.BlockRef, error)
}

// Facing is the horizontal direction a placed block takes.
//
// The four values are the game's own horizontal directions and nothing more.
// How a version numbers them is the version's business; this says which way the
// block points.
type Facing uint8

const (
	// FacingNorth is -Z.
	FacingNorth Facing = iota
	// FacingSouth is +Z.
	FacingSouth
	// FacingWest is -X.
	FacingWest
	// FacingEast is +X.
	FacingEast
)

// String returns the facing's name.
func (f Facing) String() string {
	switch f {
	case FacingNorth:
		return "north"
	case FacingSouth:
		return "south"
	case FacingWest:
		return "west"
	case FacingEast:
		return "east"
	default:
		return "facing(?)"
	}
}

// FacingFromYaw is the direction a player at this yaw is looking along.
//
// Transcribed from EnumFacing.fromAngle on 1.8.9 and Direction.fromYRot on
// 26.1.2, which are the same arithmetic: the yaw is divided into four quadrants
// with a half-quadrant offset, and the result wraps. Both games place a stair,
// a furnace, and a chest by it.
//
// The quadrant order is the game's: zero yaw looks south, and it turns
// clockwise through west, north, and east.
func FacingFromYaw(yaw float32) Facing {
	quadrant := int(math.Floor(float64(yaw)/90+0.5)) & 3
	switch quadrant {
	case 0:
		return FacingSouth
	case 1:
		return FacingWest
	case 2:
		return FacingNorth
	default:
		return FacingEast
	}
}

// Half is which half of a cell a slab or a stair occupies.
type Half uint8

const (
	// HalfBottom is the lower half.
	HalfBottom Half = iota
	// HalfTop is the upper half.
	HalfTop
)

// String returns the half's name.
func (h Half) String() string {
	if h == HalfTop {
		return "top"
	}

	return "bottom"
}

// HalfFor is the half a click puts a slab or a stair in.
//
// Transcribed from BlockSlab.onBlockPlaced and BlockStairs.onBlockPlaced on
// 1.8.9, and from SlabBlock and StairBlock's placement on 26.1.2, which agree:
// clicking the underside of a block makes a top half, clicking the top makes a
// bottom half, and clicking a side is decided by how high in the face the click
// landed.
//
// The comparison is against a half exactly, and it is not symmetric: the game
// writes hitY <= 0.5 for the bottom half, so a click exactly halfway up a side
// places a bottom slab.
func HalfFor(face mining.Face, cursor geom.Vec3) Half {
	switch face {
	case mining.FaceBottom:
		return HalfTop
	case mining.FaceTop:
		return HalfBottom
	case mining.FaceNorth, mining.FaceSouth, mining.FaceWest, mining.FaceEast:
		if cursor.Y > 0.5 {
			return HalfTop
		}

		return HalfBottom
	default:
		return HalfBottom
	}
}

// Axis is the direction a pillar block lies along.
type Axis uint8

const (
	// AxisX runs east to west.
	AxisX Axis = iota
	// AxisY runs up and down.
	AxisY
	// AxisZ runs north to south.
	AxisZ
)

// String returns the axis's name.
func (a Axis) String() string {
	switch a {
	case AxisX:
		return "x"
	case AxisY:
		return "y"
	case AxisZ:
		return "z"
	default:
		return "axis(?)"
	}
}

// AxisFor is the axis a log takes when placed against a face.
//
// A log lies along the axis of the face it was placed against: clicking the top
// or the bottom of a block stands it upright, clicking a side lays it down
// pointing at the player. Both editions decide it from the face alone.
func AxisFor(face mining.Face) Axis {
	switch face {
	case mining.FaceBottom, mining.FaceTop:
		return AxisY
	case mining.FaceNorth, mining.FaceSouth:
		return AxisZ
	case mining.FaceWest, mining.FaceEast:
		return AxisX
	default:
		return AxisY
	}
}
