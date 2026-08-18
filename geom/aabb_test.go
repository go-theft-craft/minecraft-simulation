package geom

import (
	"math"
	"math/rand/v2"
	"testing"
)

func unit() AABB {
	return AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 1, MaxZ: 1}
}

func TestBlockAABBCoversExactlyOneCell(t *testing.T) {
	got := BlockAABB(BlockPos{X: -1, Y: 4, Z: 2})
	want := AABB{MinX: -1, MinY: 4, MinZ: 2, MaxX: 0, MaxY: 5, MaxZ: 3}
	if got != want {
		t.Fatalf("BlockAABB = %+v, want %+v", got, want)
	}
}

func TestOffsetMovesBothFaces(t *testing.T) {
	got := unit().Offset(Vec3{X: 2, Y: -1, Z: 0.5})
	want := AABB{MinX: 2, MinY: -1, MinZ: 0.5, MaxX: 3, MaxY: 0, MaxZ: 1.5}
	if got != want {
		t.Fatalf("Offset = %+v, want %+v", got, want)
	}
}

func TestStretchGrowsOnlyTowardMotion(t *testing.T) {
	positive := unit().Stretch(Vec3{X: 2, Y: 0, Z: 0})
	if positive.MinX != 0 || positive.MaxX != 3 {
		t.Fatalf("positive stretch = %+v, want the far face to move", positive)
	}

	negative := unit().Stretch(Vec3{X: -2, Y: 0, Z: 0})
	if negative.MinX != -2 || negative.MaxX != 1 {
		t.Fatalf("negative stretch = %+v, want the near face to move", negative)
	}

	if got := unit().Stretch(Vec3{}); got != unit() {
		t.Fatalf("zero stretch changed the box: %+v", got)
	}
}

func TestStretchCoversEveryAxisAtOnce(t *testing.T) {
	got := unit().Stretch(Vec3{X: 1, Y: -1, Z: 2})
	want := AABB{MinX: 0, MinY: -1, MinZ: 0, MaxX: 2, MaxY: 1, MaxZ: 3}
	if got != want {
		t.Fatalf("Stretch = %+v, want %+v", got, want)
	}
}

