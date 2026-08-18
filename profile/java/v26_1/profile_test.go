package v26_1

import (
	"context"
	"slices"
	"testing"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
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
	if got, want := built(t).ID().String(), "java/26.1.2@1"; got != want {
		t.Errorf("the profile calls itself %q, want %q", got, want)
	}
}

func TestNewRejectsADataSetItCannotBuildFrom(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New accepted a nil data set")
	}

	// A set with blocks and no physics has no slipperiness, no motion
	// constants, and no trigonometry table. A profile built from it would
	// answer plausible numbers for all three.
	set, err := data.NewSet(data.SetOptions{
		Blocks:          fakeBlocks{{Name: "stone", MinStateID: 0, MaxStateID: 0}},
		CollisionShapes: cubeShapes("stone"),
	})
	if err != nil {
		t.Fatalf("build a data set: %v", err)
	}
	if _, err := New(set); err == nil {
		t.Error("New accepted a data set with no physics")
	}
}

func TestThePhaseListIsThisVersionsOrder(t *testing.T) {
	// Asserted as a literal, because reordering is exactly the kind of change
	// that breaks trajectories without breaking anything that looks like a test.
	//
	// Two of these have no counterpart in the 1.8.9 list: the input shaping,
	// which that version's client does differently and its tick does not do at
	// all, and the block speed factor, which that version applies from inside a
	// block's own collision callback.
	want := []string{
		"v26_1.jump-countdown",
		"v26_1.motion-threshold",
		"v26_1.input-shaping",
		"v26_1.jump",
		"v26_1.friction",
		"v26_1.acceleration",
		"v26_1.apply-input",
		"v26_1.item-gravity",
		"v26_1.move",
		"v26_1.block-speed-factor",
		"v26_1.gravity",
		"v26_1.vertical-drag",
		"v26_1.horizontal-drag",
		"v26_1.item-drag",
		"v26_1.item-bounce",
		"v26_1.arrow-inertia",
		"v26_1.arrow-gravity",
		"v26_1.commit",
	}

	var got []string
	for _, phase := range built(t).Phases() {
		got = append(got, phase.ID())
	}
	if !slices.Equal(got, want) {
		t.Errorf("the phase list is\n%v\nwant\n%v", got, want)
	}
}

func TestEachPhaseListHasItsOwnScratch(t *testing.T) {
	// Two kernels built from one profile must be steppable independently, and
	// the scratch is the only thing they could share.
	profile := built(t)

	first := profile.Phases()
	second := profile.Phases()
	if len(first) != len(second) {
		t.Fatal("two phase lists of different lengths")
	}
	if &first[0] == &second[0] {
		t.Fatal("the two lists are the same slice")
	}
}

func TestThePlayerConstantsAreTheDatasets(t *testing.T) {
	constants := built(t).Motion(entity.FamilyPlayer)

	// The step height is the value M8.2's oracle caught in the other version:
	// the game holds it as a float and widens it where the step-up reads it, so
	// a profile answering the round 0.6 would step differently from the game.
	// This version stores the attribute already widened, and the dataset carries
	// the widened form.
	for _, check := range []struct {
		name string
		got  float64
		want float64
	}{
		{"gravity", constants.Gravity, 0.08},
		{"horizontal drag", constants.HorizontalDrag, float64(float32(0.91))},
		{"vertical drag", constants.VerticalDrag, float64(float32(0.98))},
		{"step height", constants.StepHeight, float64(float32(0.6))},
	} {
		if check.got != check.want {
			t.Errorf("%s = %v, want %v", check.name, check.got, check.want)
		}
	}
}

func TestAnUnknownFamilyGetsNoConstants(t *testing.T) {
	// A body whose family nobody set must not be moved with a player's numbers.
	if got := built(t).Motion(entity.FamilyUnknown); got != (sim.MotionConstants{}) {
		t.Errorf("an unknown family answered with %+v, want nothing", got)
	}
}

func TestSlipperinessOfAnUnknownHandleIsTheDefault(t *testing.T) {
	if got, want := built(t).Slipperiness(0), 0.6; float32(got) != float32(want) {
		t.Errorf("the zero handle is %v slippery, want the default %v", got, want)
	}
}

