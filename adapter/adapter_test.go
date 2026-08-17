package adapter

import (
	"context"
	"errors"
	"testing"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/runtime"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// stone is the one block handle these tests use.
const stone world.BlockRef = 1

// phaseFunc adapts a function to sim.Phase, so a test can state a rule inline.
type phaseFunc struct {
	id  string
	run func(*sim.TickState)
}

func (p phaseFunc) ID() string { return p.id }

func (p phaseFunc) Run(_ context.Context, tick *sim.TickState) error {
	p.run(tick)

	return nil
}

// staticSource answers with fields.
type staticSource struct {
	tick     sim.Tick
	commands []sim.Command
	limits   sim.Limits
	scope    sim.Scope
}

func (s staticSource) Tick() sim.Tick          { return s.tick }
func (s staticSource) Commands() []sim.Command { return s.commands }
func (s staticSource) Limits() sim.Limits      { return s.limits }
func (s staticSource) Scope() sim.Scope        { return s.scope }

// recordingSink writes through to a store and records what it saw.
type recordingSink struct {
	store    *runtime.Memory
	observed []sim.TickResult
	failure  error
}

func (r *recordingSink) Apply(changes sim.ChangeSet) error {
	if r.failure != nil {
		return r.failure
	}

	return r.store.Apply(changes)
}

func (r *recordingSink) Observe(result sim.TickResult) {
	r.observed = append(r.observed, result)
}

// walk is an intent no rule in these tests handles.
type walk struct{}

func (walk) CommandKind() string { return "movement.walk" }

func player() entity.State {
	return entity.State{
		Family:     entity.FamilyPlayer,
		Box:        geom.AABB{MinX: -0.3, MinZ: -0.3, MaxX: 0.3, MaxY: 1.8, MaxZ: 0.3},
		StepHeight: float64(float32(0.6)),
	}
}

// harness builds a kernel, a store, and a sink over the given phases.
func harness(t *testing.T, phases ...sim.Phase) (sim.Kernel, *runtime.Memory, *recordingSink) {
	t.Helper()

	profile := &sim.StaticProfile{
		Identity:  sim.ProfileID{Edition: "java", GameVersion: "1.8.9", RulesRevision: "1"},
		PhaseList: phases,
		Shapes:    map[world.BlockRef]geom.Shape{stone: geom.FullCube()},
	}
	kernel, err := sim.NewKernel(profile)
	if err != nil {
		t.Fatalf("NewKernel: %v", err)
	}
	store := runtime.NewMemory(profile)

	return kernel, store, &recordingSink{store: store}
}

func TestDriveAppliesACompleteResult(t *testing.T) {
	kernel, store, sink := harness(t, phaseFunc{id: "spawn", run: func(tick *sim.TickState) {
		tick.SetEntity(1, player())
	}})

	got, err := Drive(context.Background(), kernel, store, staticSource{tick: 4}, sink)
	if err != nil {
		t.Fatalf("Drive: %v", err)
	}
	if !got.Completeness.Complete {
		t.Fatalf("the result was incomplete: %+v", got.Completeness)
	}
	if got.Tick != 4 {
		t.Fatalf("the result carries tick %d, want the source's 4", got.Tick)
	}
	if body, ok := store.Entities().Entity(1); !ok || body != player() {
		t.Fatalf("Entity = (%+v, %v), want the body the tick wrote", body, ok)
	}
	if store.Revision() != 1 {
		t.Fatalf("Revision = %d after one applied tick, want 1", store.Revision())
	}
}

func TestDriveDoesNotApplyAnIncompleteResult(t *testing.T) {
	kernel, store, sink := harness(t, phaseFunc{id: "read", run: func(tick *sim.TickState) {
		tick.SetEntity(1, player())
		tick.BlockShape(geom.BlockPos{X: 900})
	}})

	got, err := Drive(context.Background(), kernel, store, staticSource{}, sink)
	if err != nil {
		t.Fatalf("Drive: %v", err)
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
}

func TestObserveSeesBothKindsOfResult(t *testing.T) {
	// One phase that reads a cell nobody described, switched on per tick, so the
	// same sink sees a complete result and an incomplete one.
	var incomplete bool
	kernel, store, sink := harness(t, phaseFunc{id: "read", run: func(tick *sim.TickState) {
		if incomplete {
			tick.BlockShape(geom.BlockPos{X: 900})
		}
	}})

	if _, err := Drive(context.Background(), kernel, store, staticSource{tick: 1}, sink); err != nil {
		t.Fatalf("Drive: %v", err)
	}
	incomplete = true
	if _, err := Drive(context.Background(), kernel, store, staticSource{tick: 2}, sink); err != nil {
		t.Fatalf("Drive: %v", err)
	}

	if len(sink.observed) != 2 {
		t.Fatalf("the sink observed %d results, want 2", len(sink.observed))
	}
	if !sink.observed[0].Completeness.Complete {
		t.Error("the first result reached the sink as incomplete")
	}
	if sink.observed[1].Completeness.Complete {
		t.Error("the incomplete result reached the sink as complete")
	}
}

func TestTheStoresRevisionReachesTheInputAsTheBaseRevision(t *testing.T) {
	kernel, store, sink := harness(t, phaseFunc{id: "spawn", run: func(tick *sim.TickState) {
		tick.SetEntity(1, player())
	}})

	first, err := Drive(context.Background(), kernel, store, staticSource{tick: 1}, sink)
	if err != nil {
		t.Fatalf("Drive: %v", err)
	}
	second, err := Drive(context.Background(), kernel, store, staticSource{tick: 2}, sink)
	if err != nil {
		t.Fatalf("Drive: %v", err)
	}

	if first.Changes.BaseRevision != 0 || second.Changes.BaseRevision != 1 {
		t.Fatalf("base revisions are %d then %d, want 0 then 1",
			first.Changes.BaseRevision, second.Changes.BaseRevision)
	}
}

func TestASinkErrorPropagatesAndTheStoreIsUnchanged(t *testing.T) {
	failure := errors.New("the fork moved on")
	kernel, store, sink := harness(t, phaseFunc{id: "spawn", run: func(tick *sim.TickState) {
		tick.SetEntity(1, player())
	}})
	sink.failure = failure

	if _, err := Drive(context.Background(), kernel, store, staticSource{}, sink); !errors.Is(err, failure) {
		t.Fatalf("Drive error = %v, want the sink's error", err)
	}
	if store.Revision() != 0 {
		t.Fatalf("Revision = %d after a refused apply, want 0", store.Revision())
	}
	if _, ok := store.Entities().Entity(1); ok {
		t.Error("the body reached the store despite the sink refusing the set")
	}
}

func TestACancelledContextReturnsWithoutStepping(t *testing.T) {
	kernel, store, sink := harness(t, phaseFunc{id: "never", run: func(*sim.TickState) {
		t.Error("a phase ran under a cancelled context")
	}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Drive(ctx, kernel, store, staticSource{}, sink); !errors.Is(err, context.Canceled) {
		t.Fatalf("Drive error = %v, want context.Canceled", err)
	}
	if len(sink.observed) != 0 {
		t.Fatalf("the sink observed %d results under a cancelled context", len(sink.observed))
	}
}

func TestTheSourcesCommandsAndBudgetsReachTheTick(t *testing.T) {
	var (
		gotCommands []sim.Command
		gotScope    sim.Scope
		gotLimits   sim.Limits
	)
	kernel, store, sink := harness(t, phaseFunc{id: "observe", run: func(tick *sim.TickState) {
		gotCommands = tick.Commands()
		gotScope = tick.Scope()
		gotLimits = tick.Limits()
	}})

	source := staticSource{
		commands: []sim.Command{walk{}},
		limits:   sim.Limits{Events: 2},
		scope:    sim.Scope{Entities: []entity.ID{7}},
	}
	if _, err := Drive(context.Background(), kernel, store, source, sink); err != nil {
		t.Fatalf("Drive: %v", err)
	}

	if len(gotCommands) != 1 || gotCommands[0].CommandKind() != "movement.walk" {
		t.Fatalf("the tick saw commands %+v, want one walk", gotCommands)
	}
	if len(gotScope.Entities) != 1 || gotScope.Entities[0] != 7 {
		t.Fatalf("the tick saw scope %+v, want entity 7", gotScope)
	}
	if gotLimits.Events != 2 {
		t.Fatalf("the tick saw an event budget of %d, want 2", gotLimits.Events)
	}
}

func TestDriveNamesThePieceItWasNotGiven(t *testing.T) {
	kernel, store, sink := harness(t)

	if _, err := Drive(context.Background(), nil, store, staticSource{}, sink); err == nil {
		t.Error("Drive ran without a kernel")
	}
	if _, err := Drive(context.Background(), kernel, nil, staticSource{}, sink); err == nil {
		t.Error("Drive ran without a store")
	}
	if _, err := Drive(context.Background(), kernel, store, nil, sink); err == nil {
		t.Error("Drive ran without a source")
	}
	if _, err := Drive(context.Background(), kernel, store, staticSource{}, nil); err == nil {
		t.Error("Drive ran without a sink")
	}
}
