package terrain

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// testBody is a two-block body 0.6 wide. The numbers are the test's, not the
// package's: terrain owns no version constant, so a body arrives as a value.
var testBody = Body{HalfWidth: 0.3, Height: 1.8, StepHeight: 0.6}

// room returns a view with a 5x5x5 air pocket floored with full cubes at y=-1.
func room() *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -2, Y: 0, Z: -2}, geom.BlockPos{X: 2, Y: 4, Z: 2}, geom.EmptyShape())
	blocks.Fill(geom.BlockPos{X: -2, Y: -1, Z: -2}, geom.BlockPos{X: 2, Y: -1, Z: 2}, geom.FullCube())

	return blocks
}

func TestFitsReportsClearAirAsClear(t *testing.T) {
	query := Query{View: room(), Body: testBody}

	fit, err := query.Fits(FeetOf(geom.BlockPos{X: 0, Y: 0, Z: 0}))
	if err != nil {
		t.Fatalf("Fits returned an error: %v", err)
	}
	if fit != FitClear {
		t.Fatalf("Fits = %v, want FitClear", fit)
	}
}

func TestFitsReportsASolidCellAsBlocked(t *testing.T) {
	blocks := room()
	blocks.Set(geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.FullCube())
	query := Query{View: blocks, Body: testBody}

	fit, err := query.Fits(FeetOf(geom.BlockPos{X: 0, Y: 0, Z: 0}))
	if err != nil {
		t.Fatalf("Fits returned an error: %v", err)
	}
	if fit != FitBlocked {
		t.Fatalf("Fits = %v, want FitBlocked", fit)
	}
}

// A two-block body whose head is in stone does not fit, even though its feet
// are in air. A test that only described the feet cell would pass on a
// one-cell implementation.
func TestFitsChecksTheHeadCell(t *testing.T) {
	blocks := room()
	blocks.Set(geom.BlockPos{X: 0, Y: 1, Z: 0}, geom.FullCube())
	query := Query{View: blocks, Body: testBody}

	fit, err := query.Fits(FeetOf(geom.BlockPos{X: 0, Y: 0, Z: 0}))
	if err != nil {
		t.Fatalf("Fits returned an error: %v", err)
	}
	if fit != FitBlocked {
		t.Fatalf("Fits = %v, want FitBlocked", fit)
	}
}

// Resting exactly on a floor is not an overlap. geom.AABB.Intersects excludes
// shared faces for this reason, and a body standing on the ground must fit.
func TestFitsIgnoresTheFloorItStandsOn(t *testing.T) {
	query := Query{View: room(), Body: testBody}

	fit, err := query.Fits(geom.Vec3{X: 0.5, Y: 0, Z: 0.5})
	if err != nil {
		t.Fatalf("Fits returned an error: %v", err)
	}
	if fit != FitClear {
		t.Fatalf("Fits = %v, want FitClear", fit)
	}
}

func TestFitsReportsUnknownForAnUndescribedCell(t *testing.T) {
	blocks := room()
	blocks.Forget(geom.BlockPos{X: 0, Y: 1, Z: 0})
	query := Query{View: blocks, Body: testBody}

	fit, err := query.Fits(FeetOf(geom.BlockPos{X: 0, Y: 0, Z: 0}))
	if err != nil {
		t.Fatalf("Fits returned an error: %v", err)
	}
	if fit != FitUnknown {
		t.Fatalf("Fits = %v, want FitUnknown", fit)
	}
}
