package geom

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestFloorRoundsTowardNegativeInfinity(t *testing.T) {
	for _, test := range []struct {
		value float64
		want  int32
	}{
		{0, 0},
		{0.5, 0},
		{1, 1},
		{-0.5, -1},
		{-1, -1},
		{-1.5, -2},
		{63.999999, 63},
		{-63.000001, -64},
	} {
		if got := Floor(test.value); got != test.want {
			t.Errorf("Floor(%v) = %d, want %d", test.value, got, test.want)
		}
	}
}

func TestBlockPosOfUsesFloorOnEveryAxis(t *testing.T) {
	got := BlockPosOf(Vec3{X: -0.5, Y: 4.9, Z: -8.1})
	want := BlockPos{X: -1, Y: 4, Z: -9}
	if got != want {
		t.Fatalf("BlockPosOf = %+v, want %+v", got, want)
	}
}

func TestVec3Arithmetic(t *testing.T) {
	a := Vec3{X: 1, Y: 2, Z: 3}
	b := Vec3{X: 0.5, Y: -1, Z: 2}

	if got := a.Add(b); got != (Vec3{X: 1.5, Y: 1, Z: 5}) {
		t.Errorf("Add = %+v", got)
	}
	if got := a.Sub(b); got != (Vec3{X: 0.5, Y: 3, Z: 1}) {
		t.Errorf("Sub = %+v", got)
	}
	if got := a.Scale(2); got != (Vec3{X: 2, Y: 4, Z: 6}) {
		t.Errorf("Scale = %+v", got)
	}
}

func TestHorizontalLengthSquaredIgnoresY(t *testing.T) {
	v := Vec3{X: 3, Y: 100, Z: 4}
	if got := v.HorizontalLengthSquared(); got != 25 {
		t.Fatalf("HorizontalLengthSquared = %v, want 25", got)
	}
}

func TestIsZero(t *testing.T) {
	if !(Vec3{}).IsZero() {
		t.Error("the zero vector is not reported as zero")
	}
	if (Vec3{X: math.SmallestNonzeroFloat64}).IsZero() {
		t.Error("a tiny non-zero vector is reported as zero")
	}
}

// TestYawIsMeasuredFromSouthTowardWest pins the convention that is neither the
// mathematical one nor a compass bearing.
//
// Getting it wrong does not fail: a body walks confidently in the wrong
// direction, and on a symmetric test world it can even arrive.
func TestYawIsMeasuredFromSouthTowardWest(t *testing.T) {
	var origin Vec3

	for _, test := range []struct {
		name string
		to   Vec3
		want float32
	}{
		{name: "south is zero", to: Vec3{Z: 1}, want: 0},
		{name: "west is ninety", to: Vec3{X: -1}, want: 90},
		{name: "north is one eighty", to: Vec3{Z: -1}, want: 180},
		{name: "east is minus ninety", to: Vec3{X: 1}, want: -90},
	} {
		if got := origin.Yaw(test.to); angleApart(float64(got), float64(test.want)) > 1e-4 {
			t.Errorf("Yaw(%s) = %v, want %v", test.name, got, test.want)
		}
	}
}

// angleApart returns how far two headings are from each other in degrees,
// taking the short way round.
//
// Due north comes back as -180 rather than +180, because the X term is negative
// zero and Atan2 respects its sign. The two name the same heading and the wire
// carries either, so comparing the numbers would fail on a value that is
// correct. Comparing the angle is what the assertion actually means.
func angleApart(a, b float64) float64 {
	difference := math.Mod(math.Abs(a-b), 360)
	if difference > 180 {
		difference = 360 - difference
	}

	return difference
}

// TestPitchIsPositiveDownward pins the sign, which is the game's and not the
// intuitive one.
//
// A client that flips it aims at the ceiling every time it means to mine the
// floor, and the mistake is invisible on flat ground — which is where most of a
// bot's testing happens.
func TestPitchIsPositiveDownward(t *testing.T) {
	eye := Vec3{X: 0, Y: 10, Z: 0}

	for _, test := range []struct {
		name string
		to   Vec3
		want float32
	}{
		{name: "straight down is ninety", to: Vec3{X: 0, Y: 0, Z: 0}, want: 90},
		{name: "straight up is minus ninety", to: Vec3{X: 0, Y: 20, Z: 0}, want: -90},
		{name: "level is zero", to: Vec3{X: 0, Y: 10, Z: 5}, want: 0},
		{name: "a forty-five degree descent", to: Vec3{X: 0, Y: 5, Z: 5}, want: 45},
		{name: "a forty-five degree climb", to: Vec3{X: 0, Y: 15, Z: 5}, want: -45},
	} {
		if got := eye.Pitch(test.to); math.Abs(float64(got-test.want)) > 1e-4 {
			t.Errorf("Pitch(%s) = %v, want %v", test.name, got, test.want)
		}
	}
}