func TestIntersectsIsStrictAndSymmetric(t *testing.T) {
	for _, test := range []struct {
		name  string
		other AABB
		want  bool
	}{
		{"overlapping", AABB{MinX: 0.5, MinY: 0.5, MinZ: 0.5, MaxX: 2, MaxY: 2, MaxZ: 2}, true},
		{"touching faces only", AABB{MinX: 1, MinY: 0, MinZ: 0, MaxX: 2, MaxY: 1, MaxZ: 1}, false},
		{"separated", AABB{MinX: 2, MinY: 0, MinZ: 0, MaxX: 3, MaxY: 1, MaxZ: 1}, false},
		{"contained", AABB{MinX: 0.2, MinY: 0.2, MinZ: 0.2, MaxX: 0.8, MaxY: 0.8, MaxZ: 0.8}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := unit().Intersects(test.other); got != test.want {
				t.Errorf("Intersects = %v, want %v", got, test.want)
			}
			if got := test.other.Intersects(unit()); got != test.want {
				t.Errorf("reversed Intersects = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNearestClampsEachAxisIndependently(t *testing.T) {
	box := AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 2, MaxZ: 1}

	for _, test := range []struct {
		name  string
		point Vec3
		want  Vec3
	}{
		{name: "outside on every axis", point: Vec3{X: -5, Y: -5, Z: -5}, want: Vec3{}},
		{name: "outside on one axis", point: Vec3{X: 0.5, Y: 5, Z: 0.5}, want: Vec3{X: 0.5, Y: 2, Z: 0.5}},
		{name: "inside returns itself", point: Vec3{X: 0.5, Y: 1, Z: 0.5}, want: Vec3{X: 0.5, Y: 1, Z: 0.5}},
		{name: "on the face", point: Vec3{X: 1, Y: 1, Z: 0.5}, want: Vec3{X: 1, Y: 1, Z: 0.5}},
	} {
		if got := box.Nearest(test.point); got != test.want {
			t.Errorf("Nearest(%s) = %v, want %v", test.name, got, test.want)
		}
	}
}

// TestReachesComparesToTheNearestPointNotTheCentre is the distinction the whole
// function exists for.
//
// The game measures reach to a target's box. A client measuring to the centre
// refuses hits the server accepts, and the taller the target the wider the
// disagreement — which is why the box here is two blocks tall.
func TestReachesComparesToTheNearestPointNotTheCentre(t *testing.T) {
	box := AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 2, MaxZ: 1}
	eye := Vec3{X: 4.5, Y: 1, Z: 0.5}

	// The near face is at x=1, so the nearest point is 3.5 away. The centre is
	// at x=0.5, which is 4.0 away — far enough that a centre-measuring reach
	// would disagree with both assertions below.
	if !box.Reaches(eye, 3.6) {
		t.Error("an eye 3.5 from the near face is out of reach at 3.6")
	}
	if box.Reaches(eye, 3.4) {
		t.Error("an eye 3.5 from the near face is in reach at 3.4")
	}
	if box.Reaches(eye, 3.9) != true {
		t.Error("a reach of 3.9 does not cover a face 3.5 away")
	}
}

// TestReachesIsExactlyTheNearestDistance pins that the test is the comparison
// and nothing else, at the boundary in both directions.
func TestReachesIsExactlyTheNearestDistance(t *testing.T) {
	box := AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 1, MaxZ: 1}
	eye := Vec3{X: 4, Y: 0.5, Z: 0.5}

	// Exactly three blocks from the face at x=1.
	if !box.Reaches(eye, 3) {
		t.Error("a target exactly at the limit is out of reach")
	}
	if box.Reaches(eye, math.Nextafter(3, 0)) {
		t.Error("a target just past the limit is in reach")
	}
}

// TestAnEyeInsideTheBoxIsAlwaysInReach pins the degenerate case a caller hits
// when it is standing inside what it is aiming at.
func TestAnEyeInsideTheBoxIsAlwaysInReach(t *testing.T) {
	box := AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 2, MaxZ: 1}

	if !box.Reaches(Vec3{X: 0.5, Y: 1, Z: 0.5}, 0) {
		t.Error("an eye inside the box is out of reach at zero")
	}
}

// TestNearestAlwaysLandsInsideTheBox is the property the two functions above
// rest on.
func TestNearestAlwaysLandsInsideTheBox(t *testing.T) {
	box := AABB{MinX: -2, MinY: 0, MinZ: -3, MaxX: 5, MaxY: 4, MaxZ: 1}
	random := rand.New(rand.NewPCG(1, 2))

	for range 10_000 {
		point := Vec3{
			X: random.Float64()*40 - 20,
			Y: random.Float64()*40 - 20,
			Z: random.Float64()*40 - 20,
		}
		nearest := box.Nearest(point)

		if nearest.X < box.MinX || nearest.X > box.MaxX ||
			nearest.Y < box.MinY || nearest.Y > box.MaxY ||
			nearest.Z < box.MinZ || nearest.Z > box.MaxZ {
			t.Fatalf("Nearest(%v) = %v, outside the box", point, nearest)
		}
	}
}

// TestNearestAgreesWithABruteForceClamp checks the implementation against the
// definition, per axis, so a transposed field shows up.
func TestNearestAgreesWithABruteForceClamp(t *testing.T) {
	box := AABB{MinX: -2, MinY: 0, MinZ: -3, MaxX: 5, MaxY: 4, MaxZ: 1}
	random := rand.New(rand.NewPCG(3, 4))

	for range 10_000 {
		point := Vec3{
			X: random.Float64()*40 - 20,
			Y: random.Float64()*40 - 20,
			Z: random.Float64()*40 - 20,
		}

		want := Vec3{
			X: math.Min(math.Max(point.X, box.MinX), box.MaxX),
			Y: math.Min(math.Max(point.Y, box.MinY), box.MaxY),
			Z: math.Min(math.Max(point.Z, box.MinZ), box.MaxZ),
		}
		if got := box.Nearest(point); got != want {
			t.Fatalf("Nearest(%v) = %v, want %v", point, got, want)
		}
	}
}
