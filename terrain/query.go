package terrain

import (
	"github.com/go-theft-craft/minecraft-simulation/collision"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Fit reports whether a body occupies a position.
type Fit uint8

const (
	// FitUnknown means the view could not answer for at least one cell the
	// body would occupy. It is the zero value so that a caller who forgets to
	// switch on it gets the cautious answer.
	FitUnknown Fit = iota
	// FitClear means the body occupies the position without overlapping
	// anything.
	FitClear
	// FitBlocked means something is in the way.
	FitBlocked
)

// Ground reports what is under a body's feet.
type Ground uint8

const (
	// GroundUnknown means the view could not answer.
	GroundUnknown Ground = iota
	// GroundSolid means something holds the body up.
	GroundSolid
	// GroundOpen means nothing does.
	GroundOpen
)

// Query is one body asking about one view.
//
// Facts may be nil. Limit is the collision candidate budget; a non-positive
// value means no limit, matching collision.Gather.
type Query struct {
	View  world.View
	Facts Facts
	Body  Body
	Limit int
}

// Fits reports whether the body occupies the position.
func (q Query) Fits(feet geom.Vec3) (Fit, error) {
	box := q.Body.BoxAt(feet)

	candidates, err := collision.Gather(q.View, box, q.Limit)
	if err != nil {
		return FitUnknown, err
	}
	if len(candidates.Unknown) != 0 {
		return FitUnknown, nil
	}

	for _, other := range candidates.Boxes {
		if box.Intersects(other) {
			return FitBlocked, nil
		}
	}

	return FitClear, nil
}
