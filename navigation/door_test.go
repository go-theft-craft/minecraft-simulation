package navigation

import (
	"context"
	"slices"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/terrain"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

const (
	refWoodenDoor world.BlockRef = 64
	refIronDoor   world.BlockRef = 71
)

// doorFacts answers doors and nothing else.
type doorFacts struct{}

func (doorFacts) Hazard(world.BlockRef) terrain.Hazard { return terrain.HazardNone }
func (doorFacts) Fluid(world.BlockRef) terrain.Fluid   { return terrain.FluidNone }
func (doorFacts) Climbable(world.BlockRef) bool        { return false }

func (doorFacts) Door(ref world.BlockRef) terrain.Door {
	switch ref {
	case refWoodenDoor:
		return terrain.DoorOperable
	case refIronDoor:
		return terrain.DoorLocked
	default:
		return terrain.DoorNone
	}
}

func doorCapability() Capability {
	capability := walker
	capability.CanOpenDoors = true
	capability.DoorTicks = 29

	return capability
}

// corridorWithDoor returns a one-cell-wide corridor with a door across it.
//
// The door is a full cube while closed, which is what a closed door does to a
// collision sweep: it fills the passage. Both of its halves are placed, because
// both versions hang a door as two cells.
func corridorWithDoor(ref world.BlockRef, wide bool) *world.Blocks {
	maxZ := int32(0)
	if wide {
		maxZ = 1
	}

	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -1, Y: -1, Z: -1}, geom.BlockPos{X: 7, Y: -1, Z: maxZ + 1}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -1, Y: 0, Z: -1}, geom.BlockPos{X: 7, Y: 3, Z: maxZ + 1}, geom.EmptyShape())
	// Walls that make it a corridor rather than open ground.
	blocks.Fill(geom.BlockPos{X: -1, Y: 0, Z: -1}, geom.BlockPos{X: 7, Y: 2, Z: -1}, geom.FullCube())
	blocks.Fill(geom.BlockPos{X: -1, Y: 0, Z: maxZ + 1}, geom.BlockPos{X: 7, Y: 2, Z: maxZ + 1}, geom.FullCube())

	for z := int32(0); z <= maxZ; z++ {
		for y := int32(0); y < doorHeight; y++ {
			blocks.SetBlock(geom.BlockPos{X: 3, Y: y, Z: z}, ref, geom.FullCube())
		}
	}

	return blocks
}

// TestAnOpenedDoorNeverInvalidatesAnEarlierEdge is the test the design asks for
// before the edge is allowed to exist outside the mutating amendment.
//
// The amendment excludes mutation because a placed block can make an earlier
// edge illegal, which is what forces the overlay and the re-search loop. A door
// is admitted as an exception only if opening one can never do that. Its failure
// would be a finding rather than a bug: it would mean EdgeDoor belongs in the
// other amendment after all.
//
// The finding is that it cannot. Opening a door swings it inside its own cell,
// so the only cell whose contents change is the door's, and it changes from
// filled to open. Every earlier edge is re-checked here against the world as it
// stands after each toggle, and none of them can lose ground that only ever
// gains it.
func TestAnOpenedDoorNeverInvalidatesAnEarlierEdge(t *testing.T) {
	view := corridorWithDoor(refWoodenDoor, false)
	capability := doorCapability()

	path, err := Find(
		context.Background(), view, doorFacts{}, capability,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 6, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("the corridor was not routed: %v", path.Reason)
	}
	if !slices.ContainsFunc(path.Edges, func(e Edge) bool { return e.Kind == EdgeDoor }) {
		t.Fatalf("passed a door without a door edge: %v", path.Edges)
	}

	// Walk the path, opening each door as it is reached, and re-check every
	// edge taken before it. This is the validation loop the parent design
	// specifies, written here once to answer one question.
	var opened world.View = view
	for i, edge := range path.Edges {
		if edge.Kind != EdgeDoor {
			continue
		}
		opened = applyOpen(opened, edge.To)

		for j, earlier := range path.Edges[:i] {
			if legal := edgeIsStillLegal(t, opened, capability, earlier); !legal {
				t.Fatalf(
					"opening the door at %v made edge %d (%v %v -> %v) illegal.\n"+
						"This is the finding the design predicted: EdgeDoor needs the "+
						"overlay and belongs in the mutating edges plan.",
					edge.To, j, earlier.Kind, earlier.From, earlier.To,
				)
			}
		}
	}
}

// applyOpen returns the world with one door swung open.
func applyOpen(view world.View, door geom.BlockPos) world.View {
	return openedView{view: view, door: door}
}

