package mctest_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	gen "github.com/go-theft-craft/minecraft-protocol/generated/java/v1_8"
	gen26 "github.com/go-theft-craft/minecraft-protocol/generated/java/v26_1"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mctest"
	v1_8 "github.com/go-theft-craft/minecraft-simulation/profile/java/v1_8"
	v26_1 "github.com/go-theft-craft/minecraft-simulation/profile/java/v26_1"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// profile builds the rules the committed fixtures were recorded under.
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

// TestTheCommittedFixturesReplay is the portable half of the milestone's gate.
//
// It needs no JDK, no jar, and no prepared workspace: the expectations in these
// files are the game's own answers, recorded once by internal/oracle. A rule
// that drifts fails here on any machine, which is what makes the check reachable
// from CI and from a later version's development.
func TestTheCommittedFixturesReplay(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		t.Fatalf("list the fixtures: %v", err)
	}

	// The six scenarios the milestone's exit criterion names. Asserted as a list
	// rather than counted, because a fixture that quietly stopped being written
	// would otherwise turn this test green by being absent.
	want := []string{"collide", "fall", "jump", "sneak", "sprint", "walk"}
	if len(paths) != len(want) {
		t.Fatalf("found %d fixtures, want %d: %v", len(paths), len(want), paths)
	}

	built := profile(t)
	for index, path := range paths {
		name := want[index]
		t.Run(name, func(t *testing.T) {
			fixture, err := mctest.Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if fixture.Name != name {
				t.Fatalf("%s records the scenario %q", path, fixture.Name)
			}
			if err := mctest.Replay(built, fixture); err != nil {
				t.Fatalf("Replay: %v", err)
			}
		})
	}
}

// profile26 builds the rules this version's committed fixtures were recorded
// under.
func profile26(t *testing.T) sim.Profile {
	t.Helper()

	set, err := gen26.Data()
	if err != nil {
		t.Fatalf("load the 26.1.2 data set: %v", err)
	}
	built, err := v26_1.New(set)
	if err != nil {
		t.Fatalf("build the 26.1.2 profile: %v", err)
	}

	return built
}

// TestTheCommitted26FixturesReplay is the same gate for the second version.
//
// It is a separate test rather than a loop over both, because the two suites are
// two milestones' exit criteria and a failure should name which version drifted
// before it names which scenario.
func TestTheCommitted26FixturesReplay(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "26_1", "*.json"))
	if err != nil {
		t.Fatalf("list the fixtures: %v", err)
	}

	want := []string{"collide", "fall", "jump", "sneak", "sprint", "walk"}
	if len(paths) != len(want) {
		t.Fatalf("found %d fixtures, want %d: %v", len(paths), len(want), paths)
	}

	built := profile26(t)
	for index, path := range paths {
		name := want[index]
		t.Run(name, func(t *testing.T) {
			fixture, err := mctest.Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if fixture.Name != name {
				t.Fatalf("%s records the scenario %q", path, fixture.Name)
			}
			if err := mctest.Replay(built, fixture); err != nil {
				t.Fatalf("Replay: %v", err)
			}
		})
	}
}

func TestAFixtureRecordedUnderAnotherProfileIsRefused(t *testing.T) {
	fixture := sample()
	fixture.Profile.GameVersion = "26.1.2"

	err := mctest.Replay(profile(t), fixture)
	if !errors.Is(err, mctest.ErrFixture) {
		t.Fatalf("Replay returned %v, want a refusal", err)
	}
}

func TestAFixtureNamingAnUnknownBlockSaysWhich(t *testing.T) {
	fixture := sample()
	fixture.World.Blocks = append(fixture.World.Blocks, scene.Block{
		Pos: geom.BlockPos{X: 0, Y: 2, Z: 0}, Name: "unobtainium",
	})

	err := mctest.Replay(profile(t), fixture)
	if !errors.Is(err, mctest.ErrFixture) {
		t.Fatalf("Replay returned %v, want a refusal", err)
	}
	if got := err.Error(); !strings.Contains(got, "unobtainium") {
		t.Fatalf("the error does not name the block: %s", got)
	}
}

func TestAWrongExpectationIsReportedAsADivergence(t *testing.T) {
	// The fixture is otherwise a valid one-tick recording, with an expectation
	// that is simply not what the rules produce.
	fixture := sample()
	fixture.Ticks[0].Box.MinY = 42

	err := mctest.Replay(profile(t), fixture)
	if !errors.Is(err, mctest.ErrDiverged) {
		t.Fatalf("Replay returned %v, want a divergence", err)
	}
}
