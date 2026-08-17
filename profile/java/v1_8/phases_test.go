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

// oneTick runs a single tick of a standing player and returns the body it left.
func oneTick(t *testing.T, profile sim.Profile, prepare func(*runtime.Memory), input movement.Input) entity.State {
	t.Helper()

	store := standingWorld(t, profile, geom.Vec3{X: 0.5, Y: 1, Z: 0.5})
	if prepare != nil {
		prepare(store)
	}

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{1}})

	result, err := runner.Step(context.Background(), []sim.Command{input})
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if !result.Completeness.Complete {
		t.Fatalf("the tick was incomplete: %+v", result.Completeness.Missing)
	}

	state, ok := store.Entities().Entity(1)
	if !ok {
		t.Fatal("the body is gone")
	}

	return state
}

func TestTheSneakScaleIsTheClientsDoubleProduct(t *testing.T) {
	profile := built(t)

	// A keyboard only ever produces axes of -1, 0, or 1, and for those the two
	// widths agree exactly — which is why nothing measurable separates them and
	// why the oracle cannot reach this rule. This axis was searched for because
	// it does separate them, so a consumer feeding fractional intent gets the
	// game's answer rather than a plausible neighbour.
	const axis float32 = 0.4613862

	sneaking := oneTick(t, profile, nil, movement.Input{Entity: 1, Strafe: axis, Sneak: true})
	asDouble := oneTick(t, profile, nil, movement.Input{
		Entity: 1, Strafe: float32(float64(axis) * sneakScale),
	})
	asFloat := oneTick(t, profile, nil, movement.Input{
		Entity: 1, Strafe: axis * float32(sneakScale),
	})

	// Everything else about the three ticks is identical, so the only thing the
	// comparison can be about is the scaling.
	if sneaking.Motion != asDouble.Motion {
		t.Fatalf("sneaking left the motion at %+v, want the double-scaled %+v",
			sneaking.Motion, asDouble.Motion)
	}
	if asDouble.Motion == asFloat.Motion {
		t.Fatal("the two widths agree for this axis; the test proves nothing as written")
	}
}

func TestTheSpawnedBoxIsTheBoxTheGameBuilds(t *testing.T) {
	// The game halves a float width and adds a float height to a double
	// position, so a vanilla player's box is not the round 0.6 by 1.8 it reads
	// as. The oracle caught this before a single rule had run: the spawned body
	// disagreed with the game's on the tick it was created.
	state, _, ok := Spawn(built(t), geom.Vec3{X: 0.5, Y: 1, Z: 0.5}, 0, 0)
	if !ok {
		t.Fatal("Spawn did not recognize its own profile")
	}

	if want := 0.5 - float64(float32(0.6)/2); state.Box.MinX != want {
		t.Errorf("MinX = %v, want %v", state.Box.MinX, want)
	}
	if state.Box.MinX == 0.2 {
		t.Error("MinX is the round decimal; the game's half width is wider than 0.3")
	}
	if want := 1 + float64(float32(1.8)); state.Box.MaxY != want {
		t.Errorf("MaxY = %v, want %v", state.Box.MaxY, want)
	}
	if state.Box.MaxY == 2.8 {
		t.Error("MaxY is the round decimal; the game's height is shorter than 1.8")
	}
}

func TestAMotionTooSmallToMatterIsDiscardedBeforeTheBodyMoves(t *testing.T) {
	profile := built(t)

	// Half the threshold, which the game throws away at the top of the tick. A
	// tick without that rule would carry it into the move and shift the body.
	const creep = 0.0024

	moved := oneTick(t, profile, func(store *runtime.Memory) {
		state, _ := store.Entities().Entity(1)
		state.Motion = geom.Vec3{X: creep, Z: -creep}
		store.SetEntity(1, state)
	}, movement.Input{Entity: 1})

	if moved.Box.MinX != 0.5-playerHalfWidth || moved.Box.MinZ != 0.5-playerHalfWidth {
		t.Fatalf("a body carrying less than the threshold moved to %+v", moved.Box)
	}

	// And a motion above the threshold does move the body, so the assertion above
	// is about the threshold rather than about the body being stuck.
	kept := oneTick(t, profile, func(store *runtime.Memory) {
		state, _ := store.Entities().Entity(1)
		state.Motion = geom.Vec3{X: 0.02}
		store.SetEntity(1, state)
	}, movement.Input{Entity: 1})

	if kept.Box.MinX <= 0.5-playerHalfWidth {
		t.Fatalf("a body carrying more than the threshold did not move: %+v", kept.Box)
	}
}
