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

// requireFamily skips a check the pinned dataset cannot answer yet, and says
// why rather than passing quietly.
//
// 26.1's item and arrow constants were transcribed for M9.2 and land in a
// minecraft-protocol release after v0.5.0. Until this module's dependency is
// bumped past it, the profile carries the player alone. A skip with this reason
// is distinguishable from a check nobody wrote, which is the whole discipline
// M9.1b's two-lane harness exists to enforce.
func requireFamily(t *testing.T, built sim.Profile, family entity.Family) sim.MotionConstants {
	t.Helper()

	constants := built.Motion(family)
	if constants == (sim.MotionConstants{}) {
		t.Skipf("the pinned dataset carries no %s motion constants for 26.1; "+
			"they are transcribed and land in the next minecraft-protocol release", family)
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
