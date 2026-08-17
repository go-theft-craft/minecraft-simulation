package world

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestUnsetPositionsAreUnknown(t *testing.T) {
	blocks := NewBlocks()

	shape, lookup := blocks.CollisionShape(geom.BlockPos{X: 1, Y: 2, Z: 3})
	if lookup != LookupUnknown {
		t.Fatalf("lookup = %v, want LookupUnknown", lookup)
	}
	if !shape.IsEmpty() {
		t.Error("an unknown position returned a non-empty shape")
	}
}

func TestSetAirIsKnownAndEmpty(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{X: 1}
	blocks.SetAir(pos)

	shape, lookup := blocks.CollisionShape(pos)
	if lookup != LookupAir {
		t.Fatalf("lookup = %v, want LookupAir", lookup)
	}
	if !shape.IsEmpty() {
		t.Error("air returned a non-empty shape")
	}
}

func TestSetShapeIsReturned(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{Y: 4}
	blocks.Set(pos, geom.FullCube())

	shape, lookup := blocks.CollisionShape(pos)
	if lookup != LookupShape {
		t.Fatalf("lookup = %v, want LookupShape", lookup)
	}
	if shape.Len() != 1 {
		t.Fatalf("shape has %d boxes, want 1", shape.Len())
	}
}

func TestSetWithAnEmptyShapeRecordsAir(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{Z: 7}
	blocks.Set(pos, geom.EmptyShape())

	if _, lookup := blocks.CollisionShape(pos); lookup != LookupAir {
		t.Fatalf("lookup = %v, want LookupAir", lookup)
	}
}

func TestForgetRestoresUnknown(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{X: 2, Y: 2, Z: 2}
	blocks.Set(pos, geom.FullCube())
	blocks.Forget(pos)

	if _, lookup := blocks.CollisionShape(pos); lookup != LookupUnknown {
		t.Fatalf("lookup = %v, want LookupUnknown", lookup)
	}
}

func TestLookupString(t *testing.T) {
	for _, test := range []struct {
		lookup Lookup
		want   string
	}{
		{LookupUnknown, "unknown"},
		{LookupAir, "air"},
		{LookupShape, "shape"},
		{Lookup(99), "Lookup(99)"},
	} {
		if got := test.lookup.String(); got != test.want {
			t.Errorf("Lookup(%d).String() = %q, want %q", test.lookup, got, test.want)
		}
	}
}

// BlockView is satisfied by Blocks. A compile-time assertion is cheaper than
// discovering the mismatch from a collision test.
var _ BlockView = (*Blocks)(nil)
