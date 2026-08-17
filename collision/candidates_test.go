package collision

import (
	"errors"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

func TestGatherReturnsBoxesForSolidCells(t *testing.T) {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1, Y: -1, Z: -1}, geom.BlockPos{X: 1, Y: 1, Z: 1}, geom.EmptyShape())
	blocks.Set(geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.FullCube())

	region := geom.AABB{MinX: -0.5, MinY: -0.5, MinZ: -0.5, MaxX: 1.4, MaxY: 1.4, MaxZ: 1.4}
	got, err := Gather(blocks, region, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(got.Unknown) != 0 {
		t.Fatalf("Gather reported unknown cells: %+v", got.Unknown)
	}
	if len(got.Boxes) != 1 {
		t.Fatalf("Gather returned %d boxes, want 1", len(got.Boxes))
	}
	want := geom.AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 1, MaxZ: 1}
	if got.Boxes[0] != want {
		t.Fatalf("box = %+v, want %+v", got.Boxes[0], want)
	}
}

func TestGatherReportsUnknownCellsAndSkipsTheirBoxes(t *testing.T) {
	blocks := world.NewBlocks()
	// Only one cell is described; everything else in the region is unknown.
	blocks.Set(geom.BlockPos{}, geom.FullCube())

	region := geom.AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1.5, MaxY: 0.5, MaxZ: 0.5}
	got, err := Gather(blocks, region, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(got.Boxes) != 1 {
		t.Fatalf("Gather returned %d boxes, want 1", len(got.Boxes))
	}
	if len(got.Unknown) != 1 || got.Unknown[0] != (geom.BlockPos{X: 1}) {
		t.Fatalf("Unknown = %+v, want exactly the cell at x=1", got.Unknown)
	}
}

func TestGatherVisitsCellsInAFixedOrder(t *testing.T) {
	blocks := world.NewBlocks()

	region := geom.AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1.5, MaxY: 1.5, MaxZ: 0.5}
	first, err := Gather(blocks, region, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	second, err := Gather(blocks, region, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	if len(first.Unknown) != 4 {
		t.Fatalf("Unknown has %d cells, want 4", len(first.Unknown))
	}
	for index := range first.Unknown {
		if first.Unknown[index] != second.Unknown[index] {
			t.Fatalf("Gather is not deterministic at index %d: %+v vs %+v",
				index, first.Unknown[index], second.Unknown[index])
		}
	}
	// X outermost, then Y, then Z.
	want := []geom.BlockPos{{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1}}
	for index, pos := range want {
		if first.Unknown[index] != pos {
			t.Fatalf("Unknown[%d] = %+v, want %+v", index, first.Unknown[index], pos)
		}
	}
}

func TestGatherExcludesAShapeItOnlyTouches(t *testing.T) {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{}, geom.BlockPos{X: 1}, geom.EmptyShape())
	blocks.Set(geom.BlockPos{X: 1}, geom.FullCube())

	// The region's max X lands exactly on the boundary of the solid cell, so the
	// two share a face and overlap in nothing.
	//
	// This test used to require the opposite, on the reasoning that a cell the
	// region reaches is a cell worth clamping against. The game disagrees: it
	// gathers a collider only where the sweep overlaps the block in a volume, and
	// the oracle's TestAFlushFaceIsNotACollider asked a 1.8.9 server to settle
	// it. A body flush against a wall and moving into it by an amount too small
	// to see is not stopped by that wall.
	region := geom.AABB{MinX: 0, MinY: 0, MinZ: 0, MaxX: 1, MaxY: 0.5, MaxZ: 0.5}
	got, err := Gather(blocks, region, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(got.Boxes) != 0 {
		t.Fatalf("Gather returned %d boxes, want the touched cell to be left out", len(got.Boxes))
	}

	// A region that reaches past the face by any amount does gather it.
	reaching := region
	reaching.MaxX = 1.0000001
	got, err = Gather(blocks, reaching, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(got.Boxes) != 1 {
		t.Fatalf("Gather returned %d boxes, want the overlapped cell to be included", len(got.Boxes))
	}
}

func TestGatherKeepsEveryBoxOfAShapeItTouches(t *testing.T) {
	// The game gathers a block's shape whole: one collider, every box in it. So a
	// shape the region reaches by one box contributes all of them, and a shape it
	// reaches by none contributes nothing.
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1}, geom.BlockPos{X: 1}, geom.EmptyShape())
	stair := geom.NewShape(
		geom.AABB{MaxX: 1, MaxY: 0.5, MaxZ: 1},
		geom.AABB{MinY: 0.5, MaxX: 1, MaxY: 1, MaxZ: 0.5},
	)
	blocks.Set(geom.BlockPos{}, stair)

	// Only the lower box is in reach, and both boxes come back.
	got, err := Gather(blocks, geom.AABB{MaxX: 0.5, MaxY: 0.25, MaxZ: 1}, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(got.Boxes) != 2 {
		t.Fatalf("Gather returned %d boxes, want both boxes of the shape", len(got.Boxes))
	}
}

func TestGatherEnforcesTheCandidateLimit(t *testing.T) {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -4, Y: -4, Z: -4}, geom.BlockPos{X: 4, Y: 4, Z: 4}, geom.FullCube())

	region := geom.AABB{MinX: -3, MinY: -3, MinZ: -3, MaxX: 3, MaxY: 3, MaxZ: 3}
	if _, err := Gather(blocks, region, 8); !errors.Is(err, ErrCandidateLimit) {
		t.Fatalf("Gather error = %v, want ErrCandidateLimit", err)
	}
}

func TestGatherOnAnEmptyViewAllocatesNoBoxes(t *testing.T) {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1, Y: -1, Z: -1}, geom.BlockPos{X: 1, Y: 1, Z: 1}, geom.EmptyShape())

	got, err := Gather(blocks, geom.AABB{MaxX: 0.5, MaxY: 0.5, MaxZ: 0.5}, 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(got.Boxes) != 0 || len(got.Unknown) != 0 {
		t.Fatalf("Gather = %+v, want nothing", got)
	}
}
