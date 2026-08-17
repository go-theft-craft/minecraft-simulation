package world

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

func TestBlockStateIsUnknownUntilSet(t *testing.T) {
	blocks := NewBlocks()

	ref, lookup := blocks.BlockState(geom.BlockPos{X: 1})
	if lookup != LookupUnknown {
		t.Fatalf("lookup = %v, want LookupUnknown", lookup)
	}
	if ref != 0 {
		t.Errorf("ref = %d, want the zero handle for an unknown position", ref)
	}
}

func TestSetBlockRecordsBothTheHandleAndTheShape(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{X: 2, Y: 3, Z: 4}
	blocks.SetBlock(pos, 77, geom.FullCube())

	ref, lookup := blocks.BlockState(pos)
	if lookup != LookupShape || ref != 77 {
		t.Fatalf("BlockState = (%d, %v), want (77, shape)", ref, lookup)
	}

	shape, lookup := blocks.CollisionShape(pos)
	if lookup != LookupShape || shape.Len() != 1 {
		t.Fatalf("CollisionShape = (%d boxes, %v), want (1, shape)", shape.Len(), lookup)
	}
}

func TestSetBlockWithAnEmptyShapeIsAirThatStillCarriesItsHandle(t *testing.T) {
	// Air is a block state too. A profile that wants to ask about the block
	// underfoot must get an answer for air, and "air" is not the same answer as
	// "nobody told me".
	blocks := NewBlocks()
	pos := geom.BlockPos{Y: 9}
	blocks.SetBlock(pos, 5, geom.EmptyShape())

	ref, lookup := blocks.BlockState(pos)
	if lookup != LookupAir {
		t.Fatalf("lookup = %v, want LookupAir", lookup)
	}
	if ref != 5 {
		t.Errorf("ref = %d, want the handle to survive being air", ref)
	}
}

func TestSetKeepsItsMeaningAndRecordsTheZeroHandle(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{Z: 1}
	blocks.Set(pos, geom.FullCube())

	if ref, lookup := blocks.BlockState(pos); ref != 0 || lookup != LookupShape {
		t.Fatalf("BlockState = (%d, %v), want (0, shape)", ref, lookup)
	}
}

func TestForgetClearsTheHandleToo(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{X: -1}
	blocks.SetBlock(pos, 12, geom.FullCube())
	blocks.Forget(pos)

	if _, lookup := blocks.BlockState(pos); lookup != LookupUnknown {
		t.Fatalf("lookup = %v, want LookupUnknown", lookup)
	}
}

func TestCloneDoesNotFollowItsOrigin(t *testing.T) {
	blocks := NewBlocks()
	pos := geom.BlockPos{X: 3}
	blocks.SetBlock(pos, 1, geom.FullCube())

	clone := blocks.Clone()
	blocks.SetBlock(pos, 2, geom.EmptyShape())

	if ref, lookup := clone.BlockState(pos); ref != 1 || lookup != LookupShape {
		t.Fatalf("the clone followed its origin: (%d, %v)", ref, lookup)
	}
}

// Blocks satisfies the whole view, not just the collision half.
var _ View = (*Blocks)(nil)
