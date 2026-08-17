package movement

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestASmallComponentIsDiscardedAndALargeOneIsNot(t *testing.T) {
	got := ClampSmallMotion(geom.Vec3{X: 0.004, Y: -0.02, Z: -0.0049}, 0.005)

	if got.X != 0 {
		t.Errorf("X = %v, want the small component discarded", got.X)
	}
	if got.Z != 0 {
		t.Errorf("Z = %v, want the small negative component discarded", got.Z)
	}
	if got.Y != -0.02 {
		t.Errorf("Y = %v, want a component above the threshold left alone", got.Y)
	}
}

func TestAMotionAtTheThresholdSurvives(t *testing.T) {
	// The game compares with a strict less-than on the magnitude, so a component
	// of exactly the threshold is kept. This is the boundary a rule written from
	// prose is most likely to put on the wrong side.
	got := ClampSmallMotion(geom.Vec3{X: 0.005, Z: -0.005}, 0.005)

	if got.X != 0.005 || got.Z != -0.005 {
		t.Fatalf("a motion of exactly the threshold was discarded: %+v", got)
	}
}

func TestClampingIsPerComponent(t *testing.T) {
	// A body walking at a shallow angle has one large component and one small
	// one, and the game discards only the small one. Clamping the whole vector
	// on its length would stop the body instead.
	got := ClampSmallMotion(geom.Vec3{X: 0.001, Z: 0.3}, 0.005)

	if got.X != 0 || got.Z != 0.3 {
		t.Fatalf("got %+v, want the small component gone and the large one kept", got)
	}
}
