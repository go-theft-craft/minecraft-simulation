package v1_8

import (
	"context"
	"errors"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// TestTheItemAndArrowConstantsAreTheDatasets pins what the two families fall
// at. The numbers are 1.8.9's own float literals widened, and they are written
// here as the widening rather than as decimals so a dataset that rounded one
// fails rather than passes.
func TestTheItemAndArrowConstantsAreTheDatasets(t *testing.T) {
	built := built(t)

	item := built.Motion(entity.FamilyItem)
	if want := float64(float32(0.04)); item.Gravity != want {
		t.Errorf("item Gravity = %v, want %v", item.Gravity, want)
	}
	if want := float64(float32(0.98)); item.HorizontalDrag != want || item.VerticalDrag != want {
		t.Errorf("item drags = %v/%v, want %v", item.HorizontalDrag, item.VerticalDrag, want)
	}

	arrow := built.Motion(entity.FamilyArrow)
	if want := float64(float32(0.05)); arrow.Gravity != want {
		t.Errorf("arrow Gravity = %v, want %v", arrow.Gravity, want)
	}
	if want := float64(float32(0.99)); arrow.HorizontalDrag != want || arrow.VerticalDrag != want {
		t.Errorf("arrow drags = %v/%v, want %v", arrow.HorizontalDrag, arrow.VerticalDrag, want)
	}

	// The player is the one this milestone did not change, and it is asserted
	// here so a dataset edit that moved every family shows up as this failing
	// too rather than as two families quietly agreeing with each other.
	if got := built.Motion(entity.FamilyPlayer).Gravity; got != 0.08 {
		t.Errorf("player Gravity = %v, want 0.08", got)
	}
}

// TestABodyFallsAtItsOwnFamilysRate is the defect M9.2 found. Every phase read
// entity.FamilyPlayer outright, so a dropped item fell at 0.08 a tick — the
// player's gravity — and nothing in the module said otherwise.
func TestABodyFallsAtItsOwnFamilysRate(t *testing.T) {
	profile := built(t)

	fall := func(family entity.Family) float64 {
		t.Helper()

		state := oneTick(t, profile, func(store *runtime.Memory) {
			body, _ := store.Entities().Entity(1)
			body.Family = family
			// Airborne, so the floor cannot clamp the motion before it is
			// visible, and level so nothing else is acting on it.
			body.Box = body.Box.Offset(geom.Vec3{Y: 4})
			body.OnGround = false
			store.SetEntity(1, body)
		}, movement.Input{Entity: 1})

		return state.Motion.Y
	}

	player, item, arrow := fall(entity.FamilyPlayer), fall(entity.FamilyItem), fall(entity.FamilyArrow)
	if player == item || player == arrow || item == arrow {
		t.Fatalf("three families fell at %v, %v, %v; each has its own gravity and drag",
			player, item, arrow)
	}

	// And the numbers are the families' own, computed from each family's own
	// order rather than from one shared formula. A player and an item both
	// apply gravity and then their vertical drag, on opposite sides of the
	// move; an arrow applies its inertia first and its gravity after, so one
	// tick from rest is the gravity undragged.
	for _, tc := range []struct {
		name   string
		family entity.Family
		got    float64
		want   func(sim.MotionConstants) float64
	}{
		{"player", entity.FamilyPlayer, player, draggedFall},
		{"item", entity.FamilyItem, item, draggedFall},
		{"arrow", entity.FamilyArrow, arrow, func(c sim.MotionConstants) float64 { return -c.Gravity }},
	} {
		if want := tc.want(profile.Motion(tc.family)); tc.got != want {
			t.Errorf("%s fell to %v in one tick, want %v", tc.name, tc.got, want)
		}
	}
}

// draggedFall is one tick of falling from rest for a family that applies its
// gravity before its vertical drag.
func draggedFall(constants sim.MotionConstants) float64 {
	return -constants.Gravity * float64(float32(constants.VerticalDrag))
}

// TestAFamilyWithNoConstantsFailsTheTick is the other half. A body nobody gave
// a family to must not fall at zero gravity and drift forever: that reads as a
// physics bug in whatever consumes the body and as nothing at all here.
func TestAFamilyWithNoConstantsFailsTheTick(t *testing.T) {
	profile := built(t)
	store := standingWorld(t, profile, geom.Vec3{X: 0.5, Y: 1, Z: 0.5})

	body, _ := store.Entities().Entity(1)
	body.Family = entity.FamilyUnknown
	store.SetEntity(1, body)

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{1}})

	_, err = runner.Step(context.Background(), []sim.Command{movement.Input{Entity: 1}})
	if !errors.Is(err, sim.ErrUnknownFamily) {
		t.Fatalf("Step = %v, want ErrUnknownFamily", err)
	}
}

// ticks runs one body of a family for several ticks over a stone floor and
// returns the state after each, so a test can assert a whole trajectory rather
// than one step of it.
func ticks(t *testing.T, family entity.Family, from geom.Vec3, motion geom.Vec3, count int) []entity.State {
	t.Helper()

	profile := built(t)
	store := standingWorld(t, profile, geom.Vec3{X: 0.5, Y: 1, Z: 0.5})

	body, _ := store.Entities().Entity(1)
	body.Family = family
	// A quarter-block box, which is what both games give an item and an arrow.
	body.Box = movement.Box(from, 0.25, 0.25)
	body.Motion = motion
	body.OnGround = false
	body.StepHeight = 0
	store.SetEntity(1, body)

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{1}})

	states := make([]entity.State, 0, count)
	for tick := range count {
		result, err := runner.Step(context.Background(), nil)
		if err != nil {
			t.Fatalf("Step %d: %v", tick, err)
		}
		if !result.Completeness.Complete {
			t.Fatalf("tick %d was incomplete: %+v", tick, result.Completeness.Missing)
		}

		state, ok := store.Entities().Entity(1)
		if !ok {
			t.Fatalf("the body is gone after tick %d", tick)
		}
		states = append(states, state)
	}

	return states
}

