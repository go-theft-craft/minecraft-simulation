// Package world declares the read-only views the simulation looks at the world
// through, and a deterministic in-memory implementation of them.
//
// A view distinguishes known air from an unknown region. The kernel never
// invents state: when a rule needs a region nobody has loaded, the result says
// so instead of guessing that it is empty.
package world

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// Lookup is the outcome of a block query.
type Lookup uint8

const (
	// LookupUnknown means nobody has told this view what is at the position.
	LookupUnknown Lookup = iota
	// LookupAir means the position is known to hold nothing collidable.
	LookupAir
	// LookupShape means the position holds a block with a collision shape.
	LookupShape
)

// String returns the lookup's name.
func (l Lookup) String() string {
	switch l {
	case LookupUnknown:
		return "unknown"
	case LookupAir:
		return "air"
	case LookupShape:
		return "shape"
	default:
		return fmt.Sprintf("Lookup(%d)", uint8(l))
	}
}

// BlockView answers collision queries about single block cells.
//
// An implementation must be deterministic: the same position must produce the
// same answer for the whole of a tick.
type BlockView interface {
	// CollisionShape returns the shape at pos in block-local coordinates. The
	// shape is empty unless the lookup is LookupShape.
	CollisionShape(pos geom.BlockPos) (geom.Shape, Lookup)
}
