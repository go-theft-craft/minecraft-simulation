package movement

import (
	"math"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestInputBelowTheThresholdMovesNothing(t *testing.T) {
	table := gameTable(t)
	motion := geom.Vec3{X: 1, Y: 2, Z: 3}

	for name, input := range map[string][2]float32{
		"no input at all": {0, 0},
		// 0.005² + 0.005² is 5e-5, below the threshold: a controller's noise
		// must not creep the body along.
		"tiny but non-zero": {0.005, 0.005},
	} {
		t.Run(name, func(t *testing.T) {
			got := ApplyHeading(table, motion, input[0], input[1], 0.1, 0)
			if got != motion {
				t.Fatalf("ApplyHeading = %+v, want the motion untouched %+v", got, motion)
			}
		})
	}
}

func TestTheThresholdComparisonIsStrict(t *testing.T) {
	// The game compares with <, so an input whose squared magnitude is exactly
	// the threshold does move the body. 0.01² is exactly 1e-4 at single width.
	table := gameTable(t)
	motion := geom.Vec3{}

	if got := ApplyHeading(table, motion, 0, 0.01, 0.1, 0); got == motion {
		t.Fatal("input exactly at the threshold was discarded; the comparison is not strict")
	}
}

func TestForwardAtZeroYawPinsTheSignConvention(t *testing.T) {
	table := gameTable(t)

	got := ApplyHeading(table, geom.Vec3{}, 0, 1, 0.1, 0)
	if got.X != 0 {
		t.Errorf("forward at zero yaw moved X by %v, want 0", got.X)
	}
	if !(got.Z > 0) {
		t.Errorf("forward at zero yaw moved Z by %v, want positive", got.Z)
	}
}

func TestStrafeAtZeroYawMovesTheOtherAxis(t *testing.T) {
	table := gameTable(t)

	got := ApplyHeading(table, geom.Vec3{}, 1, 0, 0.1, 0)
	if got.Z != 0 {
		t.Errorf("strafe at zero yaw moved Z by %v, want 0", got.Z)
	}
	if !(got.X > 0) {
		t.Errorf("strafe at zero yaw moved X by %v, want positive", got.X)
	}
}

func TestDiagonalInputIsNormalized(t *testing.T) {
	// Walking diagonally is not faster than walking straight, which is what the
	// magnitude division buys.
	table := gameTable(t)

	straight := ApplyHeading(table, geom.Vec3{}, 0, 1, 0.1, 0)
	diagonal := ApplyHeading(table, geom.Vec3{}, 1, 1, 0.1, 0)

	// Agreement is to single-width precision, not to the bit: the diagonal's
	// magnitude divides by a rounded square root, so the two answers differ in
	// the last places a float32 has.
	const tolerance = 1e-7

	straightSpeed := math.Hypot(straight.X, straight.Z)
	diagonalSpeed := math.Hypot(diagonal.X, diagonal.Z)
	if math.Abs(straightSpeed-diagonalSpeed) > tolerance {
		t.Fatalf("diagonal travels at %v and straight at %v; they must agree",
			diagonalSpeed, straightSpeed)
	}
}

func TestInputBelowUnitMagnitudeIsNotNormalizedUp(t *testing.T) {
	// The clamp is max(1, magnitude), so half input travels half as far. Without
	// it, a light touch would accelerate like a full stride.
	table := gameTable(t)

	full := ApplyHeading(table, geom.Vec3{}, 0, 1, 0.1, 0)
	half := ApplyHeading(table, geom.Vec3{}, 0, 0.5, 0.1, 0)

	if !(half.Z < full.Z) {
		t.Fatalf("half input travelled %v and full input %v; half must be less", half.Z, full.Z)
	}
	if math.Abs(half.Z-full.Z/2) > 1e-7 {
		t.Fatalf("half input travelled %v, want half of %v", half.Z, full.Z)
	}
}

func TestTheWideningHappensOnce(t *testing.T) {
	table := gameTable(t)

	const (
		strafe  float32 = 0.7
		forward float32 = 0.7
		speed   float32 = 0.09807128
		yaw     float32 = 37
	)

	// The expectation is built the way the rule must compute it: every bracket at
	// single width, widened once at the end.
	magnitude := float32(math.Sqrt(float64(strafe*strafe + forward*forward)))
	if magnitude < 1 {
		magnitude = 1
	}
	scale := speed / magnitude
	scaledStrafe := strafe * scale
	scaledForward := forward * scale
	angle := radians(yaw)
	sin, cos := table.Sin(angle), table.Cos(angle)

	wantX := float64(scaledStrafe*cos - scaledForward*sin)
	wantZ := float64(scaledForward*cos + scaledStrafe*sin)

	got := ApplyHeading(table, geom.Vec3{}, strafe, forward, speed, yaw)
	if got.X != wantX || got.Z != wantZ {
		t.Fatalf("ApplyHeading = (%v, %v), want (%v, %v)", got.X, got.Z, wantX, wantZ)
	}

	// And the case that fails if the discipline breaks: the same arithmetic
	// carried out at double width throughout gives a different answer, so a
	// widened implementation cannot pass the assertion above.
	wideX := (float64(scaledStrafe) * float64(cos)) - (float64(scaledForward) * float64(sin))
	if wideX == wantX {
		t.Skip("this input does not separate the two widths; choose another")
	}
}

func TestTheHeadingConvertsDegreesInTwoFloatSteps(t *testing.T) {
	table := gameTable(t)

	// The heading multiplies the yaw by a float pi and then divides by 180. The
	// jump impulse, three rules away, multiplies by a single pre-divided
	// constant. They are different expressions in the game and they disagree: at
	// this yaw they read entries 61440 and 61439 of the table.
	//
	// The oracle found it. A walking body drifted four millionths of a block per
	// tick along one axis while matching exactly along the other, which is what
	// reading a neighbouring sine entry looks like.
	const yaw float32 = 337.5

	if radians(yaw) == yaw*degreesToRadians {
		t.Fatal("this yaw does not separate the two conversions; the test proves nothing")
	}

	got := ApplyHeading(table, geom.Vec3{}, 0, 1, 0.1, yaw)

	want := headingWith(table, radians(yaw))
	if got.X != want.X || got.Z != want.Z {
		t.Fatalf("ApplyHeading = %+v, want the two-step conversion's answer %+v", got, want)
	}
	if other := headingWith(table, yaw*degreesToRadians); got.X == other.X && got.Z == other.Z {
		t.Fatal("the pre-divided conversion gives the same answer; the assertion above is vacuous")
	}
}

// headingWith applies the heading of a full forward stride at a given angle,
// with everything but the angle held fixed.
func headingWith(table Table, angle float32) geom.Vec3 {
	const speed float32 = 0.1

	sin, cos := table.Sin(angle), table.Cos(angle)

	return geom.Vec3{
		X: float64(-speed * sin),
		Z: float64(speed * cos),
	}
}

func TestAFullTurnOfYawAgreesWithNone(t *testing.T) {
	table := gameTable(t)

	none := ApplyHeading(table, geom.Vec3{}, 0, 1, 0.1, 0)
	turn := ApplyHeading(table, geom.Vec3{}, 0, 1, 0.1, 360)

	// Within a table step: adding 360 changes how the angle itself rounds, so the
	// index can land one entry away.
	if math.Abs(none.X-turn.X) > 1e-4 || math.Abs(none.Z-turn.Z) > 1e-4 {
		t.Fatalf("yaw 0 gave (%v, %v) and yaw 360 gave (%v, %v)", none.X, none.Z, turn.X, turn.Z)
	}
}

func TestHeadingAddsToTheExistingMotion(t *testing.T) {
	// The rule accumulates: the tick's input is added to whatever the body was
	// already doing, which is what carries momentum between ticks.
	table := gameTable(t)
	before := geom.Vec3{X: 0.5, Y: -0.1, Z: 0.25}

	got := ApplyHeading(table, before, 0, 1, 0.1, 0)
	if got.X != before.X {
		t.Errorf("X = %v, want it untouched at %v", got.X, before.X)
	}
	if got.Y != before.Y {
		t.Errorf("Y = %v, want the heading to leave it alone at %v", got.Y, before.Y)
	}
	if !(got.Z > before.Z) {
		t.Errorf("Z = %v, want it above the original %v", got.Z, before.Z)
	}
}
