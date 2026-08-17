package sim

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// ErrIncomplete reports that a tick was asked to do something an incomplete
// result cannot support. A tick that merely lacked data is not an error: it
// returns a result whose Completeness names what was missing.
var ErrIncomplete = errors.New("sim: tick is incomplete")

// ErrNaNInResult reports that a tick produced a NaN. A NaN in simulation state
// is a bug: it spreads through every later tick, and no correction brings a body
// back from it. The kernel refuses the result rather than hashing it.
var ErrNaNInResult = errors.New("sim: a NaN reached the tick result")

// Kernel runs one profile's phases and nothing else.
//
// A kernel holds no mutable state: everything a tick reads arrives in its input,
// and everything it produces leaves in its result. One kernel may be stepped by
// one goroutine at a time, and two kernels built from the same profile are
// interchangeable.
type Kernel interface {
	// Profile returns the rules this kernel runs.
	Profile() Profile
	// Step simulates one tick.
	//
	// A tick that lacked data returns a complete result record whose
	// Completeness reports otherwise: no operations, no events, and a digest. A
	// tick that was cancelled, exhausted a budget, or hit a phase error returns
	// an error and a result with no applicable change set.
	Step(ctx context.Context, input TickInput) (TickResult, error)
}

// kernel is the only implementation of Kernel.
type kernel struct {
	profile Profile
	phases  []Phase
}

// NewKernel builds a kernel from a profile.
//
// It rejects a profile with no identity, because a digest that names no rules
// cannot be compared against anything, and it rejects two phases sharing an
// identifier, because a replay that reports "phase move failed" must name one
// phase.
func NewKernel(profile Profile) (Kernel, error) {
	if profile == nil {
		return nil, errors.New("sim: a kernel needs a profile")
	}
	if err := profile.ID().validate(); err != nil {
		return nil, err
	}

	phases := profile.Phases()
	seen := make(map[string]struct{}, len(phases))
	for index, phase := range phases {
		if phase == nil {
			return nil, fmt.Errorf("sim: profile %s has a nil phase at index %d", profile.ID(), index)
		}
		id := phase.ID()
		if id == "" {
			return nil, fmt.Errorf("sim: profile %s has an unnamed phase at index %d", profile.ID(), index)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("sim: profile %s declares phase %q twice", profile.ID(), id)
		}
		seen[id] = struct{}{}
	}

	return &kernel{profile: profile, phases: phases}, nil
}

// Profile implements Kernel.
func (k *kernel) Profile() Profile { return k.profile }

// Step implements Kernel.
func (k *kernel) Step(ctx context.Context, input TickInput) (TickResult, error) {
	if err := k.validateInput(input); err != nil {
		return TickResult{}, err
	}

	state := &TickState{
		profile:    k.profile,
		tick:       input.Tick,
		limits:     input.Limits.withDefaults(),
		scope:      input.Scope,
		blocks:     input.Blocks,
		entities:   input.Entities,
		locomotion: input.Locomotion,
		commands:   input.Commands,
		random:     input.Random.Clone(),
	}

	for _, phase := range k.phases {
		if err := ctx.Err(); err != nil {
			return TickResult{}, err
		}
		if err := phase.Run(ctx, state); err != nil {
			return TickResult{}, fmt.Errorf("sim: phase %s: %w", phase.ID(), err)
		}
		if state.err != nil {
			return TickResult{}, fmt.Errorf("sim: phase %s: %w", phase.ID(), state.err)
		}
	}
	if err := ctx.Err(); err != nil {
		return TickResult{}, err
	}

	result := TickResult{
		Revision:     input.Revision,
		Tick:         input.Tick,
		Changes:      ChangeSet{BaseRevision: input.Revision, Ops: state.ops},
		Domain:       state.domain,
		Presentation: state.presentation,
		Outcomes:     state.outcomes,
		Random:       state.random,
		Read:         sortDependencies(state.read),
		Completeness: Completeness{Complete: true},
	}

	if len(state.missing) != 0 {
		// An incomplete tick keeps none of its work. The caller is expected to
		// load what was missing and run the tick again, and a half-applied tick
		// would leave a store nobody can reason about.
		result.Changes = ChangeSet{BaseRevision: input.Revision}
		result.Domain = nil
		result.Presentation = nil
		result.Completeness = Completeness{Missing: sortDependencies(state.missing)}
	}

	if err := assertNoNaN(result); err != nil {
		return TickResult{}, err
	}
	result.Digest = result.computeDigest(k.profile.ID())

	return result, nil
}

// validateInput reports why a tick cannot run.
func (k *kernel) validateInput(input TickInput) error {
	switch {
	case input.Profile == nil:
		return errors.New("sim: tick input carries no profile")
	case input.Profile.ID() != k.profile.ID():
		return fmt.Errorf("sim: tick input names profile %s, kernel runs %s",
			input.Profile.ID(), k.profile.ID())
	case input.Blocks == nil:
		return errors.New("sim: tick input carries no block view")
	case input.Entities == nil:
		return errors.New("sim: tick input carries no entity view")
	}

	return nil
}

// assertNoNaN refuses a result holding a NaN.
//
// The canonical encoder folds every NaN to one pattern so that a digest stays
// portable, which is a separate concern: this check is what keeps the bug from
// being hashed and stored as though it were a state.
func assertNoNaN(result TickResult) error {
	for index, op := range result.Changes.Ops {
		// A slice rather than a map, so that two runs that both fail name the
		// same field first.
		for _, field := range []struct {
			name  string
			value float64
		}{
			{"motion x", op.State.Motion.X},
			{"motion y", op.State.Motion.Y},
			{"motion z", op.State.Motion.Z},
			{"step height", op.State.StepHeight},
		} {
			if math.IsNaN(field.value) {
				return fmt.Errorf("%w: operation %d entity %d %s",
					ErrNaNInResult, index, op.Entity, field.name)
			}
		}
		if boxHasNaN(op.State.Box) {
			return fmt.Errorf("%w: operation %d entity %d box", ErrNaNInResult, index, op.Entity)
		}
		// The locomotion fields are float32, and a NaN yaw is as fatal as a NaN
		// motion: it indexes the sine table, so it would move the body somewhere
		// arbitrary rather than nowhere.
		for _, field := range []struct {
			name  string
			value float32
		}{
			{"yaw", op.Locomotion.Yaw},
			{"pitch", op.Locomotion.Pitch},
			{"move speed", op.Locomotion.MoveSpeed},
			{"jump factor", op.Locomotion.JumpFactor},
		} {
			if field.value != field.value {
				return fmt.Errorf("%w: operation %d entity %d %s",
					ErrNaNInResult, index, op.Entity, field.name)
			}
		}
	}

	return nil
}

// boxHasNaN reports whether any bound of the box is a NaN.
func boxHasNaN(box geom.AABB) bool {
	return math.IsNaN(box.MinX) || math.IsNaN(box.MinY) || math.IsNaN(box.MinZ) ||
		math.IsNaN(box.MaxX) || math.IsNaN(box.MaxY) || math.IsNaN(box.MaxZ)
}