// TestLookAgreesWithYawAndPitchSeparately pins that the pair is the two
// functions and not a third implementation of them.
func TestLookAgreesWithYawAndPitchSeparately(t *testing.T) {
	random := rand.New(rand.NewPCG(5, 6))

	for range 1000 {
		from := Vec3{
			X: random.Float64()*200 - 100,
			Y: random.Float64()*200 - 100,
			Z: random.Float64()*200 - 100,
		}
		to := Vec3{
			X: random.Float64()*200 - 100,
			Y: random.Float64()*200 - 100,
			Z: random.Float64()*200 - 100,
		}

		yaw, pitch := from.Look(to)
		if yaw != from.Yaw(to) || pitch != from.Pitch(to) {
			t.Fatalf("Look(%v, %v) = %v, %v; separately %v, %v",
				from, to, yaw, pitch, from.Yaw(to), from.Pitch(to))
		}
	}
}

// TestPitchAtTheSameColumnIsVertical pins the degenerate case that would be
// asymptotic if the angle were measured against a straight-line distance.
func TestPitchAtTheSameColumnIsVertical(t *testing.T) {
	eye := Vec3{X: 3, Y: 10, Z: 3}

	if got := eye.Pitch(Vec3{X: 3, Y: 4, Z: 3}); math.Abs(float64(got-90)) > 1e-6 {
		t.Errorf("looking straight down = %v, want exactly 90", got)
	}
	if got := eye.Pitch(Vec3{X: 3, Y: 16, Z: 3}); math.Abs(float64(got+90)) > 1e-6 {
		t.Errorf("looking straight up = %v, want exactly -90", got)
	}
}

// TestTowardStopsExactlyOnTheTarget pins the arrival condition.
//
// Overshooting turns arrival into a point a body oscillates around rather than
// a state it reaches.
func TestTowardStopsExactlyOnTheTarget(t *testing.T) {
	from := Vec3{X: 0, Y: 64, Z: 0}
	to := Vec3{X: 3, Y: 70, Z: 4}

	// Five blocks away horizontally, stepping ten: it arrives rather than
	// overshooting, and it keeps its own Y because it chooses no height.
	if got, want := from.Toward(to, 10), (Vec3{X: 3, Y: 64, Z: 4}); got != want {
		t.Fatalf("Toward = %v, want %v", got, want)
	}
}

// TestTowardClampsALongStep pins the other half.
func TestTowardClampsALongStep(t *testing.T) {
	from := Vec3{X: 0, Y: 64, Z: 0}
	to := Vec3{X: 10, Y: 64, Z: 0}

	got := from.Toward(to, 2.5)
	if math.Abs(got.X-2.5) > 1e-9 || got.Y != 64 || got.Z != 0 {
		t.Fatalf("Toward = %v, want a 2.5-block step along X at the original height", got)
	}
}

// TestTowardAtZeroDistanceDoesNotDivideByZero pins the case a body reaches
// every time it arrives.
func TestTowardAtZeroDistanceDoesNotDivideByZero(t *testing.T) {
	at := Vec3{X: 1, Y: 2, Z: 3}

	if got := at.Toward(at, 0.5); got != at {
		t.Fatalf("Toward onto itself = %v, want %v", got, at)
	}
}

// TestHorizontalDistanceIgnoresHeight pins what the name promises.
func TestHorizontalDistanceIgnoresHeight(t *testing.T) {
	low := Vec3{X: 0, Y: 0, Z: 0}
	high := Vec3{X: 3, Y: 1000, Z: 4}

	if got := low.HorizontalDistance(high); math.Abs(got-5) > 1e-9 {
		t.Fatalf("HorizontalDistance = %v, want 5", got)
	}
}
