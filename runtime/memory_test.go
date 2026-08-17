package runtime

import (
	"errors"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// stone is the one block handle these tests use, and dirt is a handle the
// profile does not know, so that a change set can name something unresolvable.
const (
	stone world.BlockRef = 1
	dirt  world.BlockRef = 2
)

func testProfile(phases ...sim.Phase) *sim.StaticProfile {
	return &sim.StaticProfile{
		Identity:  sim.ProfileID{Edition: "java", GameVersion: "1.8.9", RulesRevision: "1"},
		PhaseList: phases,
		Shapes:    map[world.BlockRef]geom.Shape{stone: geom.FullCube()},
	}
}

func player() entity.State {
	return entity.State{
		Family:     entity.FamilyPlayer,
		Box:        geom.AABB{MinX: -0.3, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3},
		StepHeight: float64(float32(0.6)),
	}
}

func TestAFreshStoreIsAtRevisionZero(t *testing.T) {
	if got := NewMemory(testProfile()).Revision(); got != 0 {
		t.Fatalf("Revision = %d, want 0", got)
	}
}

func TestApplyAdvancesTheRevision(t *testing.T) {
	store := NewMemory(testProfile())

	// An empty set still applies: a tick that decided nothing is a tick that
	// happened, and the revision counts applied sets rather than changes.
	if err := store.Apply(sim.ChangeSet{BaseRevision: 0}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := store.Revision(); got != 1 {
		t.Fatalf("Revision = %d, want 1", got)
	}
}

func TestApplyRefusesAStaleOrFutureRevision(t *testing.T) {
	store := NewMemory(testProfile())
	if err := store.Apply(sim.ChangeSet{BaseRevision: 0}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	stale := sim.ChangeSet{BaseRevision: 0, Ops: []sim.Op{
		{Kind: sim.OpSetEntity, Entity: 1, State: player()},
	}}
	if err := store.Apply(stale); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("Apply error = %v, want ErrStaleRevision", err)
	}
	if _, ok := store.Entities().Entity(1); ok {
		t.Error("a refused change set wrote a body anyway")
	}
	if got := store.Revision(); got != 1 {
		t.Fatalf("a refused change set advanced the revision to %d", got)
	}

	future := sim.ChangeSet{BaseRevision: 9}
	if err := store.Apply(future); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("Apply error = %v, want ErrStaleRevision for a future revision", err)
	}
}

func TestApplyWritesEntitiesAndBlocks(t *testing.T) {
	store := NewMemory(testProfile())
	pos := geom.BlockPos{X: 1, Y: 2, Z: 3}

	changes := sim.ChangeSet{Ops: []sim.Op{
		{Kind: sim.OpSetEntity, Entity: 1, State: player()},
		{Kind: sim.OpSetBlock, Block: pos, Ref: stone},
	}}
	if err := store.Apply(changes); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, ok := store.Entities().Entity(1)
	if !ok || got != player() {
		t.Fatalf("Entity = (%+v, %v), want the body the set wrote", got, ok)
	}

	shape, lookup := store.Blocks().CollisionShape(pos)
	if lookup != world.LookupShape || shape.Len() != 1 {
		t.Fatalf("CollisionShape = (%d boxes, %v), want the resolved shape", shape.Len(), lookup)
	}
	ref, lookup := store.Blocks().BlockState(pos)
	if lookup != world.LookupShape || ref != stone {
		t.Fatalf("BlockState = (%d, %v), want (%d, shape)", ref, lookup, stone)
	}
}

func TestApplyRemovesAnEntity(t *testing.T) {
	store := NewMemory(testProfile())
	store.SetEntity(1, player())

	changes := sim.ChangeSet{Ops: []sim.Op{{Kind: sim.OpRemoveEntity, Entity: 1}}}
	if err := store.Apply(changes); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, ok := store.Entities().Entity(1); ok {
		t.Fatal("the body survived a remove operation")
	}
}

func TestApplyRefusesAnUnresolvableHandleWhole(t *testing.T) {
	store := NewMemory(testProfile())
	first := geom.BlockPos{X: 1}

	// The valid operation comes first, so that a store which wrote as it walked
	// would leave evidence.
	changes := sim.ChangeSet{Ops: []sim.Op{
		{Kind: sim.OpSetBlock, Block: first, Ref: stone},
		{Kind: sim.OpSetBlock, Block: geom.BlockPos{X: 2}, Ref: dirt},
	}}

	if err := store.Apply(changes); !errors.Is(err, ErrUnknownBlock) {
		t.Fatalf("Apply error = %v, want ErrUnknownBlock", err)
	}
	if _, lookup := store.Blocks().BlockState(first); lookup != world.LookupUnknown {
		t.Fatalf("the earlier operation was written anyway: lookup = %v", lookup)
	}
	if got := store.Revision(); got != 0 {
		t.Fatalf("a refused change set advanced the revision to %d", got)
	}
}

func TestSetBlockRefusesAnUnknownHandle(t *testing.T) {
	store := NewMemory(testProfile())
	if err := store.SetBlock(geom.BlockPos{}, dirt); !errors.Is(err, ErrUnknownBlock) {
		t.Fatalf("SetBlock error = %v, want ErrUnknownBlock", err)
	}
}

func TestSnapshotDoesNotFollowItsOrigin(t *testing.T) {
	store := NewMemory(testProfile())
	pos := geom.BlockPos{X: 4}
	store.SetEntity(1, player())
	if err := store.SetBlock(pos, stone); err != nil {
		t.Fatalf("SetBlock: %v", err)
	}

	fork := store.Snapshot()
	if fork.Revision() != store.Revision() {
		t.Fatalf("the fork is at revision %d, want %d", fork.Revision(), store.Revision())
	}

	// Advance the origin, then check the fork kept what it forked.
	changes := sim.ChangeSet{Ops: []sim.Op{
		{Kind: sim.OpRemoveEntity, Entity: 1},
		{Kind: sim.OpSetBlock, Block: pos, Ref: stone},
	}}
	if err := store.Apply(changes); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if _, ok := fork.Entities().Entity(1); !ok {
		t.Error("the fork lost a body its origin removed afterwards")
	}
	if fork.Revision() != 0 {
		t.Fatalf("the fork followed its origin to revision %d", fork.Revision())
	}

	// And the reverse: work applied to the fork must not reach the origin.
	forkChanges := sim.ChangeSet{Ops: []sim.Op{{Kind: sim.OpSetEntity, Entity: 2, State: player()}}}
	if err := fork.Apply(forkChanges); err != nil {
		t.Fatalf("Apply to the fork: %v", err)
	}
	if _, ok := store.Entities().Entity(2); ok {
		t.Fatal("a body written to the fork reached the origin")
	}
}

var _ Store = (*Memory)(nil)

func TestApplyWritesLocomotion(t *testing.T) {
	store := NewMemory(testProfile())
	want := movement.Locomotion{JumpTicks: 10, Yaw: 90, MoveSpeed: 0.1, JumpFactor: 0.02}

	changes := sim.ChangeSet{Ops: []sim.Op{
		{Kind: sim.OpSetLocomotion, Entity: 1, Locomotion: want},
	}}
	if err := store.Apply(changes); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, ok := store.Locomotion().Locomotion(1)
	if !ok || got != want {
		t.Fatalf("Locomotion = (%+v, %v), want %+v", got, ok, want)
	}
}

func TestRemovingABodyRemovesItsLocomotion(t *testing.T) {
	// The two are one body. Leaving movement state behind for a removed entity
	// would let a later spawn inherit a dead body's jump counter.
	store := NewMemory(testProfile())
	store.SetEntity(1, player())
	store.SetLocomotion(1, movement.Locomotion{JumpTicks: 4})

	changes := sim.ChangeSet{Ops: []sim.Op{{Kind: sim.OpRemoveEntity, Entity: 1}}}
	if err := store.Apply(changes); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, ok := store.Locomotion().Locomotion(1); ok {
		t.Fatal("the locomotion state survived the body being removed")
	}
}

func TestSnapshotForksLocomotionToo(t *testing.T) {
	store := NewMemory(testProfile())
	original := movement.Locomotion{JumpTicks: 7, Yaw: 45}
	store.SetLocomotion(1, original)

	fork := store.Snapshot()
	changes := sim.ChangeSet{Ops: []sim.Op{
		{Kind: sim.OpSetLocomotion, Entity: 1, Locomotion: movement.Locomotion{}},
	}}
	if err := store.Apply(changes); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got, _ := fork.Locomotion().Locomotion(1); got != original {
		t.Fatalf("the fork followed its origin: %+v", got)
	}
}
