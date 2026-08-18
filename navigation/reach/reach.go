// Package reach measures how far a body's jump actually carries it, by running
// the profile's own movement kernel over an otherwise empty world.
//
// It is a separate package because it imports sim, movement, runtime, scene,
// and a profile, and navigation may import none of those. The number crosses
// that boundary as a navigation.Capability field rather than as an import.
//
// It is measured rather than tabulated because the two supported versions
// disagree about nearly every constant on the jump path, and because a
// hand-written maximum gap is a number this repository has no way to verify.
// The navigation plan of 2026-08-17 deferred the jump edge for exactly that
// reason:
//
//	Doing it honestly needs a per-profile reach table computed from the
//	movement kernel, which is its own deliverable. A guessed maximum gap would
//	be a number this repository does not verify.
//
// # What each profile measured
//
// Measured 2026-08-18, over 40 ticks, with each profile's own spawned player
// from a standing start on flat ground:
//
//	Profile   Jump        HorizontalBlocks   PeakRise
//	1.8.9     walking             1.455741   1.249187
//	1.8.9     sprinting           2.439107   1.249187
//	26.1.2    walking             1.455741   1.252203
//	26.1.2    sprinting           2.731274   1.252203
//
// These are documentation of a measurement, not constants anything here reads.
// The number a caller routes with comes from calling [Measure], so a change in
// a kernel constant moves it and no transcription has to be remembered. The
// test logs every figure above, which is how the table is refreshed.
//
// Two things in it are worth knowing before routing on them. The walk jump is
// the same distance in both versions and the sprint jump is not: the sprint
// boost is where the two kernels part company, which is also why a table built
// from a shared constant would pass a walk comparison and fail a sprint one.
// And every figure is from a standing start — a body already running clears
// more, so these are the conservative reach, which is the side to be wrong on
// when the number decides whether to jump a hole.
package reach

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// ErrNoLanding reports a jump that did not come back down within the ticks it
// was given.
//
// It is an error rather than a zero table because a table of zero is a body
// that cannot jump at all, and a caller that routed with it would refuse every
// gap while looking like it had measured one.
var ErrNoLanding = errors.New("reach: the body did not land")

// measured is the entity the measurement moves. One body, so the identifier is
// a constant rather than an argument.
const measured = entity.ID(1)

// Table is what one body's jump clears.
type Table struct {
	// HorizontalBlocks is how far the body travels between leaving the ground
	// and landing back on it.
	HorizontalBlocks float64
	// PeakRise is the highest the body's feet get above the take-off height. A
	// jump edge needs it to know what clearance the arc requires.
	PeakRise float64
}

// Body is the body whose jump is measured.
//
// Build its state and locomotion from the profile's own player constructor —
// each supported profile exposes a Spawn — rather than writing a box here. The
// player's dimensions and default attributes are version facts like every
// other, and a measurement that used one version's box for both would be
// reporting the wrong body's arc.
//
// It carries locomotion and a sprint flag alongside the state because a jump is
// not a property of geometry: the distance depends on the movement-speed
// attribute, on the airborne factor, on the facing, and on whether the body is
// sprinting, and only the first two of those live in entity.State. A signature
// that took the state alone could not tell a walk jump from a sprint jump,
// which is the one comparison this package exists to make.
type Body struct {
	// State is the body's geometry and motion at take-off.
	State entity.State
	// Locomotion is its movement state: facing, speed, and jump factor.
	Locomotion movement.Locomotion
	// Sprint holds the sprint input for every tick of the arc. It is held
	// rather than pressed because the game reads it as a state.
	Sprint bool
}

// Sprinting returns the body with its sprint input held.
func (b Body) Sprinting() Body {
	b.Sprint = true

	return b
}

const (
	// span is how far the described region reaches from the origin on each
	// horizontal axis. A sprint jump covers about four blocks, and the region
	// has to hold the whole arc: a tick over an undescribed cell reports itself
	// incomplete rather than moving the body.
	span = 16
	// headroom is how far above the take-off the region reaches. The arc rises
	// about one and a quarter blocks, so this is generous rather than tuned.
	headroom = 16
	// depth is how far below the floor the region reaches. Nothing descends
	// into it; it exists so the floor itself is a described solid rather than
	// the boundary of the description.
	depth = 2
)

