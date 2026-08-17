package replay

import (
	"context"
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// Record runs a setup and fills in the digest every tick produced.
//
// The setup carries the world, the bodies, the limits, the random state, and
// each tick's input; the digests it arrives with are ignored and the returned
// recording carries the ones this build computed. That is what makes
// re-recording a loaded file the natural way to regenerate one: the inputs stay
// exactly as reviewed, and only the answers move.
//
// Which is also why regenerating is never the fix for a failing matrix. These
// digests are this module's own output, not the game's — the oracle is what
// checks the numbers against vanilla — so the only thing the matrix can tell us
// is whether six platforms compute them identically. A recording refreshed to
// make a red matrix green converts that check into a rubber stamp.
func Record(profile sim.Profile, setup Recording) (Recording, error) {
	runner, err := start(profile, setup)
	if err != nil {
		return Recording{}, err
	}

	recorded := setup
	recorded.Ticks = make([]Tick, 0, len(setup.Ticks))
	for index, tick := range setup.Ticks {
		result, err := step(runner, tick, index, setup.Name)
		if err != nil {
			return Recording{}, err
		}
		recorded.Ticks = append(recorded.Ticks, Tick{
			Input:  tick.Input,
			Digest: result.Digest.String(),
		})
	}

	return recorded, nil
}

// start builds the world, places the bodies, and returns a runner over them.
func start(profile sim.Profile, setup Recording) (*runtime.Runner, error) {
	if err := setup.check(profile); err != nil {
		return nil, err
	}

	store := runtime.NewMemory(profile)
	if err := setup.World.Describe(profile, store.SetBlock); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRecording, err)
	}

	scope := sim.Scope{Entities: make([]entity.ID, 0, len(setup.Bodies))}
	for _, body := range setup.Bodies {
		store.SetEntity(body.ID, entity.State{
			Family:     body.Family,
			Box:        body.Box,
			Motion:     body.Motion,
			OnGround:   body.OnGround,
			StepHeight: body.StepHeight,
		})
		store.SetLocomotion(body.ID, body.Locomotion)
		// The scope keeps the recording's order. The kernel walks it in order and
		// the digest covers it, so two recordings differing only in body order
		// are different runs.
		scope.Entities = append(scope.Entities, body.ID)
	}

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		return nil, fmt.Errorf("replay: build a kernel: %w", err)
	}

	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(scope)
	runner.SetLimits(setup.Limits)
	runner.SetRandom(setup.Random)

	return runner, nil
}

// step runs one recorded tick.
//
// An incomplete tick is refused rather than digested. A recording whose body
// wandered out of its described region would otherwise pin the digest of a tick
// that changed nothing, and the matrix would agree about it on six platforms
// while testing none of the arithmetic it exists for.
func step(runner *runtime.Runner, tick Tick, index int, name string) (sim.TickResult, error) {
	commands := make([]sim.Command, 0, len(tick.Input))
	for _, recorded := range tick.Input {
		command, err := recorded.command()
		if err != nil {
			return sim.TickResult{}, fmt.Errorf("%s tick %d: %w", name, index, err)
		}
		commands = append(commands, command)
	}

	result, err := runner.Step(context.Background(), commands)
	if err != nil {
		return sim.TickResult{}, fmt.Errorf("replay: %s tick %d: %w", name, index, err)
	}
	if !result.Completeness.Complete {
		return sim.TickResult{}, fmt.Errorf(
			"%w: %s tick %d left the described world: %+v",
			ErrRecording, name, index, result.Completeness.Missing,
		)
	}

	return result, nil
}
