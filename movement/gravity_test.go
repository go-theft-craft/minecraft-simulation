package movement

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestGravityTouchesOnlyTheVertical(t *testing.T) {
	before := geom.Vec3{X: 1, Y: 0, Z: -2}
	got := ApplyGravity(before, 0.08)

	if got.X != before.X || got.Z != before.Z {
		t.Fatalf("gravity moved a horizontal component: %+v", got)
	}
	if got.Y != -0.08 {
		t.Fatalf("Y = %v, want -0.08", got.Y)
	}
}

func TestGravityIsADoubleSubtraction(t *testing.T) {
	// The game's gravity is a double literal, so subtracting it is plain
	// double arithmetic. A rule that narrowed it to float32 first would produce
	// 0.07999999821186066 instead, and a test that expected the narrowed value
	// would enshrine the mistake.
	got := ApplyGravity(geom.Vec3{Y: 1}, 0.08)
	if want := 1 - 0.08; got.Y != want {
		t.Fatalf("Y = %v, want the double difference %v", got.Y, want)
	}
	if got.Y == 1-float64(float32(0.08)) {
		t.Fatal("gravity was narrowed to single width before being applied")
	}
}

func TestVerticalDragIsADoubleWidthProduct(t *testing.T) {
	const drag float32 = 0.9800000190734863

	// The drag is a float and the motion is a double, so the game widens the
	// drag and multiplies at double width. Narrowing the motion first is a
	// different number, and this is where it shows: a tenth is not exact at
	// single width, so the two answers disagree. A value like a half would be
	// exact in both and the assertion below could not fail.
	motion := geom.Vec3{X: 1, Y: -0.1, Z: 2}
	got := ApplyVerticalDrag(motion, drag)

	if got.X != motion.X || got.Z != motion.Z {
		t.Fatalf("the vertical drag moved a horizontal component: %+v", got)
	}
	if want := motion.Y * float64(drag); got.Y != want {
		t.Fatalf("Y = %v, want the double-width product %v", got.Y, want)
	}
	if got.Y == float64(float32(motion.Y)*drag) {
		t.Fatal("the vertical motion was narrowed to single width before the drag")
	}
}

func TestHorizontalDragTouchesBothHorizontalsAndNotTheVertical(t *testing.T) {
	const friction float32 = 0.5460000038146973 // 0.6 × 0.91 at single width.

	motion := geom.Vec3{X: 0.1, Y: -0.4, Z: -0.3}
	got := ApplyHorizontalDrag(motion, friction)

	if got.Y != motion.Y {
		t.Fatalf("the horizontal drag moved Y to %v", got.Y)
	}
	if want := motion.X * float64(friction); got.X != want {
		t.Fatalf("X = %v, want %v", got.X, want)
	}
	if want := motion.Z * float64(friction); got.Z != want {
		t.Fatalf("Z = %v, want %v", got.Z, want)
	}

	// The assertions above pin double width, but only for values where the two
	// widths actually disagree. Most do not, so the separating value is searched
	// for rather than guessed at, and a run that cannot find one fails: a check
	// that silently became vacuous is worse than no check.
	separated := false
	for _, candidate := range []float64{0.1, 0.2, 0.3, 0.7, 1.0 / 3, 0.123456789} {
		narrow := float64(float32(candidate) * friction)
		wide := candidate * float64(friction)
		if narrow == wide {
			continue
		}
		separated = true

		if drifted := ApplyHorizontalDrag(geom.Vec3{X: candidate}, friction); drifted.X != wide {
			t.Fatalf("X = %v for %v, want the double-width product %v (single width gives %v)",
				drifted.X, candidate, wide, narrow)
		}
	}
	if !separated {
		t.Fatal("no candidate separated the two widths; this test proves nothing as written")
	}
}

func TestAFallReachesATerminalSpeed(t *testing.T) {
	// Gravity and the drag together bound a fall. This is not a precision check —
	// the oracle owns that — but it fails loudly if the two are applied in a way
	// that lets a fall accelerate without bound.
	motion := geom.Vec3{}
	for range 200 {
		motion = ApplyVerticalDrag(ApplyGravity(motion, 0.08), 0.9800000190734863)
	}

	if motion.Y > -3.5 || motion.Y < -4.5 {
		t.Fatalf("after 200 ticks of falling the motion is %v, want about -3.92", motion.Y)
	}
}
