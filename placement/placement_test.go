package placement_test

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mining"
	"github.com/go-theft-craft/minecraft-simulation/placement"
)

func TestClickingASolidFacePlacesAgainstIt(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		face mining.Face
		want geom.BlockPos
	}{
		{mining.FaceTop, geom.BlockPos{X: 0, Y: 1, Z: 0}},
		{mining.FaceBottom, geom.BlockPos{X: 0, Y: -1, Z: 0}},
		{mining.FaceNorth, geom.BlockPos{X: 0, Y: 0, Z: -1}},
		{mining.FaceSouth, geom.BlockPos{X: 0, Y: 0, Z: 1}},
		{mining.FaceWest, geom.BlockPos{X: -1, Y: 0, Z: 0}},
		{mining.FaceEast, geom.BlockPos{X: 1, Y: 0, Z: 0}},
	} {
		got := placement.Resolve(geom.BlockPos{}, test.face, false)
		if got.Placed != test.want {
			t.Errorf("face %v placed at %v, want %v", test.face, got.Placed, test.want)
		}
		if got.Clicked != (geom.BlockPos{}) {
			t.Errorf("face %v moved the clicked cell to %v", test.face, got.Clicked)
		}
		if got.Replacing {
			t.Errorf("face %v reported replacing a solid block", test.face)
		}
	}
}

func TestClickingAReplaceableBlockPlacesIntoIt(t *testing.T) {
	t.Parallel()

	// Grass, tall grass, snow layers, and water are replaced in place. The face
	// is irrelevant when the clicked block is replaceable, and honouring it
	// anyway is what puts the block one cell off.
	got := placement.Resolve(geom.BlockPos{X: 3, Y: 64, Z: 3}, mining.FaceTop, true)
	if got.Placed != (geom.BlockPos{X: 3, Y: 64, Z: 3}) {
		t.Fatalf("placed at %v, want the clicked cell itself", got.Placed)
	}
	if !got.Replacing {
		t.Fatal("Replacing = false when replacing a replaceable block")
	}
}

func TestEveryFaceIsItsOwnDirection(t *testing.T) {
	t.Parallel()

	// The six faces must reach six distinct cells. A switch missing a case
	// returns the clicked cell, which is the same answer a replacement gives —
	// so the defect would show up as a block placed inside the one clicked,
	// and the test above would still pass.
	seen := make(map[geom.BlockPos]mining.Face, 6)
	for _, face := range []mining.Face{
		mining.FaceBottom, mining.FaceTop, mining.FaceNorth,
		mining.FaceSouth, mining.FaceWest, mining.FaceEast,
	} {
		placed := placement.Resolve(geom.BlockPos{}, face, false).Placed
		if other, ok := seen[placed]; ok {
			t.Fatalf("faces %v and %v both place at %v", other, face, placed)
		}
		if placed == (geom.BlockPos{}) {
			t.Fatalf("face %v places into the cell that was clicked", face)
		}
		seen[placed] = face
	}
}
