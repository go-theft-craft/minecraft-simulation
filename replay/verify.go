package replay

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// MismatchError reports a tick whose digest differs from the recording's.
//
// The plan called this type Mismatch. It is MismatchError because the linter
// requires an error type to say so in its name, and being able to spot an error
// type by its name is worth more than the shorter word.
type MismatchError struct {
	// Recording names the run, so a failure in a suite says which file.
	Recording string
	// Tick is the first tick that differed.
	Tick int
	// Want and Got are the recorded and computed digests.
	Want string
	Got  string
	// Detail renders what this build actually produced for that tick.
	//
	// The recording holds digests and not results, so there is nothing to render
	// on the expected side — which is the right trade, because the way a
	// cross-platform mismatch is investigated is by running the same recording on
	// both platforms and diffing these two blocks. A CI log is often the only
	// access anyone has to the machine that disagreed.
	Detail string
}

// Error implements error.
func (m *MismatchError) Error() string {
	return fmt.Sprintf("replay: %s tick %d: digest %s, the recording says %s\n%s",
		m.Recording, m.Tick, m.Got, m.Want, m.Detail)
}

// Verify replays a recording and reports the first tick that disagrees.
//
// It stops there. A run that diverges at tick 12 differs at every tick after it,
// and eighty-eight consequences of one cause bury the cause.
func Verify(profile sim.Profile, recording Recording) error {
	runner, err := start(profile, recording)
	if err != nil {
		return err
	}

	for index, tick := range recording.Ticks {
		result, err := step(runner, tick, index, recording.Name)
		if err != nil {
			return err
		}

		if got := result.Digest.String(); got != tick.Digest {
			return &MismatchError{
				Recording: recording.Name,
				Tick:      index,
				Want:      tick.Digest,
				Got:       got,
				Detail:    describe(result),
			}
		}
	}

	return nil
}

// describe renders a tick result for a human reading a failure.
//
// Every float is printed at full precision. A mismatch here is a difference in
// the last bits by construction — that is what the matrix is looking for — and a
// rounded printout would show two identical-looking blocks and waste the trip.
func describe(result sim.TickResult) string {
	var out strings.Builder

	fmt.Fprintf(&out, "  tick %d, revision %d, complete %t\n",
		result.Tick, result.Revision, result.Completeness.Complete)

	for _, op := range result.Changes.Ops {
		switch op.Kind {
		case sim.OpSetEntity:
			state := op.State
			fmt.Fprintf(&out, "  entity %d box [%s %s %s .. %s %s %s]\n", op.Entity,
				full(state.Box.MinX), full(state.Box.MinY), full(state.Box.MinZ),
				full(state.Box.MaxX), full(state.Box.MaxY), full(state.Box.MaxZ))
			fmt.Fprintf(&out, "  entity %d motion (%s %s %s) onGround %t\n", op.Entity,
				full(state.Motion.X), full(state.Motion.Y), full(state.Motion.Z), state.OnGround)
		case sim.OpSetLocomotion:
			loco := op.Locomotion
			fmt.Fprintf(&out,
				"  entity %d locomotion yaw %s pitch %s jumpTicks %d sprint %t sneak %t jump %t\n",
				op.Entity, single(loco.Yaw), single(loco.Pitch), loco.JumpTicks,
				loco.Sprinting, loco.Sneaking, loco.Jumping)
		case sim.OpSetBlock:
			fmt.Fprintf(&out, "  block %+v -> %d\n", op.Block, op.Ref)
		case sim.OpRemoveEntity:
			fmt.Fprintf(&out, "  entity %d removed\n", op.Entity)
		default:
			fmt.Fprintf(&out, "  op %s entity %d\n", op.Kind, op.Entity)
		}
	}

	for _, event := range result.Domain {
		fmt.Fprintf(&out, "  event %s entity %d\n", event.Kind, event.Entity)
	}

	return out.String()
}

func full(value float64) string { return strconv.FormatFloat(value, 'g', 17, 64) }

func single(value float32) string { return strconv.FormatFloat(float64(value), 'g', 9, 32) }
