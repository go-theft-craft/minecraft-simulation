package runtime

import (
	"context"
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// Runner drives one kernel against one store, one tick at a time.
//
// It holds the state that carries between ticks and nothing else: the tick
// number, the scope, the random state, and the budgets. What a tick reads comes
// from the store, and what it decides goes back through the store's revision
// check.
//
// Runner is not safe for concurrent use.
type Runner struct {
	store  Store
	kernel sim.Kernel

	tick   sim.Tick
	scope  sim.Scope
	random sim.RandomState
	limits sim.Limits
}

// NewRunner returns a runner at tick zero.
func NewRunner(store Store, kernel sim.Kernel) *Runner {
	return &Runner{store: store, kernel: kernel}
}

// Tick returns the next tick number this runner will simulate.
func (r *Runner) Tick() sim.Tick { return r.tick }

// SetLimits bounds the work each tick may do.
func (r *Runner) SetLimits(limits sim.Limits) { r.limits = limits }

// SetScope names what each tick simulates.
func (r *Runner) SetScope(scope sim.Scope) { r.scope = scope }

// SetRandom replaces the random state the next tick draws from.
func (r *Runner) SetRandom(state sim.RandomState) { r.random = state }

// Step simulates one tick, and applies its change set when the result is
// complete.
//
// The tick counter advances whether or not the result was applicable: a tick
// that could not run still happened, and a replay that skipped its number could
// not be lined up against the one that produced it. The revision advances only
// when a change set is applied, which is why the two are separate counters.
//
// An incomplete result is returned to the caller unapplied and without an error.
// The caller is expected to load what the result names as missing and step
// again.
func (r *Runner) Step(ctx context.Context, commands []sim.Command) (sim.TickResult, error) {
	input := sim.TickInput{
		Profile:  r.kernel.Profile(),
		Revision: r.store.Revision(),
		Tick:     r.tick,
		Blocks:   r.store.Blocks(),
		Entities: r.store.Entities(),
		Scope:    r.scope,
		Commands: commands,
		Random:   r.random,
		Limits:   r.limits,
	}

	result, err := r.kernel.Step(ctx, input)
	if err != nil {
		return result, err
	}
	r.tick++
	r.random = result.Random

	if !result.Completeness.Complete {
		return result, nil
	}
	if err := r.store.Apply(result.Changes); err != nil {
		return result, fmt.Errorf("runtime: applying tick %d: %w", result.Tick, err)
	}

	return result, nil
}