func TestTheDataDigestPinsTheNumbersAndNotTheFile(t *testing.T) {
	profile, ok := built(t).(sim.DataDigest)
	if !ok {
		t.Fatal("the profile cannot name the data it was built from")
	}
	if profile.DataDigest().IsZero() {
		t.Fatal("the data digest is zero")
	}

	// Two profiles from the same dataset agree; a profile whose block table
	// differs does not. The second half is what makes the first meaningful.
	other, err := New(dataset(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if profile.DataDigest() != other.(sim.DataDigest).DataDigest() {
		t.Error("two profiles from one dataset disagree about their data")
	}

	set := dataset(t)
	physics := set.Physics()
	physics.BlockSlipperiness["stone"] = 0.7
	moved, err := data.NewSet(data.SetOptions{
		Blocks:          set.Blocks(),
		CollisionShapes: set.CollisionShapes(),
		Physics:         physics,
	})
	if err != nil {
		t.Fatalf("build a data set: %v", err)
	}
	changed, err := New(moved)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if changed.(sim.DataDigest).DataDigest() == profile.DataDigest() {
		t.Error("a dataset with a different stone friction hashed the same")
	}
}

func TestTheProfileIdentityIsSeparateFromTheOtherVersions(t *testing.T) {
	// A digest carries the identity, so two profiles that ran different rules
	// can never collide. This is the one line that keeps 26.1.2 recordings from
	// being replayable as 1.8.9 ones.
	if Identity.GameVersion == "1.8.9" {
		t.Fatal("this profile claims to be 1.8.9")
	}
	if Identity.Edition != "java" || Identity.RulesRevision != "1" {
		t.Errorf("the identity is %+v", Identity)
	}
}

func TestTheSpawnedBoxIsTheBoxTheGameBuilds(t *testing.T) {
	// The game halves a float width and adds a float height to a double
	// position, so a player's box is not the round 0.6 by 1.8 it reads as. This
	// version builds it from the same two floats in the same way, which is a
	// measured agreement with 1.8.9 rather than a shared line of code.
	state, loco, ok := Spawn(built(t), geom.Vec3{X: 0.5, Y: 1, Z: 0.5}, 0, 0)
	if !ok {
		t.Fatal("Spawn did not recognize its own profile")
	}

	const half = float64(float32(0.6) / 2)
	want := geom.AABB{
		MinX: 0.5 - half, MinY: 1, MinZ: 0.5 - half,
		MaxX: 0.5 + half, MaxY: 1 + float64(float32(1.8)), MaxZ: 0.5 + half,
	}
	if state.Box != want {
		t.Errorf("the spawned box is %+v, want %+v", state.Box, want)
	}
	if state.Box.MaxX-state.Box.MinX == 0.6 {
		t.Error("the box is exactly 0.6 wide; the float halving was skipped")
	}
	if state.StepHeight != float64(float32(0.6)) {
		t.Errorf("the spawned step height is %v", state.StepHeight)
	}
	if loco.MoveSpeed != 0.1 {
		t.Errorf("the spawned move speed is %v, want the player's 0.1", loco.MoveSpeed)
	}
}

func TestSpawnRefusesAProfileItDidNotBuild(t *testing.T) {
	if _, _, ok := Spawn(nil, geom.Vec3{}, 0, 0); ok {
		t.Error("Spawn accepted a profile from somewhere else")
	}
	if _, ok := Ref(nil, "stone"); ok {
		t.Error("Ref accepted a profile from somewhere else")
	}
	if _, ok := RefState(nil, 1); ok {
		t.Error("RefState accepted a profile from somewhere else")
	}
}

func TestAStateIdentifierResolvesThroughTheProfile(t *testing.T) {
	profile := built(t)

	stone, ok := dataset(t).Blocks().ByName("stone")
	if !ok {
		t.Fatal("the data set does not know stone")
	}

	byName, ok := Ref(profile, "stone")
	if !ok {
		t.Fatal("the profile does not know stone")
	}
	byState, ok := RefState(profile, stone.DefaultState)
	if !ok {
		t.Fatalf("the profile does not know state %d", stone.DefaultState)
	}
	if byName != byState {
		t.Errorf("stone is handle %d by name and %d by state", byName, byState)
	}
}

// standingWorld builds a floor of one block, with a player standing on it.
func standingWorld(t *testing.T, profile sim.Profile, pos geom.Vec3, floor string) *runtime.Memory {
	t.Helper()

	ground, ok := Ref(profile, floor)
	if !ok {
		t.Fatalf("the profile does not know %s", floor)
	}
	air, ok := Ref(profile, "air")
	if !ok {
		t.Fatal("the profile does not know air")
	}

	store := runtime.NewMemory(profile)
	for x := int32(-5); x <= 5; x++ {
		for z := int32(-5); z <= 5; z++ {
			if err := store.SetBlock(geom.BlockPos{X: x, Y: 0, Z: z}, ground); err != nil {
				t.Fatalf("SetBlock: %v", err)
			}
			// The supporting-block probe reads a cell below the floor, because
			// the game's own sweep does: a block's shape may reach out of its
			// cell, so the cell under the one being stood on is consulted too.
			if err := store.SetBlock(geom.BlockPos{X: x, Y: -1, Z: z}, ground); err != nil {
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
	store := standingWorld(t, profile, geom.Vec3{X: 0.5, Y: 1, Z: 0.5}, "stone")

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{1}})

	for tick := range 20 {
		result, err := runner.Step(context.Background(), nil)
		if err != nil {
			t.Fatalf("tick %d: %v", tick, err)
		}
		if !result.Completeness.Complete {
			t.Fatalf("tick %d was incomplete: %+v", tick, result.Completeness.Missing)
		}
	}

	state, ok := store.Entities().Entity(1)
	if !ok {
		t.Fatal("the body is gone")
	}
	if state.Box.MinY != 1 {
		t.Errorf("the body's feet are at %v, want 1", state.Box.MinY)
	}
	if !state.OnGround {
		t.Error("the body does not report standing")
	}
	if state.Motion.X != 0 || state.Motion.Z != 0 {
		t.Errorf("a standing body drifted: %+v", state.Motion)
	}
}
