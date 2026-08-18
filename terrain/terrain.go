// Package terrain answers static questions about a world view: whether a body
// fits somewhere, whether anything holds it up, and what would hurt it.
//
// It owns no version constant. A body's width and step height arrive as a
// value, and every fact about a block that its collision shape does not carry
// arrives through Facts, because a world.BlockRef is opaque and only the
// profile that minted it can say what it names.
//
// Nothing here searches. Composing these answers into a route is navigation's
// job, and keeping the two apart is what lets a mob and a bot share the
// predicates while disagreeing about the route.
package terrain

import (
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Hazard names what makes a position dangerous to occupy.
type Hazard uint8

const (
	// HazardNone means the block does no harm.
	HazardNone Hazard = iota
	// HazardBurn is fire and lava.
	HazardBurn
	// HazardContact is damage on touch, such as cactus.
	HazardContact
)

// Fluid names the fluid filling a cell.
type Fluid uint8

const (
	// FluidNone means the cell holds no fluid.
	FluidNone Fluid = iota
	// FluidWater is water, flowing or still.
	FluidWater
	// FluidLava is lava, flowing or still.
	FluidLava
)

// Facts is what a profile tells terrain about a block that the block's
// collision shape does not already say.
//
// It is an interface rather than a table because world.BlockRef is opaque:
// this package cannot look at a handle and see sand. It is separate from
// sim.Profile for the same reason sim.BlockNames is — a tick never asks these
// questions, and terrain must not import sim.
//
// A nil Facts is legal. It answers HazardNone, FluidNone, not climbable, and
// DoorNone for everything, which is what a caller that only cares about
// geometry wants.
type Facts interface {
	// Hazard reports what a block does to a body occupying it.
	Hazard(ref world.BlockRef) Hazard
	// Fluid reports the fluid a block is.
	Fluid(ref world.BlockRef) Fluid
	// Climbable reports whether a body can climb the column this block
	// occupies — a ladder, a vine, and in later versions several more.
	//
	// It is here rather than derived from a collision shape because a shape
	// cannot say it. A ladder's box is empty, so a caller reading collision
	// alone cannot tell one from air, and the two lead somewhere very
	// different. Like the other two answers it is a fact about a handle only
	// the profile that minted it can resolve.
	Climbable(ref world.BlockRef) bool
	// Door reports whether a block is a door and whether a body may work it.
	//
	// It is a fact rather than a shape for the same reason the others are: a
	// closed door and a wall have the same effect on a collision sweep, and
	// only the profile that minted the handle can tell them apart.
	Door(ref world.BlockRef) Door
}

// Door says whether a block is a door and whether a body may open it.
type Door uint8

const (
	// DoorNone means the block is not a door.
	DoorNone Door = iota
	// DoorOperable is a door a body opens by hand.
	DoorOperable
	// DoorLocked is a door a body cannot open by hand: an iron one, or any
	// door the version gates behind redstone.
	//
	// It is named rather than left out so that a caller can route around one
	// deliberately. A bot that walks into an iron door forever is worse than
	// one that takes the long way, and a door reported as "not a door" would
	// be a wall the search never understood.
	DoorLocked
)

// String returns the value's name.
func (d Door) String() string {
	switch d {
	case DoorNone:
		return "none"
	case DoorOperable:
		return "operable"
	case DoorLocked:
		return "locked"
	default:
		return fmt.Sprintf("Door(%d)", uint8(d))
	}
}
