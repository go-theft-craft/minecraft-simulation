package v26_1

import (
	"context"
	"math"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// oneTick runs a single tick of a player standing on a floor and returns what
// it left behind.
func oneTick(
	t *testing.T, profile sim.Profile, floor string, prepare func(*runtime.Memory), input movement.Input,
) (entity.State, movement.Locomotion) {
	t.Helper()

	store := standingWorld(t, profile, geom.Vec3{X: 0.5, Y: 1, Z: 0.5}, floor)
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
	loco, ok := store.Locomotion().Locomotion(1)
	if !ok {
		t.Fatal("the body has no locomotion state")
	}

	return state, loco
}

func TestTheMotionThresholdIsAVectorRule(t *testing.T) {
	// Each axis alone is under 1.8.9's per-axis threshold and under this
	// version's 0.003, but the vector is longer than 0.003 — so 1.8.9 would
	// discard both components and this version keeps them. That difference is
	// the whole reason this phase is not shared.
	kept := geom.Vec3{X: 0.0025, Z: 0.0025}
	if got := clampMotion(kept); got != kept {
		t.Errorf("a motion of %v was discarded as %v; the rule is on the vector", kept, got)
	}

	// One axis alone, shorter than 0.003: the vector is too, so both go.
	discarded := geom.Vec3{X: 0.0029}
	if got := clampMotion(discarded); got.X != 0 || got.Z != 0 {
		t.Errorf("a motion of %v survived as %v", discarded, got)
	}

	// The vertical is its own axis, in this version as in the other.
	vertical := geom.Vec3{Y: 0.0029}
	if got := clampMotion(vertical); got.Y != 0 {
		t.Errorf("a vertical motion of %v survived as %v", vertical, got)
	}
	if got := clampMotion(geom.Vec3{Y: 0.0031}); got.Y != 0.0031 {
		t.Errorf("a vertical motion above the threshold was discarded as %v", got)
	}
}

func TestTheAccelerationDividesTheRawFriction(t *testing.T) {
	// The two versions write this rule differently and mean the same thing: this
	// version divides 0.21600002F by the cube of the block's own friction, and
	// 1.8.9 divides 0.16277136F by the cube of that friction times 0.91, which is
	// the same number to within a rounding because 0.16277136 / 0.91³ is
	// 0.21600002.
	//
	// So the danger is not the formula. It is the half-port: keeping 1.8.9's
	// denominator and swapping only the constant, which is what a profile written
	// by analogy would do. That is what this pins.
	for _, friction := range []float32{0.6, 0.98, 0.989, 0.8} {
		raw := accelerationNumerator / (friction * friction * friction)

		drag := friction * frictionDrag
		halfPorted := accelerationNumerator / (drag * drag * drag)
		if raw == halfPorted {
			t.Fatalf("at friction %v the raw and the half-ported rules agree; "+
				"the case proves nothing", friction)
		}
		if halfPorted < raw {
			t.Errorf("at friction %v the half-ported rule accelerates at %v against %v; "+
				"it should be the larger, since it divides by a smaller cube",
				friction, halfPorted, raw)
		}

		// And the version it came from, written out in full, agrees with this one
		// to within single-width rounding.
		asOldVersion := float32(0.16277136) / (drag * drag * drag)
		if diff := math.Abs(float64(raw - asOldVersion)); diff > 1e-6 {
			t.Errorf("at friction %v the two versions accelerate at %v and %v", friction, raw, asOldVersion)
		}
	}
}

func TestAJumpTakesTheLargerOfThePowerAndTheMotion(t *testing.T) {
	profile := built(t)

	// Standing still and jumping: the motion becomes the jump power, which is
	// the attribute at single width.
	state, loco := oneTick(t, profile, "stone", nil, movement.Input{Entity: 1, Jump: true})
	if loco.JumpTicks != jumpDelay {
		t.Errorf("the jump counter is %d, want %d", loco.JumpTicks, jumpDelay)
	}

	// The tick applies gravity and the vertical drag after the jump, so the
	// motion the tick leaves is the jump power carried through both.
	want := (float64(defaultJumpStrength) - 0.08) * float64(float32(0.98))
	if state.Motion.Y != want {
		t.Errorf("a jump left the motion at %v, want %v", state.Motion.Y, want)
	}

	// A body already rising faster than a jump keeps its own motion here, where
	// 1.8.9 assigns over it.
	rising := oneTickFrom(t, profile, geom.Vec3{Y: 0.5}, movement.Input{Entity: 1, Jump: true})
	if rising <= want {
		t.Errorf("a body rising at 0.5 jumped to %v, want more than the jump's %v", rising, want)
	}
}

// oneTickFrom runs a tick for a body that starts with a motion.
func oneTickFrom(t *testing.T, profile sim.Profile, motion geom.Vec3, input movement.Input) float64 {
	t.Helper()

	state, _ := oneTick(t, profile, "stone", func(store *runtime.Memory) {
		body, ok := store.Entities().Entity(1)
		if !ok {
			t.Fatal("the body is gone")
		}
		body.Motion = motion
		store.SetEntity(1, body)
	}, input)

	return state.Motion.Y
}

func TestTheJumpCounterBlocksTheNextTen(t *testing.T) {
	profile := built(t)
	store := standingWorld(t, profile, geom.Vec3{X: 0.5, Y: 1, Z: 0.5}, "stone")

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	runner := runtime.NewRunner(store, kernel)
	runner.SetScope(sim.Scope{Entities: []entity.ID{1}})

	// Holding jump through a whole flight: the counter is what stops a body from
	// jumping again the moment it lands, and it is the same counter 1.8.9 has.
	var jumps int
	previous := 1.0
	for range 40 {
		if _, err := runner.Step(context.Background(), []sim.Command{
			movement.Input{Entity: 1, Jump: true},
		}); err != nil {
			t.Fatalf("Step: %v", err)
		}
		state, _ := store.Entities().Entity(1)
		if state.Box.MinY > previous {
			jumps++
		}
		previous = state.Box.MinY
	}
	if jumps == 0 {
		t.Fatal("holding jump never left the ground")
	}
}

func TestSneakingScalesTheInputBeforeTheDecayIsClamped(t *testing.T) {
	// A single axis: the shaping is the identity beyond the decay and the sneak
	// factor, so the result is the plain product and the order does not show.
	strafe, forward := ShapeInput(0, 1, false)
	if strafe != 0 || forward != inputDecay {
		t.Errorf("a forward input shaped to (%v, %v), want (0, %v)", strafe, forward, inputDecay)
	}

	sneaked, forwardSneaked := ShapeInput(0, 1, true)
	if want := inputDecay * sneakingSpeed; forwardSneaked != want || sneaked != 0 {
		t.Errorf("a sneaking forward input shaped to (%v, %v), want (0, %v)",
			sneaked, forwardSneaked, want)
	}
}

func TestADiagonalInputReachesTheUnitSquare(t *testing.T) {
	// The rule the shaping exists for. A keyboard diagonal is (1, 1), which is
	// longer than one, so the clamp takes it back to one — and the clamp
	// discards the 0.98 decay with everything else above one. A body walking
	// diagonally therefore moves at the full input rather than at 0.98 of it,
	// and a profile that decayed after the shaping would be two percent slow on
	// every diagonal.
	strafe, forward := ShapeInput(1, 1, false)

	want := float32(math.Sqrt(0.5))
	if strafe != want || forward != want {
		t.Errorf("a diagonal input shaped to (%v, %v), want (%v, %v)", strafe, forward, want, want)
	}

	// The composition is not the same as decaying afterwards, which is the
	// statement the phase order rests on.
	if strafe == want*inputDecay {
		t.Fatal("the decay survived the clamp; the shaping is written in the wrong order")
	}

	// A zero input is untouched, including its sign.
	if x, z := ShapeInput(0, 0, true); x != 0 || z != 0 {
		t.Errorf("a zero input shaped to (%v, %v)", x, z)
	}
}

func TestTheBlockBelowIsTheColumnOfTheBlockStoodOn(t *testing.T) {
	// The difference from 1.8.9, in one state. The body stands on the lip of a
	// block with its centre out past the edge, over air, and the move that put it
	// there recorded which block held it up. 1.8.9 reads the cell under the
	// position and finds nothing; this version keeps the supporting block's
	// column.
	state := entity.State{
		Position: geom.Vec3{X: 1.05, Y: 1, Z: 0.5},
		Support:  entity.Support{Block: geom.BlockPos{}, Present: true},
	}
	state.Box = playerBox(state.Position)

	if got, want := blockBelow(state), (geom.BlockPos{}); got != want {
		t.Errorf("the friction cell is %+v, want the supporting block's column %+v", got, want)
	}

	// Without a record there is nothing to keep, and the rule falls back to the
	// column of the body's own position — which is 1.8.9's answer, one cell over.
	bare := state
	bare.Support = entity.Support{}
	if got, want := blockBelow(bare), (geom.BlockPos{X: 1}); got != want {
		t.Errorf("with no record the friction cell is %+v, want %+v", got, want)
	}
}

func TestABodyOnASlabReadsThroughIt(t *testing.T) {
	// A quirk worth pinning rather than smoothing over: the supporting block
	// supplies the column and the offset supplies the height, and half a block
	// below a slab's top is the cell *under* the slab. So a body standing on an
	// ice slab does not get ice friction — it gets whatever holds the slab up.
	state := entity.State{
		Position: geom.Vec3{X: 0.5, Y: 0.5, Z: 0.5},
		Support:  entity.Support{Block: geom.BlockPos{}, Present: true},
	}
	state.Box = playerBox(state.Position)

	if got, want := blockBelow(state), (geom.BlockPos{Y: -1}); got != want {
		t.Errorf("the friction cell is %+v, want %+v, the cell under the slab", got, want)
	}
}

func TestTheSupportProbeReportsACellNobodyDescribed(t *testing.T) {
	// A body over a region nobody has described must leave the tick incomplete
	// rather than record a support the world might have contradicted.
	blocks := world.NewBlocks()

	pos := geom.Vec3{X: 0.5, Y: 1, Z: 0.5}
	box := playerBox(pos)
	if _, _, ok := supportingBlock(blocks.CollisionShape, geom.AABB{
		MinX: box.MinX, MinY: box.MinY - supportProbe, MinZ: box.MinZ,
		MaxX: box.MaxX, MaxY: box.MinY, MaxZ: box.MaxZ,
	}, pos); ok {
		t.Error("the probe answered over a world it could not read")
	}
}

func TestTheSupportProbeTakesTheNearestBlockUnderTheBody(t *testing.T) {
	blocks := describedRegion(t)
	blocks.SetBlock(geom.BlockPos{}, 1, geom.FullCube())
	blocks.SetBlock(geom.BlockPos{X: 1}, 2, geom.FullCube())

	// Straddling two blocks, closer to the second: the record is the nearer of
	// the two, by the distance from the body's position to each cell's centre.
	pos := geom.Vec3{X: 1.4, Y: 1, Z: 0.5}
	box := playerBox(pos)
	probe := geom.AABB{
		MinX: box.MinX, MinY: box.MinY - supportProbe, MinZ: box.MinZ,
		MaxX: box.MaxX, MaxY: box.MinY, MaxZ: box.MaxZ,
	}

	block, found, ok := supportingBlock(blocks.CollisionShape, probe, pos)
	if !ok || !found {
		t.Fatalf("the probe found nothing under a body standing on two blocks (ok=%v)", ok)
	}
	if want := (geom.BlockPos{X: 1}); block != want {
		t.Errorf("the support is %+v, want the nearer block %+v", block, want)
	}
}

// describedRegion is a small world of air, so that a probe reads a described
// cell wherever it looks and a test can place the one block it cares about.
func describedRegion(t *testing.T) *world.Blocks {
	t.Helper()

	blocks := world.NewBlocks()
	for x := int32(-3); x <= 3; x++ {
		for y := int32(-3); y <= 5; y++ {
			for z := int32(-3); z <= 3; z++ {
				blocks.SetAir(geom.BlockPos{X: x, Y: y, Z: z})
			}
		}
	}

	return blocks
}
