package reach_test

import (
	"testing"

	gen "github.com/go-theft-craft/minecraft-protocol/generated/java/v1_8"
	gen26 "github.com/go-theft-craft/minecraft-protocol/generated/java/v26_1"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/navigation/reach"
	v1_8 "github.com/go-theft-craft/minecraft-simulation/profile/java/v1_8"
	v26_1 "github.com/go-theft-craft/minecraft-simulation/profile/java/v26_1"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// version is one profile and the way to spawn its own player.
//
// The spawn is per version rather than a shared body literal because the
// player's box and its default attributes are version facts like every other,
// and a measurement that used one version's body for both would report the
// wrong body's arc under the other's name.
type version struct {
	name  string
	build func(*testing.T) sim.Profile
	spawn func(sim.Profile, geom.Vec3, float32, float32) (entity.State, movement.Locomotion, bool)
}

func versions() []version {
	return []version{
		{name: "1.8.9", build: profile18, spawn: v1_8.Spawn},
		{name: "26.1.2", build: profile26, spawn: v26_1.Spawn},
	}
}

// standing returns a player spawned on the floor Measure lays, facing +Z.
//
// The yaw is zero rather than the captured fixtures' 356.3471 because a
// measurement wants an axis and a capture wants whatever the player happened to
// be doing. The distance is the same either way; the axis makes a failure
// readable.
func (v version) standing(t *testing.T, built sim.Profile) reach.Body {
	t.Helper()

	state, loco, ok := v.spawn(built, geom.Vec3{X: 0.5, Y: 1, Z: 0.5}, 0, 0)
	if !ok {
		t.Fatalf("%s: the profile did not spawn a player", v.name)
	}

	return reach.Body{State: state, Locomotion: loco}
}

// arcTicks is long enough for either version's jump to come back down, and
// short enough that a body which never lands fails rather than hanging.
const arcTicks = 40

func profile18(t *testing.T) sim.Profile {
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

func measure(t *testing.T, profile sim.Profile, body reach.Body) reach.Table {
	t.Helper()

	table, err := reach.Measure(profile, body, arcTicks)
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}

	return table
}

// TestASprintJumpClearsMoreThanAWalkJump pins the ordering any correct
// measurement must produce.
//
// The distances themselves are each profile's, and this deliberately asserts
// none of them. What it asserts is the relation between them, so a table built
// from a broken kernel fails here rather than becoming a routing constant that
// looks plausible.
func TestASprintJumpClearsMoreThanAWalkJump(t *testing.T) {
	t.Parallel()

	for _, version := range versions() {
		t.Run(version.name, func(t *testing.T) {
			t.Parallel()

			built := version.build(t)
			body := version.standing(t, built)
			walk := measure(t, built, body)
			sprint := measure(t, built, body.Sprinting())

			t.Logf("%s walk jump:   horizontal %.6f, peak rise %.6f",
				version.name, walk.HorizontalBlocks, walk.PeakRise)
			t.Logf("%s sprint jump: horizontal %.6f, peak rise %.6f",
				version.name, sprint.HorizontalBlocks, sprint.PeakRise)

			if sprint.HorizontalBlocks <= walk.HorizontalBlocks {
				t.Errorf("sprint jump cleared %v and walk jump cleared %v; want the sprint further",
					sprint.HorizontalBlocks, walk.HorizontalBlocks)
			}
			// A jump that rises nowhere is a measurement of a body that never
			// left the ground, which every other assertion here would pass.
			if walk.PeakRise <= 0 {
				t.Errorf("the walk jump rose %v; want a body that leaves the ground", walk.PeakRise)
			}
		})
	}
}

// TestTheTwoVersionsDisagreeAboutTheReach is the test that catches a table
// reading a shared constant instead of each version's own kernel.
//
// If these came out identical the measurement would not be measuring, which is
// the exact failure the 2026-08-17 plan refused to ship when it deferred this
// edge rather than guessing a maximum gap.
func TestTheTwoVersionsDisagreeAboutTheReach(t *testing.T) {
	t.Parallel()

	all := versions()
	oldBuilt, modernBuilt := all[0].build(t), all[1].build(t)
	old := measure(t, oldBuilt, all[0].standing(t, oldBuilt).Sprinting())
	modern := measure(t, modernBuilt, all[1].standing(t, modernBuilt).Sprinting())

	t.Logf("1.8.9 sprint jump:  horizontal %.9f, peak rise %.9f", old.HorizontalBlocks, old.PeakRise)
	t.Logf("26.1.2 sprint jump: horizontal %.9f, peak rise %.9f", modern.HorizontalBlocks, modern.PeakRise)

	if old == modern {
		t.Fatalf("both versions measured %+v; the table is not reading the kernel", old)
	}
}

// TestMeasureIsDeterministic pins that the number a route is planned against
// does not move between two runs of the same build.
//
// It matters more here than in most places: the reach feeds a Capability, and
// the navigation determinism gate asserts that a thousand identical searches
// return the identical path. A measurement that drifted in its last bits would
// break that gate somewhere far away from here.
func TestMeasureIsDeterministic(t *testing.T) {
	t.Parallel()

	built := profile18(t)
	body := versions()[0].standing(t, built).Sprinting()
	first := measure(t, built, body)

	for range 100 {
		again := measure(t, built, body)
		if again != first {
			t.Fatalf("Measure returned %+v then %+v", first, again)
		}
	}
}

// TestABodyThatCannotLandIsAnError pins that a measurement which does not
// complete reports so rather than returning a zero table.
//
// A zero table is a body that cannot jump at all. A caller handed one would
// refuse every gap in the world while looking like it had measured one, which
// is the failure mode worth an error.
func TestABodyThatCannotLandIsAnError(t *testing.T) {
	t.Parallel()

	built := profile18(t)

	// One tick is enough to leave the ground and not enough to come back.
	if _, err := reach.Measure(built, versions()[0].standing(t, built), 1); err == nil {
		t.Fatal("Measure reported a table for a jump that never landed")
	}
}

// TestMeasureRefusesAnImpossibleRequest pins the two arguments that cannot
// produce a measurement at all.
func TestMeasureRefusesAnImpossibleRequest(t *testing.T) {
	t.Parallel()

	built := profile18(t)
	body := versions()[0].standing(t, built)

	if _, err := reach.Measure(nil, body, arcTicks); err == nil {
		t.Error("Measure accepted a nil profile")
	}
	if _, err := reach.Measure(built, body, 0); err == nil {
		t.Error("Measure accepted a run of no ticks")
	}
}
