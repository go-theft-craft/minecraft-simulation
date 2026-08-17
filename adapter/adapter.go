// Package adapter is the seam a consumer implements to drive one kernel.
//
// A headless client predicting locally and a server deciding authoritatively run
// the same rules and differ only in what they do with a result: the client
// applies it to a fork it may throw away, the server applies it and tells its
// players. Both differences live behind Source and Sink, and the tick assembly
// they share lives in Drive.
//
// Nothing here knows about packets. Commands and events cross this boundary;
// wire types do not. That is a constraint on the whole simulation module, and
// this package is where a consumer would be tempted to break it.
package adapter

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// The pieces Drive cannot run without. They are separate errors so that a
// consumer wiring itself up gets told which one it forgot.
var (
	errNoKernel = errors.New("driving a tick needs a kernel")
	errNoStore  = errors.New("driving a tick needs a store")
	errNoSource = errors.New("driving a tick needs a source")
	errNoSink   = errors.New("driving a tick needs a sink")
)

// Source is what a consumer contributes to one tick.
//
// A source is asked once per tick, in the order the methods appear here, and it
// must answer consistently within that tick: the same tick number, the same
// commands, the same budgets.
type Source interface {
	// Tick returns the tick number to simulate.
	Tick() sim.Tick
	// Commands returns the intents to apply, in order. The order is what the
	// resulting outcomes are indexed against.
	Commands() []sim.Command
	// Limits bounds the work the tick may do. A zero field means the default.
	Limits() sim.Limits
	// Scope names what the tick simulates.
	Scope() sim.Scope
}

// Sink is what a consumer does with a result.
//
// Observe receives every result, complete or not. A client needs to know a tick
// was incomplete so that it can stop predicting until its chunks arrive, and a
// server needs to log it; a sink that only saw applicable results could do
// neither.
//
// Apply is called only for a complete result. Where the change set goes is the
// consumer's decision: a server applies it to its authoritative store, a client
// applies it to the fork it predicts against.
type Sink interface {
	// Apply writes a complete tick's change set, or reports why it did not.
	Apply(changes sim.ChangeSet) error
	// Observe records a result, applicable or not.
	Observe(result sim.TickResult)
}

// Drive simulates one tick.
//
// It assembles the input from the store's revision and views and from the
// source, steps the kernel, hands the result to the sink, and applies the change
// set only when the result is complete. It is the one piece of logic both
// consumers share, and it is deliberately small: everything version-specific,
// network-specific, or policy-specific is on the far side of Source and Sink.
//
// A cancelled context returns without stepping. An incomplete result is observed
// and not applied, and it is not an error: the caller is expected to load what
// the result names as missing and drive the tick again.
//
// Random state is not part of this seam. No rule the simulation has yet draws
// from one, and a consumer that needs a sequence of ticks to share a generator
// drives runtime.Runner, which carries the state between them. When a rule does
// draw, the state belongs on Source and Sink alongside the commands and the
// change set rather than hidden inside Drive.
func Drive(
	ctx context.Context,
	kernel sim.Kernel,
	store runtime.Store,
	source Source,
	sink Sink,
) (sim.TickResult, error) {
	switch {
	case kernel == nil:
		return sim.TickResult{}, fmt.Errorf("adapter: %w", errNoKernel)
	case store == nil:
		return sim.TickResult{}, fmt.Errorf("adapter: %w", errNoStore)
	case source == nil:
		return sim.TickResult{}, fmt.Errorf("adapter: %w", errNoSource)
	case sink == nil:
		return sim.TickResult{}, fmt.Errorf("adapter: %w", errNoSink)
	}
	if err := ctx.Err(); err != nil {
		return sim.TickResult{}, err
	}

	input := sim.TickInput{
		Profile:  kernel.Profile(),
		Revision: store.Revision(),
		Tick:     source.Tick(),
		Blocks:   store.Blocks(),
		Entities: store.Entities(),
		Scope:    source.Scope(),
		Commands: source.Commands(),
		Limits:   source.Limits(),
	}

	result, err := kernel.Step(ctx, input)
	if err != nil {
		return result, fmt.Errorf("adapter: tick %d: %w", input.Tick, err)
	}

	sink.Observe(result)
	if !result.Completeness.Complete {
		return result, nil
	}
	if err := sink.Apply(result.Changes); err != nil {
		return result, fmt.Errorf("adapter: applying tick %d: %w", result.Tick, err)
	}

	return result, nil
}
