package geom

import (
	"reflect"
	"testing"
)

func TestFullCubeIsTheUnitBox(t *testing.T) {
	got := FullCube().BoxesAt(BlockPos{}, nil)
	want := []AABB{{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 1, MaxZ: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FullCube = %+v, want %+v", got, want)
	}
}

func TestEmptyShapeYieldsNoBoxes(t *testing.T) {
	if !EmptyShape().IsEmpty() {
		t.Error("EmptyShape is not empty")
	}
	if got := EmptyShape().BoxesAt(BlockPos{X: 3}, nil); len(got) != 0 {
		t.Fatalf("EmptyShape produced %d boxes", len(got))
	}
}

func TestBoxesAtTranslatesToTheCell(t *testing.T) {
	slab := NewShape(AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 0.5, MaxZ: 1})

	got := slab.BoxesAt(BlockPos{X: 2, Y: -1, Z: 5}, nil)
	want := []AABB{{MinX: 2, MinY: -1, MinZ: 5, MaxX: 3, MaxY: -0.5, MaxZ: 6}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BoxesAt = %+v, want %+v", got, want)
	}
}

func TestBoxesAtAppendsAndPreservesOrder(t *testing.T) {
	fence := NewShape(
		AABB{MinX: 0.375, MinY: 0, MinZ: 0, MaxX: 0.625, MaxY: 1.5, MaxZ: 1},
		AABB{MinX: 0, MinY: 0, MinZ: 0.375, MaxX: 1, MaxY: 1.5, MaxZ: 0.625},
	)

	dst := []AABB{{MinX: -99}}
	got := fence.BoxesAt(BlockPos{}, dst)
	if len(got) != 3 {
		t.Fatalf("BoxesAt returned %d boxes, want the existing one plus two", len(got))
	}
	if got[0].MinX != -99 {
		t.Error("BoxesAt overwrote the destination slice")
	}
	if got[1].MinX != 0.375 || got[2].MinZ != 0.375 {
		t.Fatalf("BoxesAt reordered the shape: %+v", got[1:])
	}
}

func TestNewShapeCopiesItsInput(t *testing.T) {
	boxes := []AABB{{MaxX: 1, MaxY: 1, MaxZ: 1}}
	shape := NewShape(boxes...)
	boxes[0].MaxY = 99

	got := shape.BoxesAt(BlockPos{}, nil)
	if got[0].MaxY != 1 {
		t.Fatalf("mutating the caller's slice changed the shape: %+v", got[0])
	}
}

func TestShapeLen(t *testing.T) {
	if got := FullCube().Len(); got != 1 {
		t.Errorf("FullCube.Len = %d, want 1", got)
	}
	if got := EmptyShape().Len(); got != 0 {
		t.Errorf("EmptyShape.Len = %d, want 0", got)
	}
}
