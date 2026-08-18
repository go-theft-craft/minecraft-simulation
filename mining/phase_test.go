package mining_test

import (
	"testing"

	gen "github.com/go-theft-craft/minecraft-protocol/generated/java/v1_8"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mining"
	v1_8 "github.com/go-theft-craft/minecraft-simulation/profile/java/v1_8"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/scene"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// digger is the body every test here digs with.
const digger entity.ID = 1

// target is the block every test here digs at.
var target = geom.BlockPos{X: 0, Y: 63, Z: 0}

// miningProfile returns a 1.8.9 profile whose only phase is the dig one.
//
// A profile rather than a fake, because the phase asks it to classify a block
// and a fake would answer whatever the test wanted — which is the difference
// between checking that the phase reads a classifier and checking that a real
// classification reaches it.
func miningProfile(t *testing.T) sim.Profile {
	t.Helper()

	set, err := gen.Data()
	if err != nil {
		t.Fatalf("load the 1.8.9 data set: %v", err)
	}

	built, err := v1_8.New(set)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return digOnly{Profile: built}
}

// digOnly narrows a profile to the dig phase, so a movement rule cannot move
// the body a dig test never asked to move.
type digOnly struct{ sim.Profile }

func (digOnly) Phases() []sim.Phase { return []sim.Phase{mining.Phase()} }

// Ref forwards the name lookup the scene and the phase both need. An embedded
// interface does not carry the optional ones the concrete profile also
// satisfies, and a silent loss of BlockNames leaves a broken block standing.
func (d digOnly) Ref(name string) (world.BlockRef, bool) {
	names, ok := d.Profile.(sim.BlockNames)
	if !ok {
		return 0, false
	}

	return names.Ref(name)
}

// Conditions forwards the classification, for the same reason Ref does.
func (d digOnly) Conditions(
	ref world.BlockRef, held mining.Held, effects mining.Effects, underwater, airborne bool,
) (mining.Conditions, error) {
	return d.Profile.(mining.Classifier).Conditions(ref, held, effects, underwater, airborne)
}

// Hardness forwards the hardness lookup.
func (d digOnly) Hardness(ref world.BlockRef) *float64 {
	return d.Profile.(mining.Classifier).Hardness(ref)
}

// stage builds a kernel and a store holding one block of the named kind at the
// target, standing on stone, with the digger on the ground.
func stage(t *testing.T, block string) (sim.Kernel, *runtime.Memory, sim.Profile) {
	t.Helper()

	profile := miningProfile(t)

	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	store := runtime.NewMemory(profile)
	described := scene.World{
		Min:  geom.BlockPos{X: -1, Y: 62, Z: -1},
		Max:  geom.BlockPos{X: 1, Y: 64, Z: 1},
		Fill: "air",
	}
	if block != "" {
		described.Blocks = []scene.Block{{Pos: target, Name: block}}
	}
	if err := described.Describe(profile, store.SetBlock); err != nil {
		t.Fatalf("describe the world: %v", err)
	}

	store.SetEntity(digger, entity.State{Family: entity.FamilyPlayer, OnGround: true})

	return kernel, store, profile
}

// step runs one tick with the given commands and applies the result.
func step(t *testing.T, kernel sim.Kernel, store *runtime.Memory, profile sim.Profile, commands ...sim.Command) sim.TickResult {
	t.Helper()

	result, err := kernel.Step(t.Context(), sim.TickInput{
		Profile:  profile,
		Revision: store.Revision(),
		Blocks:   store.Blocks(),
		Entities: store.Entities(),
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

// digAt returns the command for one tick of digging, after elapsed earlier ones.
func digAt(elapsed int, held mining.Held) mining.Dig {
	return mining.Dig{
		Entity: digger, Block: target, Face: mining.FaceTop,
		Held: held, Elapsed: elapsed,
	}
}

// countBreaks counts the break events in a result.
func countBreaks(result sim.TickResult) int {
	var broke int
	for _, event := range result.Domain {
		if event.Kind == mining.EventBroke {
			broke++
		}
	}

	return broke
}

// onlyOutcome returns the single command outcome a result carries.
func onlyOutcome(t *testing.T, result sim.TickResult) sim.CommandOutcome {
	t.Helper()

	if len(result.Outcomes) != 1 {
		t.Fatalf("the tick recorded %d outcomes, want one", len(result.Outcomes))
	}

	return result.Outcomes[0]
}

func TestADigBreaksTheBlockOnItsLastTickAndNotBefore(t *testing.T) {
	t.Parallel()

	// A break is progress reaching one, not a countdown, and the exact tick it
	// lands on is what a server validates the elapsed time against.
	kernel, store, profile := stage(t, "stone")

	// Stone with a bare hand: 151 ticks, per mining.BreakTicks.
	const want = 151

	var broke int
	for elapsed := range want {
		broke += countBreaks(step(t, kernel, store, profile, digAt(elapsed, mining.Held{})))
	}

	if broke != 1 {
		t.Fatalf("broke the block %d times over its exact break time, want 1", broke)
	}
}

func TestADigDoesNotBreakEarly(t *testing.T) {
	t.Parallel()

	kernel, store, profile := stage(t, "stone")

	for elapsed := range 150 {
		if countBreaks(step(t, kernel, store, profile, digAt(elapsed, mining.Held{}))) != 0 {
			t.Fatalf("stone broke on tick %d of 151 with a bare hand", elapsed+1)
		}
	}
}

func TestInterruptingADigResetsProgress(t *testing.T) {
	t.Parallel()

	// Vanilla resets rather than pausing, and it does so by keeping the
	// progress on the controller rather than on the world. The elapsed count is
	// the caller's for that reason, so a resumed dig starts at zero and this
	// tick — which would have been the last — is the first.
	kernel, store, profile := stage(t, "stone")

	for elapsed := range 150 {
		step(t, kernel, store, profile, digAt(elapsed, mining.Held{}))
	}
	step(t, kernel, store, profile)

	if countBreaks(step(t, kernel, store, profile, digAt(0, mining.Held{}))) != 0 {
		t.Fatal("the block broke one tick after an interruption; progress was " +
			"paused rather than reset")
	}
}

func TestABrokenBlockBecomesAir(t *testing.T) {
	t.Parallel()

	// The event alone is not enough. A consumer that applied the change set and
	// still saw stone would predict a wall that is not there.
	kernel, store, profile := stage(t, "stone")

	for elapsed := range 151 {
		step(t, kernel, store, profile, digAt(elapsed, mining.Held{}))
	}

	air, ok := profile.(sim.BlockNames).Ref("air")
	if !ok {
		t.Fatal("the profile does not know air")
	}
	if got, _ := store.Blocks().BlockState(target); got != air {
		t.Fatalf("the broken block is handle %d, want air's %d", got, air)
	}
}

func TestDiggingAnUnbreakableBlockIsRejectedWithAReason(t *testing.T) {
	t.Parallel()

	// The outcome must say why. A dig that silently makes no progress is
	// indistinguishable from a dig that is merely slow, and a caller waiting on
	// it waits forever.
	kernel, store, profile := stage(t, "bedrock")

	outcome := onlyOutcome(t, step(t, kernel, store, profile, digAt(0, mining.Held{})))
	if outcome.Accepted {
		t.Fatal("a dig on bedrock was accepted")
	}
	if outcome.Reason == "" {
		t.Fatal("a rejected dig gave no reason; a caller cannot tell it from a slow one")
	}
}

func TestDiggingAnUndescribedBlockIsIncompleteRatherThanImpossible(t *testing.T) {
	t.Parallel()

	// The tri-state block view exists for this. A block the client has not
	// received is not a block that cannot be dug — it is a tick that could not
	// be computed, and the caller is expected to load it and retry.
	kernel, store, profile := stage(t, "stone")

	outside := geom.BlockPos{X: 500, Y: 63, Z: 500}
	result := step(t, kernel, store, profile, mining.Dig{Entity: digger, Block: outside})

	if result.Completeness.Complete {
		t.Fatal("a dig against an undescribed block reported a complete tick")
	}
	if len(result.Completeness.Missing) == 0 {
		t.Fatal("an incomplete tick named nothing missing")
	}
}

func TestABetterToolBreaksTheSameBlockSooner(t *testing.T) {
	t.Parallel()

	// The phase reads the classifier rather than assuming a rate. A phase that
	// ignored the held item would pass every test above and none of this one.
	kernel, store, profile := stage(t, "stone")

	set, err := gen.Data()
	if err != nil {
		t.Fatalf("load the data set: %v", err)
	}
	pickaxe, ok := set.Items().ByName("diamond_pickaxe")
	if !ok {
		t.Fatal("this version has no diamond pickaxe")
	}

	// Stone with a diamond pickaxe: six ticks.
	var broke int
	for elapsed := range 6 {
		broke += countBreaks(step(t, kernel, store, profile,
			digAt(elapsed, mining.Held{Item: pickaxe.ID})))
	}

	if broke != 1 {
		t.Fatalf("a diamond pickaxe broke stone %d times in six ticks, want once", broke)
	}
}
