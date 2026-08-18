package v1_8

import (
	"context"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

func built(t *testing.T) sim.Profile {
	t.Helper()

	profile, err := New(dataset(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return profile
}

func TestTheProfileNamesItself(t *testing.T) {
	if got, want := built(t).ID().String(), "java/1.8.9@1"; got != want {
		t.Fatalf("ID = %q, want %q", got, want)
	}
}

func TestThePhaseListIsTheTickInOrder(t *testing.T) {
	// Asserted as a literal slice, because reordering these is exactly the kind
	// of change that breaks a trajectory without breaking anything else.
	want := []string{
		"v1_8.jump-countdown",
		"v1_8.motion-threshold",
		"v1_8.jump",
		"v1_8.input-decay",
		"v1_8.friction",
		"v1_8.acceleration",
		"v1_8.apply-input",
		"v1_8.item-gravity",
		"v1_8.move",
		"v1_8.gravity",
		"v1_8.vertical-drag",
		"v1_8.horizontal-drag",
		"v1_8.item-drag",
		"v1_8.item-bounce",
		"v1_8.arrow-stick",
		"v1_8.arrow-inertia",
		"v1_8.arrow-gravity",
		"v1_8.commit",
	}

	phases := built(t).Phases()
	if len(phases) != len(want) {
		t.Fatalf("the profile declares %d phases, want %d", len(phases), len(want))
	}
	for index, id := range want {
		if got := phases[index].ID(); got != id {
			t.Fatalf("phase %d is %q, want %q", index, got, id)
		}
	}
}

func TestTwoKernelsFromOneProfileDoNotShareScratch(t *testing.T) {
	// The phases of one list share per-tick working state, so Phases hands out a
	// fresh list per kernel. The property that matters is not that the lists are
	// distinct objects but that interleaving two kernels changes nothing: each
	// must produce the digests it would have produced alone.
	profile := built(t)

	series := func(kernels int) [][]sim.Digest {
		runners := make([]*runtime.Runner, kernels)
		for index := range runners {
			store := standingWorld(t, profile, geom.Vec3{X: 0.5, Y: 4, Z: 0.5})
			kernel, err := sim.NewKernel(profile)
			if err != nil {
				t.Fatalf("NewKernel: %v", err)
			}
			runners[index] = runtime.NewRunner(store, kernel)
			runners[index].SetScope(sim.Scope{Entities: []entity.ID{1}})
		}

		digests := make([][]sim.Digest, kernels)
		for range 12 {
			// Interleaved: every kernel advances one tick before any advances two.
			for index, runner := range runners {
				result, err := runner.Step(context.Background(), []sim.Command{
					movement.Input{Entity: 1, Forward: 1, Yaw: 45},
				})
				if err != nil {
					t.Fatalf("Step: %v", err)
				}
				digests[index] = append(digests[index], result.Digest)
			}
		}

		return digests
	}

	alone := series(1)[0]
	for index, interleaved := range series(3) {
		if len(interleaved) != len(alone) {
			t.Fatalf("kernel %d ran %d ticks, want %d", index, len(interleaved), len(alone))
		}
		for tick := range alone {
			if interleaved[tick] != alone[tick] {
				t.Fatalf("kernel %d diverged at tick %d: %s, want %s",
					index, tick, interleaved[tick], alone[tick])
			}
		}
	}
}

func TestTheMotionConstantsComeFromTheDataset(t *testing.T) {
	got := built(t).Motion(entity.FamilyPlayer)

	// StepHeight is the constant M8.2's oracle caught: the game holds it as a
	// float and widens it where the step-up applies it. A test that accepted the
	// round 0.6 would let that regress silently.
	if want := float64(float32(0.6)); got.StepHeight != want {
		t.Errorf("StepHeight = %v, want %v", got.StepHeight, want)
	}
	if got.StepHeight == 0.6 {
		t.Error("StepHeight is the round decimal; the widened float is a different number")
	}
	if got.Gravity != 0.08 {
		t.Errorf("Gravity = %v, want exactly 0.08: it is a double literal in the game", got.Gravity)
	}
	if want := float64(float32(0.91)); got.HorizontalDrag != want {
		t.Errorf("HorizontalDrag = %v, want %v", got.HorizontalDrag, want)
	}
	if want := float64(float32(0.98)); got.VerticalDrag != want {
		t.Errorf("VerticalDrag = %v, want %v", got.VerticalDrag, want)
	}
}

func TestAnUnknownFamilyHasNoConstants(t *testing.T) {
	// A body whose family nobody set is an error for the caller to notice, not a
	// body that falls at a guessed rate.
	if got := built(t).Motion(entity.FamilyUnknown); got != (sim.MotionConstants{}) {
		t.Fatalf("Motion(unknown) = %+v, want the zero constants", got)
	}
}

func TestSlipperinessOfAnUnknownHandleIsTheDefault(t *testing.T) {
	// The default at the width the game holds it: the dataset stores the round
	// decimal and the game's field is a float, so this is float32(0.6) widened.
	if got, want := built(t).Slipperiness(0), float64(float32(0.6)); got != want {
		t.Fatalf("Slipperiness(0) = %v, want the default %v", got, want)
	}
}

// standingWorld returns a store holding a stone floor at y = 0 with air above,
// and a player standing on it at the given position.
func standingWorld(t *testing.T, profile sim.Profile, pos geom.Vec3) *runtime.Memory {
	t.Helper()

	stone, ok := Ref(profile, "stone")
	if !ok {
		t.Fatal("the profile does not know stone")
	}
	air, ok := Ref(profile, "air")
	if !ok {
		t.Fatal("the profile does not know air")
	}

	store := runtime.NewMemory(profile)
	for x := int32(-4); x <= 4; x++ {
		for z := int32(-4); z <= 4; z++ {
			if err := store.SetBlock(geom.BlockPos{X: x, Y: 0, Z: z}, stone); err != nil {
				t.Fatalf("SetBlock: %v", err)
			}
			for y := int32(1); y <= 6; y++ {
				if err := store.SetBlock(geom.BlockPos{X: x, Y: y, Z: z}, air); err != nil {
					t.Fatalf("SetBlock: %v", err)
				}
			}
		}
	}

	state, loco, ok := Spawn(profile, pos, 0, 0)
	if !ok {
		t.Fatal("Spawn did not recognize its own profile")
	}
	store.SetEntity(1, state)
	store.SetLocomotion(1, loco)

	return store
}

func TestAStandingPlayerStaysOnTheFloor(t *testing.T) {
	profile := built(t)
	store := standingWorld(t, profile, geom.Vec3{X: 0.5, Y: 1, Z: 0.5})

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{1}})

	// Several ticks, because standing still is not a static state in this game.
	// Gravity is applied after the move, so a body handed zero motion is airborne
	// for its first tick and is clamped by the floor on every tick after it. That
	// is what a standing vanilla player does: it falls a sixteenth of a block and
	// is stopped, twenty times a second.
	var last entity.State
	for tick := range 5 {
		result, err := runner.Step(context.Background(), []sim.Command{
			movement.Input{Entity: 1, Yaw: 0},
		})
		if err != nil {
			t.Fatalf("tick %d: Step: %v", tick, err)
		}
		if !result.Completeness.Complete {
			t.Fatalf("tick %d was incomplete: %+v", tick, result.Completeness.Missing)
		}

		state, ok := store.Entities().Entity(1)
		if !ok {
			t.Fatalf("the body is gone after tick %d", tick)
		}
		if state.Box.MinY != 1 {
			t.Fatalf("tick %d left the feet at %v, want 1: the body sank into the floor",
				tick, state.Box.MinY)
		}
		if state.Box.MinX != 0.5-playerHalfWidth || state.Box.MaxX != 0.5+playerHalfWidth {
			t.Fatalf("tick %d drifted horizontally: %+v", tick, state.Box)
		}
		last = state
	}

	if !last.OnGround {
		t.Error("a player standing on stone ended up reporting itself airborne")
	}
	if last.Motion.Y >= 0 {
		t.Errorf("the settled vertical motion is %v, want the tick's fall waiting to be clamped",
			last.Motion.Y)
	}
}

func TestATickOverAnUndescribedRegionIsIncompleteAndChangesNothing(t *testing.T) {
	profile := built(t)
	store := runtime.NewMemory(profile)

	state, loco, ok := Spawn(profile, geom.Vec3{X: 0.5, Y: 1, Z: 0.5}, 0, 0)
	if !ok {
		t.Fatal("Spawn did not recognize its own profile")
	}
	store.SetEntity(1, state)
	store.SetLocomotion(1, loco)

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{1}})

	result, err := runner.Step(context.Background(), nil)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if result.Completeness.Complete {
		t.Fatal("a tick over a world nobody described reported itself complete")
	}
	if store.Revision() != 0 {
		t.Fatalf("Revision = %d after an incomplete tick, want 0", store.Revision())
	}
	if got, _ := store.Entities().Entity(1); got != state {
		t.Fatalf("the body moved on an incomplete tick: %+v", got)
	}
}

func TestABodyOutOfScopeIsNotSimulated(t *testing.T) {
	profile := built(t)
	store := standingWorld(t, profile, geom.Vec3{X: 0.5, Y: 4, Z: 0.5})
	before, _ := store.Entities().Entity(1)

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	runner := runtime.NewRunner(store, kernel)
	// No scope: an empty scope means no body, not every body.

	if _, err := runner.Step(context.Background(), nil); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if got, _ := store.Entities().Entity(1); got != before {
		t.Fatalf("a body outside the scope was simulated: %+v", got)
	}
}
