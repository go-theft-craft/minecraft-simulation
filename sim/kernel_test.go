package sim

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// phaseFunc adapts a function to Phase, so tests can state a phase inline.
type phaseFunc struct {
	id  string
	run func(context.Context, *TickState) error
}

func (p phaseFunc) ID() string { return p.id }

func (p phaseFunc) Run(ctx context.Context, tick *TickState) error {
	return p.run(ctx, tick)
}

// emptyWorld describes a small region as air, so a lookup inside it is known.
func emptyWorld() *world.Blocks {
	blocks := world.NewBlocks()
	blocks.Fill(geom.BlockPos{X: -4, Y: -4, Z: -4}, geom.BlockPos{X: 4, Y: 4, Z: 4}, geom.EmptyShape())

	return blocks
}

func testInput(profile Profile) TickInput {
	return TickInput{
		Profile:  profile,
		Revision: 5,
		Tick:     9,
		Blocks:   emptyWorld(),
		Entities: entity.NewBodies(),
	}
}

func TestNewKernelRejectsABadProfile(t *testing.T) {
	if _, err := NewKernel(nil); err == nil {
		t.Error("NewKernel accepted a nil profile")
	}
	if _, err := NewKernel(&StaticProfile{}); err == nil {
		t.Error("NewKernel accepted a profile with no identity")
	}

	duplicate := &StaticProfile{
		Identity: sampleProfileID,
		PhaseList: []Phase{
			phaseFunc{id: "a", run: func(context.Context, *TickState) error { return nil }},
			phaseFunc{id: "a", run: func(context.Context, *TickState) error { return nil }},
		},
	}
	if _, err := NewKernel(duplicate); err == nil {
		t.Error("NewKernel accepted two phases with the same identifier")
	}
}

func TestStepRejectsAnInputItCannotRun(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	noProfile := testInput(profile)
	noProfile.Profile = nil
	if _, err := kernel.Step(context.Background(), noProfile); err == nil {
		t.Error("Step accepted an input with no profile")
	}

	otherProfile := testInput(&StaticProfile{
		Identity: ProfileID{Edition: "java", GameVersion: "26.1.2", RulesRevision: "1"},
	})
	if _, err := kernel.Step(context.Background(), otherProfile); err == nil {
		t.Error("Step accepted an input naming another profile")
	}

	noBlocks := testInput(profile)
	noBlocks.Blocks = nil
	if _, err := kernel.Step(context.Background(), noBlocks); err == nil {
		t.Error("Step accepted an input with no block view")
	}

	noEntities := testInput(profile)
	noEntities.Entities = nil
	if _, err := kernel.Step(context.Background(), noEntities); err == nil {
		t.Error("Step accepted an input with no entity view")
	}
}

// TestEmptyTickIsStable is the milestone's first exit criterion: a tick that
// does no work produces the same digest every time it runs.
func TestEmptyTickIsStable(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	first, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	second, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if first.Digest != second.Digest {
		t.Fatalf("an empty tick digested differently: %s vs %s", first.Digest, second.Digest)
	}
	if first.Digest.IsZero() {
		t.Fatal("an empty tick produced no digest")
	}
	if !first.Completeness.Complete {
		t.Fatalf("an empty tick was incomplete: %+v", first.Completeness)
	}
	if !first.Changes.IsEmpty() {
		t.Fatalf("an empty tick produced operations: %+v", first.Changes.Ops)
	}
	if first.Revision != 5 || first.Tick != 9 {
		t.Fatalf("the result did not carry its input revision and tick: %+v", first)
	}
	if first.Changes.BaseRevision != 5 {
		t.Fatalf("BaseRevision = %d, want the input revision", first.Changes.BaseRevision)
	}
}

// TestEmptyTickDigestIsPinned makes an accidental encoding change visible. When
// the encoding changes on purpose, run the test, read the new value from the
// failure, and update this constant in the same commit as the change.
func TestEmptyTickDigestIsPinned(t *testing.T) {
	const want = "5587fe58fcacf086e87614abebf246688f27403756de1f4b41f535870e89c602"

	profile := &StaticProfile{Identity: sampleProfileID}
	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	got, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got.Digest.String() != want {
		t.Fatalf("the empty tick digest is %s, pinned at %s", got.Digest, want)
	}
}

