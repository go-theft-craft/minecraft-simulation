package replay_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/replay"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// recorded returns a short run over the sample world.
func recorded(t *testing.T, built sim.Profile, ticks int) replay.Recording {
	t.Helper()

	run := setup(t, built)
	for tick := range ticks {
		run.Ticks = append(run.Ticks, replay.Tick{Input: walking(tick)})
	}

	got, err := replay.Record(built, run)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	return got
}

func TestARecordedRunVerifies(t *testing.T) {
	built := profile(t)
	run := recorded(t, built, 30)

	if err := replay.Verify(built, run); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestEveryTickGetsADigest(t *testing.T) {
	built := profile(t)
	run := recorded(t, built, 12)

	if run.Covers() != 12 {
		t.Fatalf("recorded %d ticks, want 12", run.Covers())
	}
	seen := make(map[string]bool, len(run.Ticks))
	for index, tick := range run.Ticks {
		if tick.Digest == "" {
			t.Fatalf("tick %d has no digest", index)
		}
		seen[tick.Digest] = true
	}
	// A body that is actually moving produces a different result every tick. All
	// twelve being equal would mean the run stood still, which is the shape of a
	// recording that proves nothing.
	if len(seen) == 1 {
		t.Fatal("every tick produced the same digest; the run is not moving")
	}
}

func TestRecordingTheSameSetupTwiceAgrees(t *testing.T) {
	// The property the matrix checks, checked locally too. If this fails, the
	// matrix has nothing to say: the disagreement is here, not across platforms.
	built := profile(t)

	first := recorded(t, built, 25)
	second := recorded(t, built, 25)

	for index := range first.Ticks {
		if first.Ticks[index].Digest != second.Ticks[index].Digest {
			t.Fatalf("tick %d differs between two recordings of one setup: %s and %s",
				index, first.Ticks[index].Digest, second.Ticks[index].Digest)
		}
	}
}

func TestACorruptedDigestNamesItsTickAndNoOther(t *testing.T) {
	built := profile(t)
	run := recorded(t, built, 20)
	run.Ticks[7].Digest = strings.Repeat("ab", 32)

	err := replay.Verify(built, run)

	var mismatch *replay.MismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("Verify returned %v, want a mismatch", err)
	}
	if mismatch.Tick != 7 {
		t.Fatalf("the mismatch names tick %d, want 7", mismatch.Tick)
	}
	if mismatch.Want == mismatch.Got {
		t.Fatal("the mismatch reports equal digests")
	}
}

func TestTheMismatchDetailIsActionableFromALogAlone(t *testing.T) {
	// Whoever reads this is looking at a CI log from a runner they cannot reach.
	// The detail has to carry the body, at full precision, because a
	// cross-platform disagreement is a difference in the last bits by
	// construction and a rounded printout shows two identical-looking blocks.
	built := profile(t)
	run := recorded(t, built, 10)
	run.Ticks[3].Digest = strings.Repeat("cd", 32)

	var mismatch *replay.MismatchError
	if !errors.As(replay.Verify(built, run), &mismatch) {
		t.Fatal("Verify did not report a mismatch")
	}

	for _, want := range []string{"entity 1", "box", "motion", "onGround", "locomotion"} {
		if !strings.Contains(mismatch.Detail, want) {
			t.Errorf("the detail does not mention %q:\n%s", want, mismatch.Detail)
		}
	}
	if !strings.Contains(mismatch.Error(), mismatch.Got) {
		t.Error("the error text does not carry the computed digest")
	}
	// Seventeen significant figures: enough to round-trip a float64, which is
	// what "actionable" means for a value that differs in its last bit. Checked
	// by counting digits rather than by matching a literal, because the literal
	// would be a coordinate and coordinates move.
	if got := longestDigitRun(mismatch.Detail); got < 17 {
		t.Errorf("the longest number in the detail has %d digits, want at least 17:\n%s",
			got, mismatch.Detail)
	}
}

// longestDigitRun returns the length of the longest unbroken run of digits,
// which for a rendered float is its significant figures.
func longestDigitRun(text string) int {
	longest, current := 0, 0
	for _, symbol := range text {
		if symbol >= '0' && symbol <= '9' {
			current++
			if current > longest {
				longest = current
			}

			continue
		}
		current = 0
	}

	return longest
}

func TestARunThatLeavesItsDescribedWorldIsRefused(t *testing.T) {
	// Otherwise the recording pins the digest of a tick that changed nothing,
	// and six platforms agree about it while testing none of the arithmetic.
	built := profile(t)

	run := setup(t, built)
	run.World = scene.World{
		Min:  geom.BlockPos{X: -1, Y: -1, Z: -1},
		Max:  geom.BlockPos{X: 1, Y: 1, Z: 1},
		Fill: "air",
	}
	run.Ticks = []replay.Tick{{Input: walking(0)}}

	_, err := replay.Record(built, run)
	if !errors.Is(err, replay.ErrRecording) {
		t.Fatalf("Record returned %v, want a refusal", err)
	}
}

func TestRecordingKeepsTheInputAndReplacesOnlyTheDigests(t *testing.T) {
	// This is how a recording is regenerated after a deliberate change: reload
	// it, re-record it, and the diff is digests alone. An input that moved would
	// mean the new file is evidence about a different run.
	built := profile(t)
	run := recorded(t, built, 8)

	stale := run
	stale.Ticks = make([]replay.Tick, len(run.Ticks))
	copy(stale.Ticks, run.Ticks)
	for index := range stale.Ticks {
		stale.Ticks[index].Digest = "stale"
	}

	again, err := replay.Record(built, stale)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	for index := range again.Ticks {
		if again.Ticks[index].Digest != run.Ticks[index].Digest {
			t.Fatalf("tick %d re-recorded to a different digest", index)
		}
		if again.Ticks[index].Input[0] != run.Ticks[index].Input[0] {
			t.Fatalf("tick %d input changed while re-recording", index)
		}
	}
}
