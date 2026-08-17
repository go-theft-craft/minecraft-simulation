package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/sim"
)

// phaseFunc adapts a function to sim.Phase so a test can state a rule inline.
type phaseFunc struct {
	id  string
	run func(context.Context, *sim.TickState) error
}

func (p phaseFunc) ID() string { return p.id }

func (p phaseFunc) Run(ctx context.Context, tick *sim.TickState) error {
	return p.run(ctx, tick)
}

func phase(id string, run func(*sim.TickState)) sim.Phase {
	return phaseFunc{id: id, run: func(_ context.Context, tick *sim.TickState) error {
		run(tick)

		return nil
	}}
}

// newRunner builds a store, a kernel, and a runner over the given phases.
func newRunner(t *testing.T, phases ...sim.Phase) (*Memory, *Runner) {
	t.Helper()

	profile := testProfile(phases...)
	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	store := NewMemory(profile)

	return store, NewRunner(store, kernel)
}

func TestSteppingAnEmptyProfileAdvancesBothCounters(t *testing.T) {
	// An empty tick still produces an applicable change set, so the revision
	// moves with it. The counters are separate because an *incomplete* tick
	// advances only one of them, which the test below covers.
	store, runner := newRunner(t)

	for want := range sim.Tick(3) {
		if got := runner.Tick(); got != want {
			t.Fatalf("Tick = %d before stepping, want %d", got, want)
		}
		if _, err := runner.Step(context.Background(), nil); err != nil {
			t.Fatalf("Step: %v", err)
		}
	}
	if got := runner.Tick(); got != 3 {
		t.Fatalf("Tick = %d after three steps, want 3", got)
	}
	if got := store.Revision(); got != 3 {
		t.Fatalf("Revision = %d after three applied ticks, want 3", got)
	}
}

func TestAWrittenEntityIsReadableAfterwards(t *testing.T) {
	store, runner := newRunner(t, phase("spawn", func(tick *sim.TickState) {
		tick.SetEntity(1, player())
	}))

	if _, err := runner.Step(context.Background(), nil); err != nil {
		t.Fatalf("Step: %v", err)
	}

	got, ok := store.Entities().Entity(1)
	if !ok || got != player() {
		t.Fatalf("Entity = (%+v, %v), want the body the tick wrote", got, ok)
	}
}

func TestTheSecondStepSeesTheFirstStepsRevision(t *testing.T) {
	_, runner := newRunner(t, phase("spawn", func(tick *sim.TickState) {
		tick.SetEntity(1, player())
	}))

	first, err := runner.Step(context.Background(), nil)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	second, err := runner.Step(context.Background(), nil)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}

	if first.Changes.BaseRevision != 0 {
		t.Fatalf("the first set is based at %d, want 0", first.Changes.BaseRevision)
	}
	if second.Changes.BaseRevision != 1 {
		t.Fatalf("the second set is based at %d, want 1", second.Changes.BaseRevision)
	}
}

func TestAnIncompleteResultIsNotApplied(t *testing.T) {
	store, runner := newRunner(t, phase("read", func(tick *sim.TickState) {
		tick.SetEntity(1, player())
		tick.BlockShape(geom.BlockPos{X: 900})
	}))

	got, err := runner.Step(context.Background(), nil)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if got.Completeness.Complete {
		t.Fatal("a tick that read an unknown cell reported itself complete")
	}
	if store.Revision() != 0 {
		t.Fatalf("Revision = %d after an incomplete tick, want 0", store.Revision())
	}
	if _, ok := store.Entities().Entity(1); ok {
		t.Error("an incomplete tick's write reached the store")
	}
	if runner.Tick() != 1 {
		t.Fatalf("Tick = %d after an incomplete tick, want 1: the tick still happened", runner.Tick())
	}
}

