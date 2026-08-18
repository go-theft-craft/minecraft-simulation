package v26_1

import (
	"slices"
	"testing"

	"github.com/go-theft-craft/minecraft-protocol/data"
	gen "github.com/go-theft-craft/minecraft-protocol/generated/java/v26_1"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// dataset returns the 26.1.2 game data every test here is built from.
func dataset(t *testing.T) *data.Set {
	t.Helper()

	set, err := gen.Data()
	if err != nil {
		t.Fatalf("load the 26.1.2 data set: %v", err)
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

// block returns one block's dataset entry, so that a test names a block rather
// than a state number: the numbers move when the game adds a block, and the
// relationships this table promises do not.
func block(t *testing.T, name string) data.Block {
	t.Helper()

	found, ok := dataset(t).Blocks().ByName(name)
	if !ok {
		t.Fatalf("the data set does not know %s", name)
	}

	return found
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

	// Five blocks differ from the default in this version where three do in
	// 1.8.9, and one of the five is named differently: 1.8.9's slime is
	// slime_block here. Blue ice is not packed ice either — it is the one value
	// that is neither the default nor 0.98, and a table carried over by analogy
	// would have missed all three facts.
	//
	// Soul sand and honey are here for the opposite reason: they slow a body
	// through the block speed factor, which is a step in this version's tick and
	// not a friction, so their slipperiness *is* the default.
	for name, want := range map[string]float32{
		"stone":       0.6,
		"ice":         0.98,
		"packed_ice":  0.98,
		"frosted_ice": 0.98,
		"blue_ice":    0.989,
		"slime_block": 0.8,
		"soul_sand":   0.6,
		"honey_block": 0.6,
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

	// 1.8.9's name for the slime block resolves to nothing here. A fixture that
	// carried the old name over would otherwise place air and say nothing.
	if _, ok := built.ref("slime"); ok {
		t.Error("the table knows slime; this version calls it slime_block")
	}
}

func TestEveryStateOfABlockSharesItsFriction(t *testing.T) {
	built := table(t)

	// Friction is the block's in the game, so a stair facing east is as slippery
	// as one facing west. The shape is the state's; the friction is not.
	ice := block(t, "blue_ice")
	for state := ice.MinStateID; state <= ice.MaxStateID; state++ {
		ref, ok := built.refState(state)
		if !ok {
			t.Fatalf("the table does not know blue ice state %d", state)
		}
		if got := built.slipperiness(ref); got != 0.989 {
			t.Errorf("slipperiness of blue ice state %d = %v, want 0.989", state, got)
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
	if got, want := built.slipperiness(1<<20), float32(0.6); got != want {
		t.Errorf("slipperiness of a handle beyond the table = %v, want the default %v", got, want)
	}
}

func TestTheDatasetStoresSlipperinessUnwidened(t *testing.T) {
	// The same finding as 1.8.9's, restated against this version's dataset
	// rather than assumed to carry over.
	//
	// The dataset records a block's slipperiness as the round decimal — 0.6,
	// 0.98 — while the game's field is a float, so the value the game computes
	// with is float32(0.6), which widens back to 0.6000000238418579. That is the
	// opposite of how the same dataset records this version's step height, which
	// it stores already widened at 0.6000000238418579.
	//
	// Narrowing at the table boundary is therefore not a lossy convenience: it is
	// what recovers the width the game uses.
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

	seen := make(map[world.BlockRef]string)
	for _, name := range []string{"stone", "dirt", "ice", "air", "slime_block", "oak_stairs"} {
		ref, ok := built.ref(name)
		if !ok {
			t.Fatalf("the table does not know %s", name)
		}
		if got := built.name(ref); got != name {
			t.Fatalf("handle %d names %q, want %q", ref, got, name)
		}
		if other, taken := seen[ref]; taken {
			t.Fatalf("%s and %s share handle %d", other, name, ref)
		}
		seen[ref] = name
	}
}

func TestAHandleIsAStateAndTheDefaultStateIsWhatANameResolvesTo(t *testing.T) {
	built := table(t)

	// The flattening is what makes this the natural handle: one number names a
	// block and its variant, and it is the number the protocol carries.
	for _, name := range []string{"stone", "air", "oak_slab", "oak_stairs", "oak_fence"} {
		entry := block(t, name)

		byName, ok := built.ref(name)
		if !ok {
			t.Fatalf("the table does not know %s", name)
		}
		byState, ok := built.refState(entry.DefaultState)
		if !ok {
			t.Fatalf("the table does not know %s state %d", name, entry.DefaultState)
		}
		if byName != byState {
			t.Errorf("%s resolves to handle %d by name and %d by its default state %d",
				name, byName, byState, entry.DefaultState)
		}

		// Every state the block owns is a handle of its own, and it names the
		// block it belongs to.
		for state := entry.MinStateID; state <= entry.MaxStateID; state++ {
			ref, ok := built.refState(state)
			if !ok {
				t.Fatalf("the table does not know %s state %d", name, state)
			}
			if got := built.name(ref); got != name {
				t.Errorf("state %d names %q, want %q", state, got, name)
			}
		}
	}
}

func TestRefStateRejectsAStateNothingDescribes(t *testing.T) {
	built := table(t)

	if _, ok := built.refState(-1); ok {
		t.Error("a negative state resolved to a handle")
	}
	if _, ok := built.refState(1 << 20); ok {
		t.Error("a state beyond the table resolved to a handle")
	}
}

func TestAStateCarriesItsOwnShape(t *testing.T) {
	built := table(t)

	// A slab is the shortest statement of why the handle is a state: its two
	// halves and its doubled form stand in three different volumes, and they are
	// three states of one block.
	slab := block(t, "oak_slab")

	bottom := geom.AABB{MaxX: 1, MaxY: 0.5, MaxZ: 1}
	top := geom.AABB{MinY: 0.5, MaxX: 1, MaxY: 1, MaxZ: 1}
	cube := geom.AABB{MaxX: 1, MaxY: 1, MaxZ: 1}

	defaultRef, ok := built.ref("oak_slab")
	if !ok {
		t.Fatal("the table does not know oak_slab")
	}
	shape, _ := built.shape(defaultRef)
	if got := shape.BoxesAt(geom.BlockPos{}, nil); len(got) != 1 || got[0] != bottom {
		t.Errorf("the default oak slab stands in %v, want the bottom half %v", got, bottom)
	}

	var sawTop, sawCube bool
	for state := slab.MinStateID; state <= slab.MaxStateID; state++ {
		ref, ok := built.refState(state)
		if !ok {
			t.Fatalf("the table does not know oak_slab state %d", state)
		}
		shape, ok := built.shape(ref)
		if !ok {
			t.Fatalf("the table could not resolve its own handle for oak_slab state %d", state)
		}
		boxes := shape.BoxesAt(geom.BlockPos{}, nil)
		if len(boxes) != 1 {
			t.Fatalf("oak_slab state %d stands in %d boxes, want one", state, len(boxes))
		}
		switch boxes[0] {
		case top:
			sawTop = true
		case cube:
			sawCube = true
		}
	}
	if !sawTop {
		t.Error("no state of oak_slab stands in the top half of its cell")
	}
	if !sawCube {
		t.Error("no state of oak_slab fills its cell")
	}
}

func TestABlockWithOneShapeGivesItToEveryState(t *testing.T) {
	built := table(t)

	// Grass has two states and the dataset names one shape for them, because
	// snow on top does not change what a body walks into. The rule that reads a
	// shape per state has to answer for that block too.
	grass := block(t, "grass_block")
	if grass.MaxStateID == grass.MinStateID {
		t.Skip("grass_block no longer varies over a property")
	}

	cube := geom.AABB{MaxX: 1, MaxY: 1, MaxZ: 1}
	for state := grass.MinStateID; state <= grass.MaxStateID; state++ {
		ref, ok := built.refState(state)
		if !ok {
			t.Fatalf("the table does not know grass_block state %d", state)
		}
		shape, _ := built.shape(ref)
		if got := shape.BoxesAt(geom.BlockPos{}, nil); len(got) != 1 || got[0] != cube {
			t.Errorf("grass_block state %d stands in %v, want the full cube %v", state, got, cube)
		}
	}
}

func TestASlabHandsTheStepUpItsGrid(t *testing.T) {
	built := table(t)

	// The shapes this table mints are what collision.ResolveVoxel steps up
	// against, and the step-up asks a shape for the heights it offers rather
	// than for its boxes. A slab offers halves.
	ref, ok := built.ref("oak_slab")
	if !ok {
		t.Fatal("the table does not know oak_slab")
	}
	shape, _ := built.shape(ref)

	want := []float64{0, 0.5, 1}
	if got := shape.GridY(); !slices.Equal(got, want) {
		t.Errorf("the default oak slab offers rises %v, want %v", got, want)
	}
}

func TestEveryBlockInTheDatasetIsInTheTable(t *testing.T) {
	set := dataset(t)
	built, err := newBlockTable(set)
	if err != nil {
		t.Fatalf("newBlockTable: %v", err)
	}

	blocks := set.Blocks().All()
	if len(built.names) != len(blocks)+1 {
		t.Fatalf("the table holds %d blocks, want the dataset's %d and the reserved one",
			len(built.names), len(blocks))
	}

	states := 0
	for _, entry := range blocks {
		states += int(entry.MaxStateID-entry.MinStateID) + 1
		if _, ok := built.ref(entry.Name); !ok {
			t.Fatalf("the table does not know %s", entry.Name)
		}
	}
	// Every handle in the table is a state, and every state is a handle: the
	// constructor rejects a hole, so the count is the check.
	if got := len(built.owner) - 1; got != states {
		t.Errorf("the table holds %d states, and the dataset describes %d", got, states)
	}
}

// fakeBlocks is a block registry a test builds by hand, for the datasets a
// generated one cannot produce.
type fakeBlocks data.Blocks

func (f fakeBlocks) ByID(id data.BlockID) (data.Block, bool) {
	for _, entry := range f {
		if entry.ID == id {
			return entry, true
		}
	}

	return data.Block{}, false
}

func (f fakeBlocks) ByName(name string) (data.Block, bool) {
	for _, entry := range f {
		if entry.Name == name {
			return entry, true
		}
	}

	return data.Block{}, false
}

func (f fakeBlocks) All() data.Blocks { return data.Blocks(f).Clone() }

// handmade builds a data set from blocks and shapes a test wrote, with physics
// that are wired up but uninteresting.
func handmade(t *testing.T, blocks data.Blocks, shapes data.CollisionShapes) *data.Set {
	t.Helper()

	set, err := data.NewSet(data.SetOptions{
		Blocks:          fakeBlocks(blocks),
		CollisionShapes: shapes,
		Physics: data.Physics{
			DefaultSlipperiness: 0.6,
			BlockSlipperiness:   data.BlockSlipperinessIndex{"stone": 0.6},
		},
	})
	if err != nil {
		t.Fatalf("build a data set: %v", err)
	}

	return set
}

// cube is the shape index a handmade set uses when the shapes are not the point
// of the test.
func cubeShapes(blocks ...string) data.CollisionShapes {
	shapes := data.CollisionShapes{
		Blocks: data.BlockShapeIndex{},
		Shapes: data.BoundingBoxIndex{1: {{MaxX: 1, MaxY: 1, MaxZ: 1}}},
	}
	for _, name := range blocks {
		shapes.Blocks[name] = data.ShapeIDs{1}
	}

	return shapes
}

func TestNewBlockTableRejectsWhatItCannotMap(t *testing.T) {
	full := data.ShapeIDs{1}

	for name, set := range map[string]*data.Set{
		"a set with no physics": func() *data.Set {
			built, err := data.NewSet(data.SetOptions{
				Blocks:          fakeBlocks{{Name: "stone", MinStateID: 0, MaxStateID: 0}},
				CollisionShapes: cubeShapes("stone"),
			})
			if err != nil {
				t.Fatalf("build a data set: %v", err)
			}

			return built
		}(),
		"a set with no blocks": handmade(t, nil, cubeShapes()),
		"a shape list that is neither one shape nor one per state": handmade(t,
			data.Blocks{{Name: "stone", MinStateID: 0, MaxStateID: 3}},
			data.CollisionShapes{
				Blocks: data.BlockShapeIndex{"stone": data.ShapeIDs{1, 1}},
				Shapes: data.BoundingBoxIndex{1: {{MaxX: 1, MaxY: 1, MaxZ: 1}}},
			}),
		"two blocks claiming one state": handmade(t,
			data.Blocks{
				{Name: "stone", MinStateID: 0, MaxStateID: 1},
				{Name: "dirt", MinStateID: 1, MaxStateID: 2},
			},
			cubeShapes("stone", "dirt")),
		"a hole in the state numbering": handmade(t,
			data.Blocks{
				{Name: "stone", MinStateID: 0, MaxStateID: 0},
				{Name: "dirt", MinStateID: 2, MaxStateID: 2},
			},
			cubeShapes("stone", "dirt")),
		"a default state outside its own block": handmade(t,
			data.Blocks{{Name: "stone", MinStateID: 0, MaxStateID: 1, DefaultState: 4}},
			cubeShapes("stone")),
		"a shape the set does not describe": handmade(t,
			data.Blocks{{Name: "stone", MinStateID: 0, MaxStateID: 0}},
			data.CollisionShapes{
				Blocks: data.BlockShapeIndex{"stone": full},
				Shapes: data.BoundingBoxIndex{},
			}),
	} {
		if _, err := newBlockTable(set); err == nil {
			t.Errorf("newBlockTable accepted %s", name)
		}
	}
}

func TestABlockWithNoShapeEntryHasNoCollider(t *testing.T) {
	// The dataset this version ships names a shape for every block, but the
	// index is a map and a version that stopped naming one for, say, a new plant
	// should give that block no collider rather than fail to build the table.
	set := handmade(t,
		data.Blocks{{Name: "stone", MinStateID: 0, MaxStateID: 0}},
		data.CollisionShapes{
			Blocks: data.BlockShapeIndex{},
			Shapes: data.BoundingBoxIndex{},
		})

	built, err := newBlockTable(set)
	if err != nil {
		t.Fatalf("newBlockTable: %v", err)
	}

	ref, ok := built.ref("stone")
	if !ok {
		t.Fatal("the table does not know stone")
	}
	shape, ok := built.shape(ref)
	if !ok {
		t.Fatal("the table could not resolve its own handle")
	}
	if !shape.IsEmpty() {
		t.Errorf("a block the shape index does not mention stands in %d boxes, want none", shape.Len())
	}
}
