package v26_1

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

// requireFamily returns a family's constants, and fails when the profile has
// none.
//
// It was a skip while 26.1's item and arrow constants were transcribed but
// unreleased, so that a lane waiting on a dependency stayed distinguishable
// from a check nobody wrote. They shipped in minecraft-protocol v0.6.0; a
// missing family is now a defect.
func requireFamily(t *testing.T, built sim.Profile, family entity.Family) sim.MotionConstants {
	t.Helper()

	constants := built.Motion(family)
	if constants == (sim.MotionConstants{}) {
		t.Fatalf("the profile carries no %s motion constants", family)
	}

	return constants
}

// TestTheItemAndArrowConstantsAre26sOwn pins the three numbers that differ from
// 1.8.9. Both gravities are double literals in this version and float literals
// in that one, and the item's vertical drag is a double where its horizontal
// drag in the same statement stayed a float.
func TestTheItemAndArrowConstantsAre26sOwn(t *testing.T) {
	built := built(t)

	item := requireFamily(t, built, entity.FamilyItem)
	if item.Gravity != 0.04 {
		t.Errorf("item Gravity = %v, want exactly 0.04; it is a double here and "+
			"float32(0.04) there", item.Gravity)
	}
	if want := float64(float32(0.98)); item.HorizontalDrag != want {
		t.Errorf("item HorizontalDrag = %v, want %v", item.HorizontalDrag, want)
	}
	if item.VerticalDrag != 0.98 {
		t.Errorf("item VerticalDrag = %v, want exactly 0.98; the two drags in "+
			"multiply(friction, 0.98, friction) are not the same number",
			item.VerticalDrag)
	}

	arrow := requireFamily(t, built, entity.FamilyArrow)
	if arrow.Gravity != 0.05 {
		t.Errorf("arrow Gravity = %v, want exactly 0.05", arrow.Gravity)
	}
	if want := float64(float32(0.99)); arrow.HorizontalDrag != want || arrow.VerticalDrag != want {
		t.Errorf("arrow drags = %v/%v, want %v", arrow.HorizontalDrag, arrow.VerticalDrag, want)
	}
}

// TestABodyFallsAtItsOwnFamilysRate26 is the 26.1 half of the defect: the
// phases named entity.FamilyPlayer outright here too.
func TestABodyFallsAtItsOwnFamilysRate26(t *testing.T) {
	profile := built(t)
	requireFamily(t, profile, entity.FamilyItem)

	fall := func(family entity.Family) float64 {
		t.Helper()

		state, _ := oneTick(t, profile, "stone", func(store *runtime.Memory) {
			body, _ := store.Entities().Entity(1)
			body.Family = family
			body.Box = body.Box.Offset(geom.Vec3{Y: 4})
			body.Position = geom.Vec3{X: body.Position.X, Y: body.Position.Y + 4, Z: body.Position.Z}
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
}

// TestAFamilyWithNoConstantsFailsTheTick26 needs no dataset entry: a family
// nobody supplied is exactly what it is about.
func TestAFamilyWithNoConstantsFailsTheTick26(t *testing.T) {
	profile := built(t)
	store := standingWorld(t, profile, geom.Vec3{X: 0.5, Y: 1, Z: 0.5}, "stone")

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

// TestTheItemsTwoDragsAreDifferentNumbers is the width difference this version
// introduced, checked where it shows: multiply(friction, 0.98, friction) takes
// a float on the horizontal axes and a double on the vertical one, so an item
// falling and sliding is dragged by two different numbers in one tick.
func TestTheItemsTwoDragsAreDifferentNumbers(t *testing.T) {
	rules := built(t)
	constants := requireFamily(t, rules, entity.FamilyItem)

	if constants.HorizontalDrag == constants.VerticalDrag {
		t.Fatalf("both drags are %v; 26.1 carries a float on one axis and a "+
			"double on the other", constants.HorizontalDrag)
	}

	own := rules.(*profile)
	shared := &scratch{bodies: []body{{
		id:      1,
		present: true,
		state: entity.State{
			Family: entity.FamilyItem,
			Motion: geom.Vec3{X: 1, Y: 1, Z: 1},
		},
	}}}

	if err := own.itemDrag(nil, shared); err != nil {
		t.Fatalf("itemDrag: %v", err)
	}

	got := shared.bodies[0].state.Motion
	if want := float64(float32(0.98)); got.X != want || got.Z != want {
		t.Errorf("horizontal = %v/%v, want the float %v", got.X, got.Z, want)
	}
	if got.Y != 0.98 {
		t.Errorf("vertical = %v, want the double 0.98", got.Y)
	}
}

// TestTheBounceNeedsDownwardMotion is the other rule 26.1 changed. 1.8.9 halves
// and inverts whatever vertical motion an item on the ground has; this version
// leaves an upward one alone.
func TestTheBounceNeedsDownwardMotion(t *testing.T) {
	own := built(t).(*profile)

	bounced := func(motion float64) float64 {
		t.Helper()

		shared := &scratch{bodies: []body{{
			id:      1,
			present: true,
			state: entity.State{
				Family:   entity.FamilyItem,
				OnGround: true,
				Motion:   geom.Vec3{Y: motion},
			},
		}}}
		if err := own.itemBounce(shared); err != nil {
			t.Fatalf("itemBounce: %v", err)
		}

		return shared.bodies[0].state.Motion.Y
	}

	if got := bounced(-0.3); got != 0.15 {
		t.Errorf("a falling item bounced to %v, want 0.15", got)
	}
	if got := bounced(0.3); got != 0.3 {
		t.Errorf("a rising item was changed to %v; this version tests the "+
			"direction before it bounces", got)
	}
}

// TestAnArrowsInertiaRunsBeforeItsGravity pins the order, which is the reverse
// of the item's. An arrow that fell before it decayed would carry a different
// number on every tick after the first.
func TestAnArrowsInertiaRunsBeforeItsGravity(t *testing.T) {
	rules := built(t)
	constants := requireFamily(t, rules, entity.FamilyArrow)

	own := rules.(*profile)
	shared := &scratch{bodies: []body{{
		id:      1,
		present: true,
		state: entity.State{
			Family: entity.FamilyArrow,
			Motion: geom.Vec3{X: 0.5},
		},
	}}}

	if err := own.arrowInertia(shared); err != nil {
		t.Fatalf("arrowInertia: %v", err)
	}
	if err := own.arrowGravity(shared); err != nil {
		t.Fatalf("arrowGravity: %v", err)
	}

	got := shared.bodies[0].state.Motion
	inertia := float64(float32(constants.HorizontalDrag))
	if want := 0.5 * inertia; got.X != want {
		t.Errorf("X = %v, want %v", got.X, want)
	}
	if want := -constants.Gravity; got.Y != want {
		t.Errorf("Y = %v, want %v: the gravity is subtracted after the inertia, "+
			"so a first tick from rest is undragged", got.Y, want)
	}
}
