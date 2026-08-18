// Package combat_test holds the shared-arithmetic tests and the helpers every
// file here uses.
//
// It is an external test package because the profiles it needs import combat,
// and a test inside the package naming them would be an import cycle.
package combat_test

import (
	"testing"

	gen "github.com/go-theft-craft/minecraft-protocol/generated/java/v1_8"
	gen26 "github.com/go-theft-craft/minecraft-protocol/generated/java/v26_1"

	"github.com/go-theft-craft/minecraft-simulation/combat"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	v1_8 "github.com/go-theft-craft/minecraft-simulation/profile/java/v1_8"
	v26_1 "github.com/go-theft-craft/minecraft-simulation/profile/java/v26_1"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// versions are the two lanes, and there are two of them on purpose: a stage
// that checks one version and skips the other is the failure this milestone
// was subdivided to prevent.
var versions = []string{"1_8_9", "26_1_2"}

// profileFor builds the real profile for one version and asserts it fights.
func profileFor(t *testing.T, version string) combat.Fighter {
	t.Helper()

	built := simProfileFor(t, version)

	fighter, ok := built.(combat.Fighter)
	if !ok {
		t.Fatalf("the %s profile declares no combat rules", version)
	}

	return fighter
}

// simProfileFor builds the real profile for one version.
func simProfileFor(t *testing.T, version string) sim.Profile {
	t.Helper()

	switch version {
	case "1_8_9":
		set, err := gen.Data()
		if err != nil {
			t.Fatalf("load the 1.8.9 data set: %v", err)
		}
		built, err := v1_8.New(set)
		if err != nil {
			t.Fatalf("v1_8.New: %v", err)
		}

		return built
	case "26_1_2":
		set, err := gen26.Data()
		if err != nil {
			t.Fatalf("load the 26.1.2 data set: %v", err)
		}
		built, err := v26_1.New(set)
		if err != nil {
			t.Fatalf("v26_1.New: %v", err)
		}

		return built
	default:
		t.Fatalf("no such version %q", version)

		return nil
	}
}

// eye is where every reach test measures from: a standing player's eye.
var eye = geom.Vec3{X: 0, Y: 1.62, Z: 0}

// boxAt returns a player-sized box whose nearest point to the test eye is
// exactly distance blocks away along +X.
func boxAt(t *testing.T, distance, y, z float64) geom.AABB {
	t.Helper()

	return geom.AABB{
		MinX: distance, MinY: y, MinZ: z - 0.3,
		MaxX: distance + 0.6, MaxY: y + 1.8, MaxZ: z + 0.3,
	}
}

func TestReachIsMeasuredToTheNearestPointOfTheBox(t *testing.T) {
	t.Parallel()

	// A standing player whose box runs from x 2.7 to 3.3. The centre is
	// sqrt(3² + 0.72²) ≈ 3.085 blocks from the eye — out of reach at 3.0 —
	// while the nearest point is 2.7 blocks away. Measuring to the centre
	// makes this target unhittable, which looks like a reach bug and is a
	// geometry bug.
	target := boxAt(t, 2.7, 0, 0)

	if !combat.InReach(eye, target, 3.0) {
		t.Fatal("a box whose nearest point is 2.7 blocks away was out of reach at 3.0")
	}
}

func TestAnEntityJustBeyondReachIsRefused(t *testing.T) {
	t.Parallel()

	// The boundary is where anti-cheats look, so the test sits on it rather
	// than well inside it.
	if combat.InReach(eye, boxAt(t, 3.01, 0.9, 0), 3.0) {
		t.Fatal("a box 3.01 blocks away was in reach at 3.0")
	}
	if !combat.InReach(eye, boxAt(t, 2.99, 0.9, 0), 3.0) {
		t.Fatal("a box 2.99 blocks away was out of reach at 3.0")
	}
}

func TestAVerticalOffsetCountsAgainstReach(t *testing.T) {
	t.Parallel()

	// A target 2.5 blocks out and 4.1 below the eye is within reach measured
	// on the ground plane alone and ≈4.8 blocks away measured to its box.
	// Reach is a distance, not a radius on the ground.
	below := geom.AABB{
		MinX: 2.5, MinY: -4.3, MinZ: -0.3,
		MaxX: 3.1, MaxY: -2.5, MaxZ: 0.3,
	}

	if combat.InReach(eye, below, 3.0) {
		t.Fatal("a box 4.8 blocks away diagonally was in reach at 3.0")
	}
}

func TestEachProfileDeclaresItsOwnReach(t *testing.T) {
	t.Parallel()

	// The numbers differ between versions and between modes. A shared
	// constant would be wrong on one of them and nobody would notice until an
	// anti-cheat did.
	for _, version := range versions {
		fighter := profileFor(t, version)

		for mode, r := range map[string]combat.Reach{
			"survival": fighter.Reach(),
			"creative": fighter.CreativeReach(),
		} {
			if r.Attack <= 0 || r.Interact <= 0 {
				t.Fatalf("the %s profile declared %s reach %+v", version, mode, r)
			}
		}
	}
}

func TestTheReachNumbersAreTheOnesTheGamesUse(t *testing.T) {
	t.Parallel()

	// Pinned so a regenerated dataset or an edited constant cannot move a
	// reach silently. The sources are recorded beside each number's
	// declaration.
	cases := []struct {
		version            string
		survival, creative combat.Reach
	}{
		{"1_8_9", combat.Reach{Attack: 3, Interact: 4.5}, combat.Reach{Attack: 6, Interact: 5}},
		{"26_1_2", combat.Reach{Attack: 3, Interact: 4.5}, combat.Reach{Attack: 5, Interact: 5}},
	}
	for _, c := range cases {
		fighter := profileFor(t, c.version)
		if got := fighter.Reach(); got != c.survival {
			t.Errorf("%s survival reach = %+v, want %+v", c.version, got, c.survival)
		}
		if got := fighter.CreativeReach(); got != c.creative {
			t.Errorf("%s creative reach = %+v, want %+v", c.version, got, c.creative)
		}
	}
}
