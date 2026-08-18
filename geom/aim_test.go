package geom

import (
	"math"
	"testing"
)

func TestBehindIsOppositeTheFacing(t *testing.T) {
	target := Vec3{X: 10, Y: 64, Z: 10}
	facing := Vec3{Z: 1} // facing south

	if got, want := Behind(target, facing, 3), (Vec3{X: 10, Y: 64, Z: 7}); got != want {
		t.Fatalf("Behind = %v, want %v", got, want)
	}
}

// TestBehindIgnoresTheFacingMagnitude pins that a caller may hand over a
// velocity or a look vector without normalising first.
func TestBehindIgnoresTheFacingMagnitude(t *testing.T) {
	var target Vec3

	unit := Behind(target, Vec3{Z: 1}, 2)
	long := Behind(target, Vec3{Z: 17}, 2)

	if unit != long {
		t.Fatalf("Behind with a long facing = %v, want %v", long, unit)
	}
}

// TestBehindAZeroFacingReturnsTheTarget pins the refusal to invent a direction.
//
// A body that faces nowhere has no behind, and picking one would put a caller
// somewhere it never asked to be.
func TestBehindAZeroFacingReturnsTheTarget(t *testing.T) {
	target := Vec3{X: 1, Y: 2, Z: 3}

	if got := Behind(target, Vec3{}, 5); got != target {
		t.Fatalf("Behind with no facing = %v, want %v", got, target)
	}
}

func TestLeadProjectsTheTargetForward(t *testing.T) {
	target := Vec3{X: 0, Y: 64, Z: 0}
	velocity := Vec3{X: 0.2, Z: -0.1}

	got := Lead(target, velocity, 10)
	if math.Abs(got.X-2) > 1e-9 || got.Y != 64 || math.Abs(got.Z+1) > 1e-9 {
		t.Fatalf("Lead = %v, want {2 64 -1}", got)
	}
}

func TestLeadWithNoVelocityReturnsTheTarget(t *testing.T) {
	target := Vec3{X: 1, Y: 2, Z: 3}

	if got := Lead(target, Vec3{}, 40); got != target {
		t.Fatalf("Lead with no velocity = %v, want %v", got, target)
	}
}

// TestTangentIsPerpendicularToTheRadius is the definition, checked as one.
func TestTangentIsPerpendicularToTheRadius(t *testing.T) {
	centre := Vec3{X: 0, Y: 64, Z: 0}
	here := Vec3{X: 5, Y: 64, Z: 0}

	got := Tangent(centre, here, true)

	radius := here.Sub(centre)
	if dot := got.X*radius.X + got.Z*radius.Z; math.Abs(dot) > 1e-9 {
		t.Fatalf("tangent %v is not perpendicular to radius %v, dot = %v", got, radius, dot)
	}
	if length := math.Hypot(got.X, got.Z); math.Abs(length-1) > 1e-9 {
		t.Fatalf("tangent %v has length %v, want 1", got, length)
	}
}

func TestTangentReversesWithDirection(t *testing.T) {
	var centre Vec3
	here := Vec3{X: 5}

	clockwise := Tangent(centre, here, true)
	other := Tangent(centre, here, false)

	if clockwise.X != -other.X || clockwise.Z != -other.Z {
		t.Fatalf("clockwise %v and counter-clockwise %v are not opposites", clockwise, other)
	}
}

// TestTangentAtTheCentreIsZero pins the degenerate case rather than dividing by
// zero. There is no tangent to a circle of radius nothing.
func TestTangentAtTheCentreIsZero(t *testing.T) {
	centre := Vec3{X: 3, Y: 64, Z: 3}

	if got := Tangent(centre, centre, true); !got.IsZero() {
		t.Fatalf("Tangent at the centre = %v, want the zero vector", got)
	}
}

func TestAwayIsOppositeTheThreatAtTheFullDistance(t *testing.T) {
	here := Vec3{X: 0, Y: 64, Z: 0}
	threat := Vec3{X: -3, Y: 64, Z: -4}

	got := Away(here, threat, 10)
	if math.Abs(got.X-6) > 1e-9 || got.Y != 64 || math.Abs(got.Z-8) > 1e-9 {
		t.Fatalf("Away = %v, want {6 64 8}", got)
	}
}

// TestAwayFromSomethingStandingOnYouPicksADirection pins the case a mob that
// has walked into a body creates.
//
// Any direction beats standing still while something hits it.
func TestAwayFromSomethingStandingOnYouPicksADirection(t *testing.T) {
	here := Vec3{X: 1, Y: 64, Z: 1}

	got := Away(here, here, 5)
	if math.Hypot(got.X-here.X, got.Z-here.Z) < 4.999 {
		t.Fatalf("Away from itself = %v, want a point five blocks off", got)
	}
	if got.Y != here.Y {
		t.Fatalf("Away changed the height to %v", got.Y)
	}
}

// TestAimingKeepsTheCallersHeight pins that nothing here chooses a Y.
//
// These functions have no physics. A result that moved a body vertically would
// be claiming it can fall or fly.
func TestAimingKeepsTheCallersHeight(t *testing.T) {
	at := Vec3{X: 2, Y: 71.5, Z: -4}

	if got := Behind(at, Vec3{X: 1, Y: 9, Z: 1}, 3); got.Y != at.Y {
		t.Errorf("Behind changed the height to %v", got.Y)
	}
	if got := Away(at, Vec3{X: 9, Y: 1, Z: 9}, 3); got.Y != at.Y {
		t.Errorf("Away changed the height to %v", got.Y)
	}
	if got := Tangent(Vec3{}, at, true); got.Y != 0 {
		t.Errorf("Tangent returned a vertical component %v", got.Y)
	}
}
