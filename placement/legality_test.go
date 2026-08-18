package placement_test

import (
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/placement"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// stoneRef is the handle the fake world mints for a solid block. Any non-zero
// handle would do; what matters is that the replaceable predicate below can
// tell it from the one air carries.
const stoneRef = world.BlockRef(1)

// blocksWith returns a view holding one described cell.
func blocksWith(t *testing.T, at geom.BlockPos, ref world.BlockRef, shape geom.Shape) *world.Blocks {
	t.Helper()

	blocks := world.NewBlocks()
	blocks.SetBlock(at, ref, shape)

	return blocks
}

// noEntities is a view with nothing in it.
func noEntities(t *testing.T) entity.View {
	t.Helper()

	return entity.NewBodies()
}

// standingAt puts a player-sized body with its feet on the floor of a cell.
func standingAt(t *testing.T, at geom.BlockPos) entity.View {
	t.Helper()

	bodies := entity.NewBodies()
	bodies.Set(1, entity.State{
		Family: entity.FamilyPlayer,
		Box: geom.AABB{
			MinX: float64(at.X) + 0.2, MinY: float64(at.Y), MinZ: float64(at.Z) + 0.2,
			MaxX: float64(at.X) + 0.8, MaxY: float64(at.Y) + 1.8, MaxZ: float64(at.Z) + 0.8,
		},
	})

	return bodies
}

// bottomSlab is the shape of a slab sitting in the lower half of its cell.
func bottomSlab() geom.Shape {
	return geom.NewShape(geom.AABB{MaxX: 1, MaxY: 0.5, MaxZ: 1})
}

// nothingIsReplaceable is the predicate a version supplies for a world of solid
// blocks. Air never reaches it: the view answers that itself.
func nothingIsReplaceable(world.BlockRef) bool { return false }

func TestPlacingIntoASolidBlockIsRefused(t *testing.T) {
	t.Parallel()

	view := blocksWith(t, geom.BlockPos{}, stoneRef, geom.FullCube())

	got, complete := placement.Check(view, noEntities(t),
		placement.Target{Placed: geom.BlockPos{}}, nothingIsReplaceable,
		geom.FullCube(), geom.Vec3{Y: 2}, 4.5)
	if !complete.Complete {
		t.Fatalf("incomplete against a known block: %v", complete.Missing)
	}
	if got.Allowed {
		t.Fatal("placing into stone was allowed")
	}
	if got.Reason != placement.ReasonOccupied {
		t.Fatalf("Reason = %q, want %q", got.Reason, placement.ReasonOccupied)
	}
}

func TestPlacingIntoAReplaceableBlockIsAllowed(t *testing.T) {
	t.Parallel()

	// The other half of the rule: the cell holds a block, and the version says
	// that block is replaced rather than placed against. Tall grass and water
	// are the cases, and a check that refused every occupied cell would refuse
	// a placement every player makes.
	view := blocksWith(t, geom.BlockPos{}, stoneRef, geom.FullCube())

	got, _ := placement.Check(view, noEntities(t),
		placement.Target{Placed: geom.BlockPos{}, Replacing: true},
		func(ref world.BlockRef) bool { return ref == stoneRef },
		geom.FullCube(), geom.Vec3{Y: 2}, 4.5)
	if !got.Allowed {
		t.Fatalf("placing into a replaceable block was refused: %s", got.Reason)
	}
}

func TestPlacingThroughAnEntityIsRefused(t *testing.T) {
	t.Parallel()

	view := world.NewBlocks()
	view.SetAir(geom.BlockPos{})

	got, _ := placement.Check(view, standingAt(t, geom.BlockPos{}),
		placement.Target{Placed: geom.BlockPos{}}, nothingIsReplaceable,
		geom.FullCube(), geom.Vec3{Y: 2}, 4.5)
	if got.Allowed {
		t.Fatal("placing a full block through a standing entity was allowed")
	}
	if got.Reason != placement.ReasonEntity {
		t.Fatalf("Reason = %q, want %q", got.Reason, placement.ReasonEntity)
	}
}

func TestABottomSlabDoesNotCollideWithAMobStandingOnIt(t *testing.T) {
	t.Parallel()

	// The other half of the rule above, and the reason the check takes a shape
	// rather than assuming a cube. The body's feet are at the top of the slab's
	// cell, which a full block would reach and a bottom slab does not.
	view := world.NewBlocks()
	view.SetAir(geom.BlockPos{})

	got, _ := placement.Check(view, standingAt(t, geom.BlockPos{Y: 1}),
		placement.Target{Placed: geom.BlockPos{}}, nothingIsReplaceable,
		bottomSlab(), geom.Vec3{Y: 3}, 4.5)
	if !got.Allowed {
		t.Fatalf("placing a bottom slab under a standing mob was refused: %s", got.Reason)
	}
}

func TestPlacingBeyondReachIsRefused(t *testing.T) {
	t.Parallel()

	view := world.NewBlocks()
	view.SetAir(geom.BlockPos{X: 100})

	got, _ := placement.Check(view, noEntities(t),
		placement.Target{Placed: geom.BlockPos{X: 100}}, nothingIsReplaceable,
		geom.FullCube(), geom.Vec3{Y: 2}, 4.5)
	if got.Allowed {
		t.Fatal("placing a hundred blocks away was allowed")
	}
	if got.Reason != placement.ReasonOutOfReach {
		t.Fatalf("Reason = %q, want %q", got.Reason, placement.ReasonOutOfReach)
	}
}

func TestReachIsMeasuredToTheBlockRatherThanItsCentre(t *testing.T) {
	t.Parallel()

	// An eye 4.6 blocks from the centre of a cell is 4.1 from its near face,
	// and the game measures to the block. Measuring to the centre refuses a
	// placement a player can make, which reads as a reach that is short by half
	// a block everywhere.
	view := world.NewBlocks()
	view.SetAir(geom.BlockPos{X: 5})

	got, _ := placement.Check(view, noEntities(t),
		placement.Target{Placed: geom.BlockPos{X: 5}}, nothingIsReplaceable,
		geom.FullCube(), geom.Vec3{X: 0.5, Y: 0.5, Z: 0.5}, 4.5)
	if !got.Allowed {
		t.Fatalf("a cell 4.5 blocks from the eye by its near face was refused: %s", got.Reason)
	}
}

func TestPlacingAgainstAnUnknownBlockIsIncompleteNotRefused(t *testing.T) {
	t.Parallel()

	// A cell the client has not received is not a cell that refuses placement.
	// Guessing air here places blocks into walls, and the tri-state view exists
	// to stop exactly that.
	got, complete := placement.Check(world.NewBlocks(), noEntities(t),
		placement.Target{Placed: geom.BlockPos{X: 1}}, nothingIsReplaceable,
		geom.FullCube(), geom.Vec3{Y: 2}, 4.5)
	if complete.Complete {
		t.Fatal("a placement against an unknown cell reported a complete decision")
	}
	if len(complete.Missing) == 0 {
		t.Fatal("an incomplete decision named nothing missing")
	}
	if complete.Missing[0].Block != (geom.BlockPos{X: 1}) {
		t.Fatalf("named %v as missing, want the placed cell", complete.Missing[0])
	}
	if got.Allowed {
		t.Fatal("an incomplete decision allowed the placement")
	}
}
