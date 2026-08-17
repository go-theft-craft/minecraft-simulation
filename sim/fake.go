package sim

import (
	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// StaticProfile is a profile whose answers are fields.
//
// It ships in the non-test build for the same reason world.Blocks does: runtime
// cannot be tested without a profile, and the fixture runner a later milestone
// adds needs one to build scenarios against. It implements no game rule, and no
// official profile is built from it.
type StaticProfile struct {
	// Identity is what ID returns.
	Identity ProfileID
	// PhaseList is what Phases returns, in order.
	PhaseList []Phase
	// Frictions answers Slipperiness per handle.
	Frictions map[world.BlockRef]float64
	// DefaultFriction answers Slipperiness for a handle Frictions does not hold.
	DefaultFriction float64
	// Motions answers Motion per family.
	Motions map[entity.Family]MotionConstants
	// Shapes resolves handles. A handle absent from this map is one this profile
	// did not mint.
	Shapes map[world.BlockRef]geom.Shape
}

// ID implements Profile.
func (s *StaticProfile) ID() ProfileID { return s.Identity }

// Slipperiness implements Profile, falling back to DefaultFriction.
func (s *StaticProfile) Slipperiness(ref world.BlockRef) float64 {
	if friction, ok := s.Frictions[ref]; ok {
		return friction
	}

	return s.DefaultFriction
}

// Motion implements Profile, returning the zero constants for a family it does
// not know.
func (s *StaticProfile) Motion(family entity.Family) MotionConstants {
	return s.Motions[family]
}

// Shape implements Profile.
func (s *StaticProfile) Shape(ref world.BlockRef) (geom.Shape, bool) {
	shape, ok := s.Shapes[ref]

	return shape, ok
}

// Phases implements Profile.
func (s *StaticProfile) Phases() []Phase { return s.PhaseList }