func TestTheRunnersRandomStateFollowsItsResults(t *testing.T) {
	_, runner := newRunner(t, phase("draw", func(tick *sim.TickState) {
		state, _ := tick.Random().Stream("world")
		tick.SetRandom(tick.Random().WithStream("world", state+1))
	}))
	runner.SetRandom(sim.RandomState{}.WithStream("world", 0))

	for range 3 {
		if _, err := runner.Step(context.Background(), nil); err != nil {
			t.Fatalf("Step: %v", err)
		}
	}

	result, err := runner.Step(context.Background(), nil)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if state, _ := result.Random.Stream("world"); state != 4 {
		t.Fatalf("the stream reached %d over four ticks, want 4", state)
	}
}

func TestCommandsReachThePhasesAndTheirOutcomesComeBack(t *testing.T) {
	_, runner := newRunner(t, phase("apply", func(tick *sim.TickState) {
		for index, command := range tick.Commands() {
			tick.RecordOutcome(sim.CommandOutcome{
				Index:    index,
				Kind:     command.CommandKind(),
				Accepted: false,
				Reason:   "no rule handles it",
			})
		}
	}))

	got, err := runner.Step(context.Background(), []sim.Command{unknownCommand{}})
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if len(got.Outcomes) != 1 || got.Outcomes[0].Accepted {
		t.Fatalf("Outcomes = %+v, want one rejection", got.Outcomes)
	}
}

// TestAStaleChangeSetIsRefused is the milestone's second exit criterion: a
// change set computed against revision N is refused by a store at revision N+1,
// which is what makes a client's discarded prediction fork safe.
func TestAStaleChangeSetIsRefused(t *testing.T) {
	store, runner := newRunner(t, phase("spawn", func(tick *sim.TickState) {
		tick.SetEntity(1, player())
	}))

	result, err := runner.Step(context.Background(), nil)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if store.Revision() != 1 {
		t.Fatalf("Revision = %d after one applied tick, want 1", store.Revision())
	}
	if result.Changes.BaseRevision != 0 {
		t.Fatalf("the set is based at %d, want 0", result.Changes.BaseRevision)
	}

	if err := store.Apply(result.Changes); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("re-applying the set returned %v, want ErrStaleRevision", err)
	}
	if store.Revision() != 1 {
		t.Fatalf("the refused set advanced the revision to %d", store.Revision())
	}
}

func TestAFailedTickLeavesTheStoreAlone(t *testing.T) {
	failure := errors.New("rule failed")
	store, runner := newRunner(t, phaseFunc{id: "boom", run: func(context.Context, *sim.TickState) error {
		return failure
	}})

	if _, err := runner.Step(context.Background(), nil); !errors.Is(err, failure) {
		t.Fatalf("Step error = %v, want the phase's error", err)
	}
	if store.Revision() != 0 {
		t.Fatalf("Revision = %d after a failed tick, want 0", store.Revision())
	}
	if runner.Tick() != 0 {
		t.Fatalf("Tick = %d after a failed tick, want 0: the tick produced no record", runner.Tick())
	}
}

func TestScopeAndLimitsReachTheTick(t *testing.T) {
	var (
		gotScope  sim.Scope
		gotLimits sim.Limits
	)
	_, runner := newRunner(t, phase("observe", func(tick *sim.TickState) {
		gotScope = tick.Scope()
		gotLimits = tick.Limits()
	}))

	runner.SetScope(sim.Scope{Entities: []entity.ID{7}})
	runner.SetLimits(sim.Limits{Events: 2})

	if _, err := runner.Step(context.Background(), nil); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if len(gotScope.Entities) != 1 || gotScope.Entities[0] != 7 {
		t.Fatalf("the tick saw scope %+v, want entity 7", gotScope)
	}
	if gotLimits.Events != 2 {
		t.Fatalf("the tick saw an event budget of %d, want 2", gotLimits.Events)
	}
	if gotLimits.EntitySteps <= 0 {
		t.Fatalf("the tick saw an unusable entity budget: %+v", gotLimits)
	}
}

// unknownCommand is an intent no phase in these tests handles.
type unknownCommand struct{}

func (unknownCommand) CommandKind() string { return "test.unknown" }
