package geom

import (
	"math"
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
