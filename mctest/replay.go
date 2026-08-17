package mctest

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// ErrDiverged reports a replay whose result differs from the recording.
var ErrDiverged = errors.New("mctest: the replay diverged from the recording")

// replayEntity is the body a fixture describes. A fixture holds one.
const replayEntity = entity.ID(1)

// Replay runs a fixture against a profile and reports the first tick that
// disagrees with it.
//
// The comparison is bit for bit and it happens every tick rather than at the
// end, because the first differing tick names the rule that drifted while a
// final position names only the scenario.
func Replay(profile sim.Profile, fixture Fixture) error {
	if got := profile.ID(); got != fixture.Profile {
		return fmt.Errorf("%w: fixture %q was recorded under %s, and the profile is %s",
			ErrFixture, fixture.Name, fixture.Profile, got)
	}

	store, err := buildWorld(profile, fixture.World)
	if err != nil {
		return err
	}

	store.SetEntity(replayEntity, entity.State{
		Family:     entity.FamilyPlayer,
		Box:        fixture.Body.Box,
		Motion:     fixture.Body.Motion,
		OnGround:   fixture.Body.OnGround,
		StepHeight: fixture.Body.StepHeight,
	})
	store.SetLocomotion(replayEntity, movement.Locomotion{
		Yaw:        fixture.Body.Yaw,
		Pitch:      fixture.Body.Pitch,
		MoveSpeed:  fixture.Body.MoveSpeed,
		JumpFactor: fixture.Body.JumpFactor,
	})

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		return fmt.Errorf("mctest: build a kernel: %w", err)
	}
	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{replayEntity}})

	for index, tick := range fixture.Ticks {
		result, err := runner.Step(context.Background(), []sim.Command{movement.Input{
			Entity:  replayEntity,
			Strafe:  tick.Strafe,
			Forward: tick.Forward,
			Yaw:     tick.Yaw,
			Pitch:   tick.Pitch,
			Jump:    tick.Jump,
			Sprint:  tick.Sprint,
			Sneak:   tick.Sneak,
		}})
		if err != nil {
			return fmt.Errorf("mctest: %s tick %d: %w", fixture.Name, index, err)
		}
		if !result.Completeness.Complete {
			// A fixture describes the whole region its trajectory needs, so an
			// incomplete tick means the body left the recorded world rather than
			// that a rule is wrong.
			return fmt.Errorf("%w: %s tick %d left the recorded world: %+v",
				ErrFixture, fixture.Name, index, result.Completeness.Missing)
		}

		got, ok := store.Entities().Entity(replayEntity)
		if !ok {
			return fmt.Errorf("%w: %s tick %d: the body is gone",
				ErrDiverged, fixture.Name, index)
		}

		if got.Box != tick.Box {
			return fmt.Errorf("%w: %s tick %d: body %+v, the recording says %+v",
				ErrDiverged, fixture.Name, index, got.Box, tick.Box)
		}
		if got.Motion != tick.Motion {
			return fmt.Errorf("%w: %s tick %d: motion %+v, the recording says %+v",
				ErrDiverged, fixture.Name, index, got.Motion, tick.Motion)
		}
		if got.OnGround != tick.OnGround {
			return fmt.Errorf("%w: %s tick %d: onGround %v, the recording says %v",
				ErrDiverged, fixture.Name, index, got.OnGround, tick.OnGround)
		}
		if collided := collided(result.Domain); collided != tick.Collided {
			return fmt.Errorf("%w: %s tick %d: collided %v, the recording says %v",
				ErrDiverged, fixture.Name, index, collided, tick.Collided)
		}
	}

	return nil
}

// buildWorld describes a fixture's region to a store.
//
// A scene failure is wrapped as a fixture failure, because from a caller's side
// a world that will not build is a fixture that will not replay.
func buildWorld(profile sim.Profile, described scene.World) (*runtime.Memory, error) {
	store := runtime.NewMemory(profile)
	if err := described.Describe(profile, store.SetBlock); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFixture, err)
	}

	return store, nil
}

// collided reports whether a tick told the world the body hit something
// horizontally.
func collided(events []sim.DomainEvent) bool {
	for _, event := range events {
		if event.Kind == "movement.collided" {
			return true
		}
	}

	return false
}