// Measure runs the profile's tick over a flat world and reports what one jump
// cleared.
//
// The world is flat and otherwise empty on purpose. This measures the arc, and
// what the arc collides with is the search's question — asked per candidate
// against the real world rather than baked in here.
//
// The jump input is held only until the body leaves the ground. The game turns
// a held jump into a fresh jump the moment the body lands, so holding it for
// the whole run would measure the second arc and report it as the first.
func Measure(profile sim.Profile, body Body, ticks int) (Table, error) {
	if profile == nil {
		return Table{}, errors.New("reach: a measurement needs a profile")
	}
	if ticks <= 0 {
		return Table{}, fmt.Errorf("reach: ticks is %d, and a jump needs at least one", ticks)
	}

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		return Table{}, fmt.Errorf("reach: build a kernel: %w", err)
	}

	feet := body.State.Box.MinY
	store := runtime.NewMemory(profile)
	if err := floor(body.State.Box).Describe(profile, store.SetBlock); err != nil {
		return Table{}, fmt.Errorf("reach: lay the floor: %w", err)
	}
	store.SetEntity(measured, body.State)
	store.SetLocomotion(measured, body.Locomotion)

	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{measured}})

	startX, startZ := horizontalCentre(body.State.Box)

	var table Table
	airborne := false

	for tick := range ticks {
		result, err := runner.Step(context.Background(), []sim.Command{movement.Input{
			Entity:  measured,
			Forward: 1,
			Yaw:     body.Locomotion.Yaw,
			Pitch:   body.Locomotion.Pitch,
			Jump:    !airborne,
			Sprint:  body.Sprint,
		}})
		if err != nil {
			return Table{}, fmt.Errorf("reach: tick %d: %w", tick, err)
		}
		if !result.Completeness.Complete {
			// The region is described well past where any jump reaches, so an
			// incomplete tick means the arc left it — which is a measurement
			// this package cannot report rather than a number to round off.
			return Table{}, fmt.Errorf("reach: tick %d left the described region: %+v",
				tick, result.Completeness.Missing)
		}

		state, ok := store.Entities().Entity(measured)
		if !ok {
			return Table{}, fmt.Errorf("reach: tick %d: the body is gone", tick)
		}

		if rise := state.Box.MinY - feet; rise > table.PeakRise {
			table.PeakRise = rise
		}

		if !airborne {
			airborne = !state.OnGround

			continue
		}
		if !state.OnGround {
			continue
		}

		x, z := horizontalCentre(state.Box)
		table.HorizontalBlocks = math.Hypot(x-startX, z-startZ)

		return table, nil
	}

	return Table{}, fmt.Errorf("%w within %d ticks", ErrNoLanding, ticks)
}

// horizontalCentre returns the middle of a box on the two horizontal axes.
//
// The box is read rather than the position because the two versions disagree
// about which of them is the original — 1.8.9 moves the box and leaves the
// position zero — and the box is the one both of them maintain.
func horizontalCentre(box geom.AABB) (x, z float64) {
	return (box.MinX + box.MaxX) / 2, (box.MinZ + box.MaxZ) / 2
}

// floor describes solid ground up to the body's feet, with air above it.
//
// The region is centred on the body rather than on the origin so that a caller
// may spawn wherever it likes, and it is laid from the body's own feet so that
// a version whose player stands at a different height still starts on the
// ground rather than one block above or inside it.
//
// Stone is named rather than handed over as a handle because a handle is an
// index into the table of whichever profile minted it, and this is called with
// two different profiles.
func floor(box geom.AABB) scene.World {
	x, z := horizontalCentre(box)
	centre := geom.BlockPos{X: int32(math.Floor(x)), Z: int32(math.Floor(z))}
	// The surface sits at the feet, so the topmost solid cell is the one below
	// them. A body whose feet are at y=1 stands on the block at y=0.
	surface := int32(math.Floor(box.MinY))

	return scene.World{
		Min:  geom.BlockPos{X: centre.X - span, Y: surface - depth, Z: centre.Z - span},
		Max:  geom.BlockPos{X: centre.X + span, Y: surface + headroom, Z: centre.Z + span},
		Fill: "air",
		Blocks: []scene.Block{{
			Pos:  geom.BlockPos{X: centre.X - span, Y: surface - depth, Z: centre.Z - span},
			To:   &geom.BlockPos{X: centre.X + span, Y: surface - 1, Z: centre.Z + span},
			Name: "stone",
		}},
	}
}
