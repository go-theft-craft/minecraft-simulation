package mctest_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mctest"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// sample is a fixture small enough to read, with the values that a careless
// encoding would round: a widened float box and a motion in its last bits.
func sample() mctest.Fixture {
	to := geom.BlockPos{X: 2, Y: 0, Z: 2}

	return mctest.Fixture{
		Name:    "sample",
		Profile: sim.ProfileID{Edition: "java", GameVersion: "1.8.9", RulesRevision: "1"},
		Source:  "written by hand for a round-trip test",
		World: mctest.World{
			Min:  geom.BlockPos{X: -2, Y: -1, Z: -2},
			Max:  geom.BlockPos{X: 2, Y: 4, Z: 2},
			Fill: "air",
			Blocks: []mctest.Block{
				{Pos: geom.BlockPos{X: -2, Y: 0, Z: -2}, To: &to, Name: "stone"},
				{Pos: geom.BlockPos{X: 1, Y: 1, Z: 1}, Name: "ice"},
			},
		},
		Body: mctest.Body{
			Box: geom.AABB{
				MinX: 0.19999998807907104, MinY: 1, MinZ: 0.19999998807907104,
				MaxX: 0.800000011920929, MaxY: 2.799999952316284, MaxZ: 0.800000011920929,
			},
			OnGround:   true,
			StepHeight: float64(float32(0.6)),
			MoveSpeed:  0.1,
			JumpFactor: 0.02,
		},
		Ticks: []mctest.Tick{{
			Forward:  1,
			Yaw:      -347.3353,
			Motion:   geom.Vec3{X: -0.0021874999510869384, Y: -0.078400001525878907},
			OnGround: true,
		}},
	}
}

func TestAFixtureSurvivesBeingWrittenAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.json")

	want := sample()
	if err := want.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := mctest.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Compared field by field rather than with reflect.DeepEqual on the whole,
	// because what this test is really about is the numbers: a fixture whose
	// floats came back rounded would still be deeply equal in shape and would
	// silently stop recording the game.
	if got.Name != want.Name || got.Profile != want.Profile || got.Source != want.Source {
		t.Errorf("the header changed: %+v", got)
	}
	if got.Body != want.Body {
		t.Errorf("the body came back as %+v, want %+v", got.Body, want.Body)
	}
	if len(got.Ticks) != 1 || got.Ticks[0] != want.Ticks[0] {
		t.Errorf("the ticks came back as %+v, want %+v", got.Ticks, want.Ticks)
	}
	if got.World.Min != want.World.Min || got.World.Max != want.World.Max {
		t.Errorf("the region came back as %+v", got.World)
	}
	if len(got.World.Blocks) != 2 {
		t.Fatalf("the world came back with %d entries, want 2", len(got.World.Blocks))
	}
	if got.World.Blocks[0].To == nil || *got.World.Blocks[0].To != *want.World.Blocks[0].To {
		t.Errorf("the span lost its far corner: %+v", got.World.Blocks[0])
	}
	if got.World.Blocks[1].To != nil {
		t.Errorf("a single cell came back as a span: %+v", got.World.Blocks[1])
	}
}

func TestAFixtureWithNoTicksIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")

	empty := sample()
	empty.Ticks = nil
	if err := empty.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := mctest.Load(path); !errors.Is(err, mctest.ErrFixture) {
		t.Fatalf("Load returned %v, want an invalid-fixture error", err)
	}
}

func TestAMissingFixtureIsAnError(t *testing.T) {
	if _, err := mctest.Load(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("Load of a file that does not exist returned no error")
	}
}