// TestADroppedItemFallsLandsAndBounces walks the whole trajectory, because the
// constants alone do not settle it: the order gravity, the move, and the drags
// run in is as much of the rule as the numbers, and only a trajectory shows it.
func TestADroppedItemFallsLandsAndBounces(t *testing.T) {
	states := ticks(t, entity.FamilyItem, geom.Vec3{X: 0.5, Y: 3, Z: 0.5}, geom.Vec3{}, 40)

	// The first tick: gravity, then the move, then the drag. An item that
	// applied its gravity after the move like a player would be one tick behind
	// here forever.
	constants := built(t).Motion(entity.FamilyItem)
	if want := draggedFall(constants); states[0].Motion.Y != want {
		t.Fatalf("after one tick the item's motion is %v, want %v", states[0].Motion.Y, want)
	}
	if states[0].Box.MinY >= 3 {
		t.Fatalf("the item did not move on its first tick: %v", states[0].Box.MinY)
	}

	// It reaches the floor.
	landed := -1
	for index, state := range states {
		if state.OnGround {
			landed = index

			break
		}
	}
	if landed < 0 {
		t.Fatal("the item never reached the floor")
	}

	// And it settles: by the end it is on the floor and has stopped.
	last := states[len(states)-1]
	if !last.OnGround {
		t.Fatalf("the item is not resting after %d ticks: %+v", len(states), last)
	}
	if last.Motion.Y > 0 {
		t.Fatalf("the item is still bouncing after %d ticks: %v", len(states), last.Motion.Y)
	}
	if got := last.Box.MinY; got != 1 {
		t.Fatalf("the item came to rest at %v, want the floor at 1", got)
	}
}

// TestAnArrowDecaysAndSticks is the arrow's shape: no bounce, no friction from
// the block below, one multiplier on every axis, and a stop.
func TestAnArrowDecaysAndSticks(t *testing.T) {
	// Slow enough to stay inside the described room: a body that reached a cell
	// nobody described would fail the tick as incomplete, which is a fact about
	// the test's world rather than about the arrow.
	const launch = 0.15

	states := ticks(t, entity.FamilyArrow,
		geom.Vec3{X: 0.5, Y: 4, Z: 0.5}, geom.Vec3{X: launch}, 20)

	constants := built(t).Motion(entity.FamilyArrow)
	inertia := float64(float32(constants.HorizontalDrag))

	// The first tick, exactly: the arrow moves at its launch speed, then the
	// inertia decays it, then gravity is subtracted. A family that applied
	// gravity before the inertia would report a different number here.
	if want := launch * inertia; states[0].Motion.X != want {
		t.Fatalf("after one tick the arrow's X motion is %v, want %v", states[0].Motion.X, want)
	}
	if want := -constants.Gravity; states[0].Motion.Y != want {
		t.Fatalf("after one tick the arrow's Y motion is %v, want %v", states[0].Motion.Y, want)
	}

	// It keeps decaying, and it is never dragged by the block it flies over.
	if want := launch * inertia * inertia; states[1].Motion.X != want {
		t.Fatalf("after two ticks the arrow's X motion is %v, want %v", states[1].Motion.X, want)
	}

	// It lands and stays there, with no bounce.
	last := states[len(states)-1]
	if !last.OnGround {
		t.Fatalf("the arrow is not on the floor after %d ticks: %+v", len(states), last)
	}
	if last.Motion.Y > 0 {
		t.Fatalf("the arrow bounced: %v", last.Motion.Y)
	}
}

// TestTheItemBounceInvertsAndHalves reaches the phase directly, because a
// falling item on flat ground never exercises it: the move zeroes the vertical
// motion the moment the floor stops it, and a zero halved and inverted is
// still zero. The rule matters where the move does not collide — an item that
// is on the ground and still carrying downward motion — so that is what this
// hands it.
func TestTheItemBounceInvertsAndHalves(t *testing.T) {
	built := built(t).(*profile)

	shared := &scratch{bodies: []body{{
		id:      1,
		present: true,
		state: entity.State{
			Family:   entity.FamilyItem,
			OnGround: true,
			Motion:   geom.Vec3{X: 0.2, Y: -0.3, Z: 0.1},
		},
	}}}

	if err := built.itemBounce(shared); err != nil {
		t.Fatalf("itemBounce: %v", err)
	}

	got := shared.bodies[0].state.Motion
	if got.Y != 0.15 {
		t.Errorf("Y = %v, want 0.15: half the motion, inverted", got.Y)
	}
	if got.X != 0.2 || got.Z != 0.1 {
		t.Errorf("horizontal motion = %v/%v, want it untouched", got.X, got.Z)
	}

	// An airborne item keeps everything: the rule is about contact.
	shared.bodies[0].state.OnGround = false
	shared.bodies[0].state.Motion = geom.Vec3{Y: -0.3}
	if err := built.itemBounce(shared); err != nil {
		t.Fatalf("itemBounce: %v", err)
	}
	if got := shared.bodies[0].state.Motion.Y; got != -0.3 {
		t.Errorf("an airborne item's Y = %v, want -0.3", got)
	}
}
