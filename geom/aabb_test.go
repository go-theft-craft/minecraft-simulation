package geom

import "testing"

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