func TestAPhaseWritesThroughTheTickState(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "spawn", run: func(_ context.Context, tick *TickState) error {
		tick.SetEntity(1, entity.State{Family: entity.FamilyPlayer})
		tick.SetBlock(geom.BlockPos{X: 1}, 4)
		tick.EmitDomain(DomainEvent{Kind: "test.spawned", Entity: 1})
		tick.EmitPresentation(PresentationEvent{Kind: "test.puff"})
		tick.RecordOutcome(CommandOutcome{Index: 0, Kind: "test.none", Accepted: true})

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	got, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if len(got.Changes.Ops) != 2 {
		t.Fatalf("the tick produced %d operations, want 2: %+v", len(got.Changes.Ops), got.Changes.Ops)
	}
	if got.Changes.Ops[0].Kind != OpSetEntity || got.Changes.Ops[1].Kind != OpSetBlock {
		t.Fatalf("operations are out of phase order: %+v", got.Changes.Ops)
	}
	if len(got.Domain) != 1 || len(got.Presentation) != 1 || len(got.Outcomes) != 1 {
		t.Fatalf("the tick lost an event or an outcome: %+v", got)
	}
}

func TestAPhaseReadsTheCommandsItWasGiven(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "apply", run: func(_ context.Context, tick *TickState) error {
		for index, command := range tick.Commands() {
			tick.RecordOutcome(CommandOutcome{
				Index:    index,
				Kind:     command.CommandKind(),
				Accepted: true,
			})
		}

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	input := testInput(profile)
	input.Commands = []Command{walkCommand{forward: 1}}

	got, err := kernel.Step(context.Background(), input)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if len(got.Outcomes) != 1 || got.Outcomes[0].Kind != "movement.walk" {
		t.Fatalf("Outcomes = %+v, want one accepted walk", got.Outcomes)
	}
}

func TestAnUnknownLookupMakesTheTickIncompleteAndDropsItsWork(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "read", run: func(_ context.Context, tick *TickState) error {
		// Write first, then read a cell nobody described. The write must not
		// survive: an incomplete result carries no applicable changes.
		tick.SetEntity(1, entity.State{Family: entity.FamilyPlayer})
		if _, lookup := tick.BlockShape(geom.BlockPos{X: 900}); lookup != world.LookupUnknown {
			t.Errorf("lookup = %v, want LookupUnknown", lookup)
		}

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	got, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got.Completeness.Complete {
		t.Fatal("a tick that read an unknown cell reported itself complete")
	}
	if len(got.Completeness.Missing) != 1 {
		t.Fatalf("Missing = %+v, want the one unknown cell", got.Completeness.Missing)
	}
	if !got.Changes.IsEmpty() || len(got.Domain) != 0 {
		t.Fatalf("an incomplete tick kept its work: %+v", got)
	}
	if got.Digest.IsZero() {
		t.Fatal("an incomplete tick produced no digest; it is still a result")
	}
}

func TestMissingBlocksDeclaresIncompletenessExplicitly(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "sweep", run: func(_ context.Context, tick *TickState) error {
		// This is what a phase that resolved collision itself must do with the
		// unknown cells the sweep reported.
		tick.MissingBlocks([]geom.BlockPos{{X: 7}, {X: 8}})

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	got, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if got.Completeness.Complete || len(got.Completeness.Missing) != 2 {
		t.Fatalf("Completeness = %+v, want the two declared cells", got.Completeness)
	}
}

func TestReadDependenciesAreSortedAndDeduplicated(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "read", run: func(_ context.Context, tick *TickState) error {
		for _, pos := range []geom.BlockPos{{X: 2}, {X: 1}, {X: 2}, {X: 0}} {
			tick.BlockShape(pos)
		}

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	got, err := kernel.Step(context.Background(), testInput(profile))
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if len(got.Read) != 3 {
		t.Fatalf("Read holds %d dependencies, want 3: %+v", len(got.Read), got.Read)
	}
	for index := 1; index < len(got.Read); index++ {
		if got.Read[index-1].Block.X >= got.Read[index].Block.X {
			t.Fatalf("Read is not sorted: %+v", got.Read)
		}
	}
}

func TestAPhaseErrorAbortsTheTick(t *testing.T) {
	failure := errors.New("rule failed")
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "boom", run: func(context.Context, *TickState) error {
		return failure
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	got, err := kernel.Step(context.Background(), testInput(profile))
	if !errors.Is(err, failure) {
		t.Fatalf("Step error = %v, want the phase's error", err)
	}
	if !got.Changes.IsEmpty() {
		t.Fatalf("a failed tick returned an applicable change set: %+v", got.Changes)
	}
}

func TestCancellationAbortsTheTick(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "never", run: func(context.Context, *TickState) error {
		t.Error("a phase ran under a cancelled context")

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := kernel.Step(ctx, testInput(profile))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Step error = %v, want context.Canceled", err)
	}
	if !got.Changes.IsEmpty() {
		t.Fatalf("a cancelled tick returned an applicable change set: %+v", got.Changes)
	}
}

func TestTheEventBudgetIsEnforced(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "flood", run: func(_ context.Context, tick *TickState) error {
		for range 10 {
			tick.EmitDomain(DomainEvent{Kind: "test.noise"})
		}

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	input := testInput(profile)
	input.Limits = Limits{Events: 3}

	if _, err := kernel.Step(context.Background(), input); !errors.Is(err, ErrLimitExhausted) {
		t.Fatalf("Step error = %v, want ErrLimitExhausted", err)
	}
}

func TestTheEntityAndBlockBudgetsAreEnforced(t *testing.T) {
	for name, flood := range map[string]func(*TickState){
		"entity steps": func(tick *TickState) {
			for range 10 {
				tick.SetEntity(1, entity.State{Family: entity.FamilyPlayer})
			}
		},
		"block updates": func(tick *TickState) {
			for index := range 10 {
				tick.SetBlock(geom.BlockPos{X: int32(index)}, 1)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			profile := &StaticProfile{Identity: sampleProfileID}
			profile.PhaseList = []Phase{phaseFunc{
				id: "flood",
				run: func(_ context.Context, tick *TickState) error {
					flood(tick)

					return nil
				},
			}}

			kernel, err := NewKernel(profile)
			if err != nil {
				t.Fatalf("NewKernel: %v", err)
			}

			input := testInput(profile)
			input.Limits = Limits{EntitySteps: 2, BlockUpdates: 2}

			if _, err := kernel.Step(context.Background(), input); !errors.Is(err, ErrLimitExhausted) {
				t.Fatalf("Step error = %v, want ErrLimitExhausted", err)
			}
		})
	}
}

func TestANaNInAResultIsRefused(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "nan", run: func(_ context.Context, tick *TickState) error {
		tick.SetEntity(1, entity.State{
			Family: entity.FamilyPlayer,
			Motion: geom.Vec3{Y: math.NaN()},
		})

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	if _, err := kernel.Step(context.Background(), testInput(profile)); !errors.Is(err, ErrNaNInResult) {
		t.Fatalf("Step error = %v, want ErrNaNInResult", err)
	}
}

func TestTheRandomStateSurvivesATick(t *testing.T) {
	profile := &StaticProfile{Identity: sampleProfileID}
	profile.PhaseList = []Phase{phaseFunc{id: "draw", run: func(_ context.Context, tick *TickState) error {
		state, _ := tick.Random().Stream("world")
		tick.SetRandom(tick.Random().WithStream("world", state+1))

		return nil
	}}}

	kernel, err := NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}

	input := testInput(profile)
	input.Random = RandomState{}.WithStream("world", 41)

	got, err := kernel.Step(context.Background(), input)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if state, _ := got.Random.Stream("world"); state != 42 {
		t.Fatalf("the result's random state is %d, want 42", state)
	}
	// The input's own state must not have moved: a kernel holds no state, and a
	// caller that wants to replay the tick has to be able to.
	if state, _ := input.Random.Stream("world"); state != 41 {
		t.Fatalf("Step mutated its input's random state to %d", state)
	}
}
