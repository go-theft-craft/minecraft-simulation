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

	// And the numbers are the families' own, not merely different from one
	// another: one tick of falling from rest is the gravity, dragged.
	for _, tc := range []struct {
		name   string
		family entity.Family
		got    float64
	}{
		{"player", entity.FamilyPlayer, player},
		{"item", entity.FamilyItem, item},
		{"arrow", entity.FamilyArrow, arrow},
	} {
		constants := profile.Motion(tc.family)
		want := -constants.Gravity * float64(float32(constants.VerticalDrag))
		if tc.got != want {
			t.Errorf("%s fell to %v in one tick, want %v", tc.name, tc.got, want)
		}
	}
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
