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

import "github.com/go-theft-craft/minecraft-simulation/world"

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
// A nil Facts is legal. It answers HazardNone and FluidNone for everything,
// which is what a caller that only cares about geometry wants.
type Facts interface {
	// Hazard reports what a block does to a body occupying it.
	Hazard(ref world.BlockRef) Hazard
	// Fluid reports the fluid a block is.
	Fluid(ref world.BlockRef) Fluid
}
