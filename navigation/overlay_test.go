package navigation

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

const refStone world.BlockRef = 1

// TestAPlacementIsVisibleThroughTheOverlayAndNotTheBase is the whole contract:
// the search sees its own placements, and the caller's snapshot never does.
func TestAPlacementIsVisibleThroughTheOverlayAndNotTheBase(t *testing.T) {
	at := geom.BlockPos{X: 0, Y: 64, Z: 0}

	base := world.NewBlocks()
	base.Set(at, geom.EmptyShape())

	overlay := NewOverlay(base)
	overlay.Place(at, refStone, geom.FullCube())

	shape, lookup := overlay.CollisionShape(at)
	if lookup == world.LookupUnknown {
		t.Fatal("the overlay does not describe a cell it placed a block in")
	}
	if shape.Len() == 0 {
		t.Fatal("the overlay does not see the placement")
	}
	if ref, _ := overlay.BlockState(at); ref != refStone {
		t.Fatalf("the overlay reports block %v, want the placed %v", ref, refStone)
	}

	// The search plans against a world it may not touch. An overlay that wrote
	// through would corrupt the caller's snapshot on a route then discarded.
	if shape, _ := base.CollisionShape(at); shape.Len() != 0 {
		t.Fatal("the placement reached the base view")
	}
}

// TestTheOverlayDoesNotInventAnAnswerForAnUnknownCell pins the rule the whole
// package turns on.
func TestTheOverlayDoesNotInventAnAnswerForAnUnknownCell(t *testing.T) {
	overlay := NewOverlay(world.NewBlocks())

	if _, lookup := overlay.CollisionShape(geom.BlockPos{X: 99, Y: 64, Z: 99}); lookup != world.LookupUnknown {
		t.Fatal("the overlay described a cell the base does not")
	}
	if _, lookup := overlay.BlockState(geom.BlockPos{X: 99, Y: 64, Z: 99}); lookup != world.LookupUnknown {
		t.Fatal("the overlay named a block in a cell the base does not describe")
	}
}

// TestResetForgetsEveryPlacement pins that one overlay serves the whole
// validation loop, so a banned round leaves nothing behind for the next.
func TestResetForgetsEveryPlacement(t *testing.T) {
	at := geom.BlockPos{X: 0, Y: 64, Z: 0}

	base := world.NewBlocks()
	base.Set(at, geom.EmptyShape())

	overlay := NewOverlay(base)
	overlay.Place(at, refStone, geom.FullCube())
	overlay.Reset()

	if shape, _ := overlay.CollisionShape(at); shape.Len() != 0 {
		t.Fatal("Reset left a placement behind")
	}
	if overlay.Len() != 0 {
		t.Fatalf("Reset left %d placements counted", overlay.Len())
	}
}

// TestRemoveForgetsOnePlacement pins that the overlay can be walked back one
// step at a time.
func TestRemoveForgetsOnePlacement(t *testing.T) {
	first := geom.BlockPos{X: 0, Y: 64, Z: 0}
	second := geom.BlockPos{X: 1, Y: 64, Z: 0}

	base := world.NewBlocks()
	base.Set(first, geom.EmptyShape())
	base.Set(second, geom.EmptyShape())

	overlay := NewOverlay(base)
	overlay.Place(first, refStone, geom.FullCube())
	overlay.Place(second, refStone, geom.FullCube())
	overlay.Remove(first)

	if shape, _ := overlay.CollisionShape(first); shape.Len() != 0 {
		t.Fatal("Remove left the placement behind")
	}
	if shape, _ := overlay.CollisionShape(second); shape.Len() == 0 {
		t.Fatal("Remove dropped a placement it was not asked about")
	}
}
