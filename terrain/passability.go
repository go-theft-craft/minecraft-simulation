package terrain

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// stepRise is the rise a body climbs by leaving the ground: one block. It is
// geometry rather than a version constant — a cell is a unit cube in every
// version — and it is named because an unexplained 1.0 in a movement rule is
// exactly the kind of number this module refuses.
const stepRise = 1.0

// Passability says why a body cannot stand somewhere, because the four answers
// lead to different places.
type Passability uint8

const (
	// Unknown means at least one cell the body needs is undescribed. It is the
	// zero value, and it is deliberately not folded into Blocked: a body that
	// treats every unloaded chunk as a wall gives up at the edge of its render
	// distance.
	Unknown Passability = iota
	// Clear means the body fits and something holds it up.
	Clear
	// Steppable means the body does not fit here but does one block higher,
	// with support. It is something to climb rather than something to avoid.
	Steppable
	// Blocked means the body cannot stand here and cannot climb it. A hole is
	// Blocked too: it is not somewhere to stand, and crossing it is a fall
	// rather than a step.
	Blocked
)

// String returns the value's name.
func (p Passability) String() string {
	switch p {
	case Unknown:
		return "unknown"
	case Clear:
		return "clear"
	case Steppable:
		return "steppable"
	case Blocked:
		return "blocked"
	default:
		return fmt.Sprintf("Passability(%d)", uint8(p))
	}
}

// Passable reports whether the body can stand in a cell.
func (q Query) Passable(cell geom.BlockPos) (Passability, error) {
	feet := FeetOf(cell)

	fit, err := q.Fits(feet)
	if err != nil {
		return Unknown, err
	}
	switch fit {
	case FitUnknown:
		return Unknown, nil
	case FitBlocked:
		return q.stepped(feet)
	case FitClear:
		// The body fits here; whether it can stand depends on the ground.
	}

	ground, err := q.Ground(feet)
	if err != nil {
		return Unknown, err
	}
	switch ground {
	case GroundUnknown:
		return Unknown, nil
	case GroundOpen:
		return Blocked, nil
	case GroundSolid:
		// Fits and supported: the body can stand.
	}

	return Clear, nil
}

// stepped answers whether an obstruction is one block tall and standable on
// top of.
func (q Query) stepped(feet geom.Vec3) (Passability, error) {
	above := feet.Add(geom.Vec3{Y: stepRise})

	fit, err := q.Fits(above)
	if err != nil {
		return Unknown, err
	}
	switch fit {
	case FitUnknown:
		return Unknown, nil
	case FitBlocked:
		return Blocked, nil
	case FitClear:
		// Room one block up; whether it is a step depends on support there.
	}

	ground, err := q.Ground(above)
	if err != nil {
		return Unknown, err
	}
	switch ground {
	case GroundUnknown:
		return Unknown, nil
	case GroundOpen:
		return Blocked, nil
	case GroundSolid:
		// Room above with support: this is a one-block step.
	}

	return Steppable, nil
}