// edgeIsStillLegal re-checks that an edge's destination is somewhere the body
// can be, against the world as it now stands.
func edgeIsStillLegal(t *testing.T, view world.View, capability Capability, edge Edge) bool {
	t.Helper()

	o := directOracle{
		query:      capability.query(view, doorFacts{}),
		capability: capability,
		crawlQuery: capability.crawling().query(view, doorFacts{}),
	}

	// A door edge's own destination holds a door, which is only passable while
	// it is open — and it is, since this is called after the toggle.
	if edge.Kind == EdgeDoor {
		passable, err := o.passableThroughDoor(edge.To)
		if err != nil {
			t.Fatalf("passableThroughDoor: %v", err)
		}

		return passable == terrain.Clear
	}

	passable, err := o.passable(edge.To)
	if err != nil {
		t.Fatalf("passable: %v", err)
	}

	return passable == terrain.Clear
}

// TestAClosedDoorStopsABodyThatCannotOpenIt is the control for the test above:
// the door really does block the corridor, so routing through it is an opening
// rather than a walk past something that was never in the way.
func TestAClosedDoorStopsABodyThatCannotOpenIt(t *testing.T) {
	path, err := Find(
		context.Background(), corridorWithDoor(refWoodenDoor, false), doorFacts{}, walker,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 6, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if path.Complete {
		t.Fatal("a body that cannot open doors walked through a closed one")
	}
}

// TestAnIronDoorIsNotOpened pins the refusal.
//
// An iron door needs redstone, and a bot standing at one forever is worse than
// one that never tried. It is reported as a locked door rather than as not a
// door, so the search knows to route around it rather than treating it as a
// wall it never understood.
func TestAnIronDoorIsNotOpened(t *testing.T) {
	path, err := Find(
		context.Background(), corridorWithDoor(refIronDoor, false), doorFacts{}, doorCapability(),
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 6, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	for _, edge := range path.Edges {
		if edge.Kind == EdgeDoor {
			t.Fatal("produced a door edge for an iron door")
		}
	}
	if path.Complete {
		t.Fatal("walked through an iron door")
	}
}

// TestAnIronDoorIsRoutedAroundWhenThereIsAWayRound is the same refusal with a
// detour available.
func TestAnIronDoorIsRoutedAroundWhenThereIsAWayRound(t *testing.T) {
	// The corridor is two cells wide and the iron door fills only the first,
	// so the second row is the way round.
	view := corridorWithDoor(refIronDoor, true)
	for y := int32(0); y < doorHeight; y++ {
		view.Set(geom.BlockPos{X: 3, Y: y, Z: 1}, geom.EmptyShape())
	}

	path, err := Find(
		context.Background(), view, doorFacts{}, doorCapability(),
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 6, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	if !path.Complete {
		t.Fatalf("the detour around the iron door was not found: %v", path.Reason)
	}
	for _, edge := range path.Edges {
		if edge.Kind == EdgeDoor {
			t.Fatal("produced a door edge for an iron door")
		}
	}
}

// TestADoorCostsItsOwnPrice pins that opening is priced apart from walking, so
// a search prefers an open corridor to a door when both reach.
func TestADoorCostsItsOwnPrice(t *testing.T) {
	capability := doorCapability()

	path, err := Find(
		context.Background(), corridorWithDoor(refWoodenDoor, false), doorFacts{}, capability,
		geom.BlockPos{X: 0, Y: 0, Z: 0}, geom.BlockPos{X: 6, Y: 0, Z: 0}, wideBudget,
	)
	if err != nil {
		t.Fatalf("Find returned an error: %v", err)
	}
	for _, edge := range path.Edges {
		if edge.Kind == EdgeDoor && edge.Cost != capability.DoorTicks {
			t.Errorf("a door cost %v, want %v", edge.Cost, capability.DoorTicks)
		}
	}
}

// TestAnOpenDoorLeavesRoomForACentredBody records the geometry the exception
// rests on.
//
// An open door is a slab three sixteenths of a block thick against one side of
// its own cell, and a body is centred and six tenths wide. The two cannot meet,
// which is why opening a door frees space and never takes any — and therefore
// why this edge needs no overlay.
func TestAnOpenDoorLeavesRoomForACentredBody(t *testing.T) {
	const (
		doorThickness = 3.0 / 16.0
		halfWidth     = 0.3
	)

	// The body spans the middle of the cell; the slab hugs one face of it.
	bodyMin, bodyMax := 0.5-halfWidth, 0.5+halfWidth
	if bodyMin <= doorThickness {
		t.Errorf("a body starting at %v overlaps a door slab %v thick", bodyMin, doorThickness)
	}
	if bodyMax >= 1-doorThickness {
		t.Errorf("a body ending at %v overlaps a door slab on the far face", bodyMax)
	}
}
