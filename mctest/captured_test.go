package mctest_test

import (
	"testing"

	gen "github.com/go-theft-craft/minecraft-protocol/generated/java/v1_8"
	gen26 "github.com/go-theft-craft/minecraft-protocol/generated/java/v26_1"

	"github.com/go-theft-craft/minecraft-simulation/mctest"
	v1_8 "github.com/go-theft-craft/minecraft-simulation/profile/java/v1_8"
	v26_1 "github.com/go-theft-craft/minecraft-simulation/profile/java/v26_1"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// capturedDir holds one trajectory per family per version, extracted from
// recordings taken through the capture proxy in front of a pinned server.
const capturedDir = "testdata/captured"

// TestCapturedTrajectoriesReplay is M9.2's second gate: the kernel against a
// real server's own account of what happened, at the checkpoints that server
// chose to send.
//
// It cannot say what a single tick did — a tracker states an item's position
// once every twenty ticks — and it is the only gate that runs against the game
// as it is actually played, over a connection, through the server's encoder.
func TestCapturedTrajectoriesReplay(t *testing.T) {
	t.Parallel()

	captures, err := mctest.LoadCapturedDir(capturedDir)
	if err != nil {
		t.Fatalf("LoadCapturedDir: %v", err)
	}
	if len(captures) == 0 {
		t.Fatal("no captured trajectories; this gate is not running")
	}

	profiles := map[sim.ProfileID]sim.Profile{
		v1_8.Identity:  built(t, false),
		v26_1.Identity: built(t, true),
	}

	for _, captured := range captures {
		t.Run(captured.Name, func(t *testing.T) {
			if captured.Absent != "" {
				t.Skipf("declared absent: %s", captured.Absent)
			}

			profile, known := profiles[captured.Profile]
			if !known {
				t.Fatalf("the capture names profile %s, which this test does not build",
					captured.Profile)
			}
			if profile.Motion(captured.Family) == (sim.MotionConstants{}) {
				// This was a skip while 26.1's item and arrow constants were
				// transcribed but unreleased. They shipped in
				// minecraft-protocol v0.6.0, so a family with no constants is
				// now a defect in the dataset or the profile rather than a
				// lane waiting on a release.
				t.Fatalf("the profile carries no %s constants; this lane cannot run",
					captured.Family)
			}

			worst, err := mctest.ReplayCaptured(profile, captured)
			if err != nil {
				t.Fatalf("ReplayCaptured: %v", err)
			}
			if worst > captured.Tolerance {
				t.Fatalf("the trajectory diverged by %v blocks, and %v is what the wire "+
					"allows.\n%s", worst, captured.Tolerance, captured.Because)
			}

			t.Logf("%d checkpoints, worst deviation %v of %v allowed",
				len(captured.Samples), worst, captured.Tolerance)
		})
	}
}

// TestBothVersionsHaveACapturedLane is the rule M9.1b's harness enforces for
// the gates that run inside the capture repository, restated here for the ones
// that run inside this one.
//
// A mechanic checked on one version claims nothing about the other. A version
// with no lane fails; a version whose lane is declared absent passes and says
// why, which is a different thing from nobody having looked.
func TestBothVersionsHaveACapturedLane(t *testing.T) {
	t.Parallel()

	captures, err := mctest.LoadCapturedDir(capturedDir)
	if err != nil {
		t.Fatalf("LoadCapturedDir: %v", err)
	}

	families := map[string]map[sim.ProfileID]bool{}
	for _, captured := range captures {
		family := captured.Family.String()
		if families[family] == nil {
			families[family] = map[sim.ProfileID]bool{}
		}
		families[family][captured.Profile] = true
	}

	if len(families) == 0 {
		t.Fatal("no captured trajectories at all")
	}
	for family, versions := range families {
		for _, want := range []sim.ProfileID{v1_8.Identity, v26_1.Identity} {
			if !versions[want] {
				t.Errorf("%s has no captured lane for %s; a family checked on one "+
					"version claims nothing about the other", family, want)
			}
		}
	}
}

func built(t *testing.T, modern bool) sim.Profile {
	t.Helper()

	if modern {
		set, err := gen26.Data()
		if err != nil {
			t.Fatalf("load the 26.1 data set: %v", err)
		}
		profile, err := v26_1.New(set)
		if err != nil {
			t.Fatalf("build the 26.1 profile: %v", err)
		}

		return profile
	}

	set, err := gen.Data()
	if err != nil {
		t.Fatalf("load the 1.8.9 data set: %v", err)
	}
	profile, err := v1_8.New(set)
	if err != nil {
		t.Fatalf("build the 1.8.9 profile: %v", err)
	}

	return profile
}
