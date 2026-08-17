package terrain

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// testFacts is a Facts whose answers a test states directly, keyed by the
// opaque handle. It is a map, but nothing here iterates it, so no output
// ordering depends on Go's map order.
type testFacts struct {
	hazards map[world.BlockRef]Hazard
	fluids  map[world.BlockRef]Fluid
}

func (f testFacts) Hazard(ref world.BlockRef) Hazard { return f.hazards[ref] }
func (f testFacts) Fluid(ref world.BlockRef) Fluid   { return f.fluids[ref] }

const (
	refLava   world.BlockRef = 10
	refCactus world.BlockRef = 81
)

func lavaWorld() (*world.Blocks, testFacts) {
	blocks := room()
	// Lava has no collision shape. A body that consulted only geometry would
	// find this cell clear and walk into it, which is the whole reason Facts
	// exists.
	blocks.SetBlock(geom.BlockPos{X: 1, Y: 0, Z: 0}, refLava, geom.EmptyShape())
	blocks.SetBlock(geom.BlockPos{X: 2, Y: 0, Z: 0}, refCactus, geom.FullCube())

	facts := testFacts{
		hazards: map[world.BlockRef]Hazard{refLava: HazardBurn, refCactus: HazardContact},
		fluids:  map[world.BlockRef]Fluid{refLava: FluidLava},
	}

	return blocks, facts
}

func TestHazardAtReportsBurnForLava(t *testing.T) {
	blocks, facts := lavaWorld()
	query := Query{View: blocks, Facts: facts, Body: testBody}

	hazard, lookup, err := query.HazardAt(geom.BlockPos{X: 1, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("HazardAt returned an error: %v", err)
	}
	if lookup == world.LookupUnknown {
		t.Fatal("HazardAt reported unknown for a described cell")
	}
	if hazard != HazardBurn {
		t.Fatalf("HazardAt = %v, want HazardBurn", hazard)
	}
}

func TestHazardAtReportsContactForCactus(t *testing.T) {
	blocks, facts := lavaWorld()
	query := Query{View: blocks, Facts: facts, Body: testBody}

	hazard, _, err := query.HazardAt(geom.BlockPos{X: 2, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("HazardAt returned an error: %v", err)
	}
	if hazard != HazardContact {
		t.Fatalf("HazardAt = %v, want HazardContact", hazard)
	}
}

func TestHazardAtReportsUnknownForAnUndescribedCell(t *testing.T) {
	blocks, facts := lavaWorld()
	blocks.Forget(geom.BlockPos{X: 1, Y: 0, Z: 0})
	query := Query{View: blocks, Facts: facts, Body: testBody}

	_, lookup, err := query.HazardAt(geom.BlockPos{X: 1, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("HazardAt returned an error: %v", err)
	}
	if lookup != world.LookupUnknown {
		t.Fatalf("HazardAt lookup = %v, want LookupUnknown", lookup)
	}
}

// A nil Facts is legal and answers "nothing special", which is what a caller
// that only cares about geometry wants.
func TestHazardAtToleratesNilFacts(t *testing.T) {
	query := Query{View: room(), Body: testBody}

	hazard, _, err := query.HazardAt(geom.BlockPos{X: 0, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("HazardAt returned an error: %v", err)
	}
	if hazard != HazardNone {
		t.Fatalf("HazardAt = %v, want HazardNone", hazard)
	}
}

func TestFluidAtReportsLava(t *testing.T) {
	blocks, facts := lavaWorld()
	query := Query{View: blocks, Facts: facts, Body: testBody}

	fluid, _, err := query.FluidAt(geom.BlockPos{X: 1, Y: 0, Z: 0})
	if err != nil {
		t.Fatalf("FluidAt returned an error: %v", err)
	}
	if fluid != FluidLava {
		t.Fatalf("FluidAt = %v, want FluidLava", fluid)
	}
}
