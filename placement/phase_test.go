package placement_test

import (
	"testing"

	"github.com/go-theft-craft/minecraft-protocol/data"
	gen "github.com/go-theft-craft/minecraft-protocol/generated/java/v1_8"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mining"
	"github.com/go-theft-craft/minecraft-simulation/placement"
	v1_8 "github.com/go-theft-craft/minecraft-simulation/profile/java/v1_8"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// builder is the body every test here places with.
const builder entity.ID = 1

// clicked is the block every test here clicks, and above it is where a
// placement against its top face lands.
var (
	clicked = geom.BlockPos{X: 0, Y: 63, Z: 0}
	above   = geom.BlockPos{X: 0, Y: 64, Z: 0}
)

// placementProfile returns a 1.8.9 profile whose only phase is the placement
// one.
//
// A real profile rather than a fake, because the phase asks it what state a
// placement produces and a fake would answer whatever the test wanted — which
// is the difference between checking that the phase reads a placer and checking
// that a real placement reaches it.
func placementProfile(t *testing.T) sim.Profile {
	t.Helper()

	set, err := gen.Data()
	if err != nil {
		t.Fatalf("load the 1.8.9 data set: %v", err)
	}

	built, err := v1_8.New(set)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return placeOnly{Profile: built}
}

// placeOnly narrows a profile to the placement phase, so a movement rule cannot
// move the body a placement test never asked to move.
type placeOnly struct{ sim.Profile }

func (placeOnly) Phases() []sim.Phase { return []sim.Phase{placement.Phase()} }

// Ref forwards the name lookup the scene and the phase both need. An embedded
// interface does not carry the optional ones the concrete profile satisfies,
// and a silent loss of BlockNames leaves a world nobody can describe.
func (p placeOnly) Ref(name string) (world.BlockRef, bool) {
	names, ok := p.Profile.(sim.BlockNames)
	if !ok {
		return 0, false
	}

	return names.Ref(name)
}

// PlacedState forwards the placement rule, for the same reason Ref does.
func (p placeOnly) PlacedState(
	item data.ItemID, target placement.Target, face mining.Face, yaw float32, cursor geom.Vec3,
) (world.BlockRef, error) {
	return p.Profile.(placement.Placer).PlacedState(item, target, face, yaw, cursor)
}

// stage builds a kernel and a store with one stone block at the clicked cell,
// air above it, and the builder standing beside it.
func stage(t *testing.T, describe bool) (sim.Kernel, *runtime.Memory, sim.Profile) {
	t.Helper()

	profile := placementProfile(t)

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	store := runtime.NewMemory(profile)
	if describe {
		described := scene.World{
			Min:    geom.BlockPos{X: -1, Y: 62, Z: -1},
			Max:    geom.BlockPos{X: 1, Y: 65, Z: 1},
			Fill:   "air",
			Blocks: []scene.Block{{Pos: clicked, Name: "stone"}},
		}
		if err := described.Describe(profile, store.SetBlock); err != nil {
			t.Fatalf("describe the world: %v", err)
		}
	}

	store.SetEntity(builder, entity.State{
		Family: entity.FamilyPlayer,
		Box: geom.AABB{
			MinX: 1.7, MinY: 64, MinZ: -0.3,
			MaxX: 2.3, MaxY: 65.8, MaxZ: 0.3,
		},
		OnGround: true,
	})

	return kernel, store, profile
}

// step runs one tick with the given commands and applies the result.
func step(
	t *testing.T, kernel sim.Kernel, store *runtime.Memory,
	profile sim.Profile, commands ...sim.Command,
) sim.TickResult {
	t.Helper()

	result, err := kernel.Step(t.Context(), sim.TickInput{
		Profile:  profile,
		Revision: store.Revision(),
		Blocks:   store.Blocks(),
		Entities: store.Entities(),
		Scope:    sim.Scope{Entities: []entity.ID{builder}},
		Commands: commands,
	})
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if result.Completeness.Complete {
		if err := store.Apply(result.Changes); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}

	return result
}

// place returns a placement of the named item against the clicked cell's top.
func place(t *testing.T, item string) placement.Place {
	t.Helper()

	set, err := gen.Data()
	if err != nil {
		t.Fatalf("load the 1.8.9 data set: %v", err)
	}
	held, ok := set.Items().ByName(item)
	if !ok {
		t.Fatalf("this version has no item %q", item)
	}

	return placement.Place{
		Entity:  builder,
		Clicked: clicked,
		Face:    mining.FaceTop,
		Cursor:  geom.Vec3{X: 0.5, Y: 1, Z: 0.5},
		Held:    held.ID,
		Eye:     geom.Vec3{X: 2, Y: 65.62, Z: 0},
		Reach:   4.5,
	}
}

// blockOps returns the block operations in a change set.
func blockOps(t *testing.T, changes sim.ChangeSet) []sim.Op {
	t.Helper()

	var ops []sim.Op
	for _, op := range changes.Ops {
		if op.Kind == sim.OpSetBlock {
			ops = append(ops, op)
		}
	}

	return ops
}

// onlyOutcome returns the single command outcome a result carries.
func onlyOutcome(t *testing.T, result sim.TickResult) sim.CommandOutcome {
	t.Helper()

	if len(result.Outcomes) != 1 {
		t.Fatalf("got %d outcomes, want 1", len(result.Outcomes))
	}

	return result.Outcomes[0]
}

func TestAnAcceptedPlacementEmitsExactlyOneSetBlock(t *testing.T) {
	t.Parallel()

	kernel, store, profile := stage(t, true)
	result := step(t, kernel, store, profile, place(t, "stone"))

	if outcome := onlyOutcome(t, result); !outcome.Accepted {
		t.Fatalf("the placement was refused: %s", outcome.Reason)
	}

	ops := blockOps(t, result.Changes)
	if len(ops) != 1 {
		t.Fatalf("emitted %d block operations, want 1: %+v", len(ops), ops)
	}
	if ops[0].Block != above {
		t.Fatalf("placed at %v, want %v", ops[0].Block, above)
	}
}

func TestAnAcceptedPlacementSaysWhereItLanded(t *testing.T) {
	t.Parallel()

	kernel, store, profile := stage(t, true)
	result := step(t, kernel, store, profile, place(t, "stone"))

	var placed int
	for _, event := range result.Domain {
		if event.Kind == placement.EventPlaced {
			placed++
			if event.Block != above {
				t.Errorf("the event names %v, want %v", event.Block, above)
			}
		}
	}
	if placed != 1 {
		t.Fatalf("emitted %d placement events, want 1", placed)
	}
}

func TestARefusedPlacementEmitsNoChangeAndSaysWhy(t *testing.T) {
	t.Parallel()

	// The pair matters. A refusal that still emits a change writes a block the
	// server will not have; a refusal with no reason gives the caller nothing
	// to act on.
	kernel, store, profile := stage(t, true)

	// Fill the cell the placement would land in, so the refusal is the
	// occupancy one rather than any other.
	stone, ok := profile.(sim.BlockNames).Ref("stone")
	if !ok {
		t.Fatal("the profile does not know stone")
	}
	if err := store.SetBlock(above, stone); err != nil {
		t.Fatalf("SetBlock: %v", err)
	}

	result := step(t, kernel, store, profile, place(t, "stone"))

	if ops := blockOps(t, result.Changes); len(ops) != 0 {
		t.Fatalf("a refused placement emitted %d block changes", len(ops))
	}
	outcome := onlyOutcome(t, result)
	if outcome.Accepted {
		t.Fatal("the placement into an occupied cell was accepted")
	}
	if outcome.Reason != placement.ReasonOccupied {
		t.Fatalf("Reason = %q, want %q", outcome.Reason, placement.ReasonOccupied)
	}
}

func TestAPlacementAgainstAnUnknownCellIsRefusedRatherThanGuessed(t *testing.T) {
	t.Parallel()

	// A cell the caller has not described is not a cell that refuses a
	// placement, and it is not one that accepts it either. The outcome names
	// the cell so the caller knows what to load, and the tick reports what it
	// was missing rather than placing into a wall.
	kernel, store, profile := stage(t, false)
	result := step(t, kernel, store, profile, place(t, "stone"))

	if outcome := onlyOutcome(t, result); outcome.Accepted {
		t.Fatal("a placement against an undescribed cell was accepted")
	}
	if len(blockOps(t, result.Changes)) != 0 {
		t.Fatal("a placement against an undescribed cell emitted a block change")
	}
	if result.Completeness.Complete {
		t.Fatal("a tick that read an undescribed cell reported itself complete")
	}
}

func TestTwoPlacementsInOneTickApplyInOrder(t *testing.T) {
	t.Parallel()

	// Operations keep insertion order and a later one sees what an earlier one
	// wrote — which it does not, inside one tick: the tick reads the world it
	// was given. So the second placement into the same cell is accepted here
	// and the change set carries two writes to one cell, the last winning.
	// That is what the kernel's own contract says, and a phase that tried to
	// see its own writes would be holding state.
	kernel, store, profile := stage(t, true)
	result := step(t, kernel, store, profile, place(t, "stone"), place(t, "stone"))

	if len(result.Outcomes) != 2 {
		t.Fatalf("got %d outcomes, want 2", len(result.Outcomes))
	}
	ops := blockOps(t, result.Changes)
	if len(ops) != 2 {
		t.Fatalf("emitted %d block operations, want 2", len(ops))
	}
	for at, op := range ops {
		if op.Block != above {
			t.Fatalf("operation %d placed at %v, want %v", at, op.Block, above)
		}
	}
}

func TestAPlacementBeyondReachIsRefused(t *testing.T) {
	t.Parallel()

	kernel, store, profile := stage(t, true)
	command := place(t, "stone")
	command.Eye = geom.Vec3{X: 40, Y: 65, Z: 0}

	result := step(t, kernel, store, profile, command)

	outcome := onlyOutcome(t, result)
	if outcome.Accepted {
		t.Fatal("a placement forty blocks away was accepted")
	}
	if outcome.Reason != placement.ReasonOutOfReach {
		t.Fatalf("Reason = %q, want %q", outcome.Reason, placement.ReasonOutOfReach)
	}
}

func TestAnItemThatPlacesNoBlockIsRefusedWithItsReason(t *testing.T) {
	t.Parallel()

	kernel, store, profile := stage(t, true)
	result := step(t, kernel, store, profile, place(t, "diamond_pickaxe"))

	outcome := onlyOutcome(t, result)
	if outcome.Accepted {
		t.Fatal("placing a pickaxe was accepted")
	}
	if outcome.Reason == "" {
		t.Fatal("a refused placement gave no reason")
	}
}

func TestAPlacementThroughTheBuilderIsRefused(t *testing.T) {
	t.Parallel()

	// The builder is standing in the cell the block would go into. The check
	// runs against the placed block's own shape through the same boxes the
	// collision path builds, so this is the full-cube half of the rule the
	// bottom-slab case is the other half of.
	kernel, store, profile := stage(t, true)
	store.SetEntity(builder, entity.State{
		Family: entity.FamilyPlayer,
		Box: geom.AABB{
			MinX: -0.3, MinY: 64, MinZ: -0.3,
			MaxX: 0.3, MaxY: 65.8, MaxZ: 0.3,
		},
		OnGround: true,
	})

	result := step(t, kernel, store, profile, place(t, "stone"))

	outcome := onlyOutcome(t, result)
	if outcome.Accepted {
		t.Fatal("a placement into the builder's own box was accepted")
	}
	if outcome.Reason != placement.ReasonEntity {
		t.Fatalf("Reason = %q, want %q", outcome.Reason, placement.ReasonEntity)
	}
}
