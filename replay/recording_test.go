package replay_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gen "github.com/go-theft-craft/minecraft-protocol/generated/java/v1_8"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	v1_8 "github.com/go-theft-craft/minecraft-simulation/profile/java/v1_8"
	"github.com/go-theft-craft/minecraft-simulation/replay"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// profile builds the rules every recording here was made under.
func profile(t *testing.T) sim.Profile {
	t.Helper()

	set, err := gen.Data()
	if err != nil {
		t.Fatalf("load the 1.8.9 data set: %v", err)
	}
	built, err := v1_8.New(set)
	if err != nil {
		t.Fatalf("build the 1.8.9 profile: %v", err)
	}

	return built
}

// setup returns a recording with a world and a body but no ticks: the shape
// Record is given and fills in.
func setup(t *testing.T, built sim.Profile) replay.Recording {
	t.Helper()

	floor := geom.BlockPos{X: 6, Y: 0, Z: 6}
	state, locomotion, ok := v1_8.Spawn(built, geom.Vec3{X: 0.5, Y: 1, Z: 0.5}, 0, 0)
	if !ok {
		t.Fatal("Spawn did not recognize its own profile")
	}

	return replay.Recording{
		Name:       "sample",
		Profile:    built.ID(),
		DataDigest: built.(sim.DataDigest).DataDigest().String(),
		World: scene.World{
			Min:  geom.BlockPos{X: -6, Y: -1, Z: -6},
			Max:  geom.BlockPos{X: 6, Y: 8, Z: 6},
			Fill: "air",
			Blocks: []scene.Block{
				{Pos: geom.BlockPos{X: -6, Y: 0, Z: -6}, To: &floor, Name: "stone"},
			},
		},
		Bodies: []replay.Body{{
			ID:         1,
			Family:     state.Family,
			Box:        state.Box,
			Motion:     state.Motion,
			OnGround:   state.OnGround,
			StepHeight: state.StepHeight,
			Locomotion: locomotion,
		}},
	}
}

// walking returns the intent a recorded run is driven with.
func walking(tick int) []replay.Command {
	return []replay.Command{{
		Kind: "movement.input", Entity: 1, Forward: 1, Yaw: float32(tick) * 11,
	}}
}

func TestARecordingSurvivesBeingWrittenAndRead(t *testing.T) {
	built := profile(t)
	path := filepath.Join(t.TempDir(), "sample.json")

	want := setup(t, built)
	want.Ticks = []replay.Tick{
		{Input: walking(0), Digest: "0f0e"},
		{Input: walking(1), Digest: "1a2b"},
	}
	if err := want.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := replay.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Name != want.Name || got.Profile != want.Profile || got.DataDigest != want.DataDigest {
		t.Errorf("the header changed: %+v", got)
	}
	if len(got.Bodies) != 1 || got.Bodies[0] != want.Bodies[0] {
		t.Errorf("the body came back as %+v, want %+v", got.Bodies, want.Bodies)
	}
	if got.Covers() != 2 {
		t.Fatalf("the recording covers %d ticks, want 2", got.Covers())
	}
	for index, tick := range got.Ticks {
		if tick.Digest != want.Ticks[index].Digest {
			t.Errorf("tick %d digest came back as %q", index, tick.Digest)
		}
		if len(tick.Input) != 1 || tick.Input[0] != want.Ticks[index].Input[0] {
			t.Errorf("tick %d input came back as %+v", index, tick.Input)
		}
	}
	if got.World.Min != want.World.Min || len(got.World.Blocks) != 1 {
		t.Errorf("the world came back as %+v", got.World)
	}
}

func TestSavingTwiceProducesTheSameBytes(t *testing.T) {
	// The recordings are committed and reviewed as diffs. A field order that
	// wandered between saves would make every re-recording unreadable, and a
	// reviewer who cannot read a diff stops reading it.
	built := profile(t)
	recording := setup(t, built)
	recording.Ticks = []replay.Tick{{Input: walking(0), Digest: "0f0e"}}

	directory := t.TempDir()
	first := filepath.Join(directory, "first.json")
	second := filepath.Join(directory, "second.json")
	if err := recording.Save(first); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := recording.Save(second); err != nil {
		t.Fatalf("Save: %v", err)
	}

	left, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	right, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(left, right) {
		t.Fatal("saving the same recording twice produced different bytes")
	}
}

func TestARecordingFromAnotherProfileIsRefusedBeforeSimulating(t *testing.T) {
	built := profile(t)
	recording := setup(t, built)
	recording.Profile.GameVersion = "26.1.2"
	recording.Ticks = []replay.Tick{{Input: walking(0), Digest: "whatever"}}

	err := replay.Verify(built, recording)
	if !errors.Is(err, replay.ErrProfileMismatch) {
		t.Fatalf("Verify returned %v, want a profile mismatch", err)
	}
}

func TestARecordingFromOtherGameDataIsRefusedBeforeSimulating(t *testing.T) {
	// A dataset correction moves the constants without moving the version. The
	// digests would all differ, and without this check the report would be a
	// mismatch at tick zero with nothing to say about why.
	built := profile(t)
	recording := setup(t, built)
	recording.DataDigest = strings.Repeat("00", 32)
	recording.Ticks = []replay.Tick{{Input: walking(0), Digest: "whatever"}}

	err := replay.Verify(built, recording)
	if !errors.Is(err, replay.ErrDataMismatch) {
		t.Fatalf("Verify returned %v, want a data mismatch", err)
	}
}

func TestAnUnknownCommandKindFailsToLoadAndSaysWhich(t *testing.T) {
	// The dangerous alternative is skipping it: the tick would still run, still
	// produce a digest, and that digest would be a plausible wrong answer.
	built := profile(t)
	path := filepath.Join(t.TempDir(), "future.json")

	recording := setup(t, built)
	recording.Ticks = []replay.Tick{{
		Input:  []replay.Command{{Kind: "inventory.click", Entity: 1}},
		Digest: "0f0e",
	}}
	if err := recording.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err := replay.Load(path)
	if !errors.Is(err, replay.ErrRecording) {
		t.Fatalf("Load returned %v, want an invalid-recording error", err)
	}
	if !strings.Contains(err.Error(), "inventory.click") {
		t.Fatalf("the error does not name the kind: %v", err)
	}
}

func TestAnEmptyRecordingLoadsAndReportsThatItCoversNothing(t *testing.T) {
	// It is a legal file and a useless one. Loading is not where that is
	// refused; the suite that gates on these files is, because only it knows
	// what coverage it needs.
	built := profile(t)
	path := filepath.Join(t.TempDir(), "empty.json")

	recording := setup(t, built)
	if err := recording.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := replay.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Covers() != 0 {
		t.Fatalf("an empty recording covers %d ticks", got.Covers())
	}
}

func TestAMissingRecordingIsAnError(t *testing.T) {
	if _, err := replay.Load(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("Load of a file that does not exist returned no error")
	}
}
