package v1_8

import (
	"testing"

	"github.com/go-theft-craft/minecraft-protocol/data"
	gen "github.com/go-theft-craft/minecraft-protocol/generated/java/v1_8"

	"github.com/go-theft-craft/minecraft-simulation/geom"
)

// dataset returns the 1.8.9 game data every test here is built from.
func dataset(t *testing.T) *data.Set {
	t.Helper()

	set, err := gen.Data()
	if err != nil {
		t.Fatalf("load the 1.8.9 data set: %v", err)
	}

	return set
}

func table(t *testing.T) blockTable {
	t.Helper()

	built, err := newBlockTable(dataset(t))
	if err != nil {
		t.Fatalf("newBlockTable: %v", err)
	}

	return built
}

func TestNewBlockTableRejectsNothing(t *testing.T) {
	if _, err := newBlockTable(nil); err == nil {
		t.Fatal("newBlockTable accepted a nil data set")
	}
}

func TestTheTableResolvesStoneAndAir(t *testing.T) {
	built := table(t)

	stone, ok := built.ref("stone")
	if !ok {
		t.Fatal("the table does not know stone")
	}
	shape, ok := built.shape(stone)
	if !ok {
		t.Fatal("the table could not resolve its own handle for stone")
	}
	if shape.Len() != 1 {
		t.Fatalf("stone resolves to %d boxes, want the one of a full cube", shape.Len())
	}

	air, ok := built.ref("air")
	if !ok {
		t.Fatal("the table does not know air")
	}
	shape, ok = built.shape(air)
	if !ok {
		t.Fatal("the table could not resolve its own handle for air")
	}
	if !shape.IsEmpty() {
		t.Fatalf("air resolves to %d boxes, want none", shape.Len())
	}
}

func TestSlipperinessComesFromTheDataset(t *testing.T) {
	built := table(t)

	// Ice and slime differ from the default, which is how a test tells a wired-up
	// table from one that quietly defaults everything. Soul sand is here for the
	// opposite reason: it slows a player through a different mechanism, so its
	// slipperiness *is* the default, and a test that expected otherwise would be
	// asserting a mechanic this milestone does not implement.
	for name, want := range map[string]float32{
		"stone":      0.6,
		"ice":        0.98,
		"packed_ice": 0.98,
		"slime":      0.8,
		"soul_sand":  0.6,
	} {
		ref, ok := built.ref(name)
		if !ok {
			t.Errorf("the table does not know %s", name)

			continue
		}
		if got := built.slipperiness(ref); got != want {
			t.Errorf("slipperiness of %s = %v, want %v", name, got, want)
		}
	}
}

func TestAnUnknownHandleReportsFalse(t *testing.T) {
	built := table(t)

	if _, ok := built.shape(0); ok {
		t.Error("the zero handle resolved to a shape; it carries no meaning")
	}
	if _, ok := built.shape(1 << 20); ok {
		t.Error("a handle beyond the table resolved to a shape")
	}
	// Slipperiness still answers, with the default: a rule asking about a cell it
	// could not identify should get the ordinary friction rather than zero, which
	// would be a body that never slows down.
	if got, want := built.slipperiness(0), float32(0.6); got != want {
		t.Errorf("slipperiness of the zero handle = %v, want the default %v", got, want)
	}
}

func TestTheDatasetStoresSlipperinessUnwidened(t *testing.T) {
	// A finding worth stating rather than papering over.
	//
	// The dataset records a block's slipperiness as the round decimal — 0.6, 0.98
	// — while the game's field is a float, so the value the game computes with is
	// float32(0.6), which widens back to 0.6000000238418579. That is the opposite
	// of how the same dataset records an entity's step height, which it stores
	// already widened.
	//
	// Narrowing at the table boundary is therefore not a lossy convenience: it is
	// what recovers the width the game uses. This test pins the asymmetry so that
	// a later dataset which starts storing widened slipperiness is noticed here
	// rather than as a drifting trajectory.
	physics := dataset(t).Physics()

	if got := float64(float32(physics.DefaultSlipperiness)); got == physics.DefaultSlipperiness {
		t.Fatalf("the default slipperiness %v now round-trips through float32; "+
			"the dataset may have started storing widened values",
			physics.DefaultSlipperiness)
	}

	// Whatever the storage, narrowing must not lose more than single-width
	// rounding: a value that moved further than that would be a dataset fault.
	for name, value := range physics.BlockSlipperiness {
		narrowed := float64(float32(value))
		if diff := narrowed - value; diff > 1e-7 || diff < -1e-7 {
			t.Errorf("slipperiness of %s is %v, which narrows to %v", name, value, narrowed)
		}
	}
}

func TestHandlesAreUniqueAndNameTheirBlock(t *testing.T) {
	built := table(t)

	seen := make(map[string]bool)
	for _, name := range []string{"stone", "dirt", "ice", "air", "slime"} {
		ref, ok := built.ref(name)
		if !ok {
			t.Fatalf("the table does not know %s", name)
		}
		if got := built.name(ref); got != name {
			t.Fatalf("handle %d names %q, want %q", ref, got, name)
		}
		if seen[name] {
			t.Fatalf("%s appears twice", name)
		}
		seen[name] = true
	}
}

func TestAHandleNamesAStateRatherThanABlock(t *testing.T) {
	built := table(t)

	// The defect this table learned metadata to fix. A stone slab's two halves
	// are one block and two states, and they do not stand in the same volume:
	// the dataset lists the bottom half for metadata 0 through 7 and the top
	// half for 8 through 15. A table with one handle per block answered the
	// bottom half for both, so a body walking under a top slab walked through
	// it.
	bottom, ok := built.refState("stone_slab", 0)
	if !ok {
		t.Fatal("the table does not know stone_slab")
	}
	top, ok := built.refState("stone_slab", 8)
	if !ok {
		t.Fatal("the table has no metadata 8 for stone_slab")
	}

	bottomShape, ok := built.shape(bottom)
	if !ok {
		t.Fatal("the bottom slab's handle resolves no shape")
	}
	topShape, ok := built.shape(top)
	if !ok {
		t.Fatal("the top slab's handle resolves no shape")
	}
	if bottomShape.BoxesAt(geom.BlockPos{}, nil)[0] == topShape.BoxesAt(geom.BlockPos{}, nil)[0] {
		t.Fatal("a top slab and a bottom slab resolve the same box")
	}

	// A name still resolves what it always resolved: the state the game calls
	// this block's default. Every fixture, scene, and trace names blocks, and
	// none of them was rewritten when the table grew.
	if plain, _ := built.ref("stone_slab"); plain != bottom {
		t.Fatalf("stone_slab resolves handle %d, want metadata zero's %d", plain, bottom)
	}
	if got := built.metadata(top); got != 8 {
		t.Fatalf("the top slab's handle carries metadata %d, want 8", got)
	}
	if got := built.name(top); got != "stone_slab" {
		t.Fatalf("the top slab's handle names %q", got)
	}
}

func TestMetadataOutsideFourBitsIsRefused(t *testing.T) {
	built := table(t)

	// A wrapped metadata names a different state, and a placement rule that
	// computed one would place a different block rather than fail.
	for _, metadata := range []int{-1, 16, 32} {
		if _, ok := built.refState("stone_slab", metadata); ok {
			t.Errorf("metadata %d resolved a handle", metadata)
		}
	}
}
