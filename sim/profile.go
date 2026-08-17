package sim

import (
	"context"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// MotionConstants are the per-family movement constants a profile applies.
//
// The fields mirror the generated dataset field for field rather than being
// reshaped into something tidier. A quantity's float width is part of its value:
// Java Edition stores a player's step height as a float and widens it where it
// is applied, so the value is 0.6000000238418579 and not 0.6. Every reshaping
// between the dataset and the rule is a chance to round that back.
type MotionConstants struct {
	// Gravity is the downward acceleration applied per tick.
	Gravity float64
	// HorizontalDrag multiplies horizontal motion each tick.
	HorizontalDrag float64
	// VerticalDrag multiplies vertical motion each tick.
	VerticalDrag float64
	// StepHeight is how far a body of this family may rise to clear an obstacle.
	StepHeight float64
}

// Profile supplies the rules one version of the game plays by.
//
// A profile names no protocol type and encodes no packet. It answers questions
// about block handles and entity families, and it supplies the phases a tick
// runs. Everything version-specific in the simulation lives behind this
// interface.
type Profile interface {
	// ID names the rules, and separates one profile's digests from another's.
	ID() ProfileID
	// Slipperiness reports the friction of the block underfoot.
	Slipperiness(ref world.BlockRef) float64
	// Motion returns the movement constants for a family.
	Motion(family entity.Family) MotionConstants
	// Shape resolves a block handle to its collision shape. It reports false
	// for a handle this profile did not mint, which is what lets a store refuse
	// a change set naming a block it cannot resolve.
	Shape(ref world.BlockRef) (geom.Shape, bool)
	// Phases returns the tick phases, in the order they run. The order is part
	// of the rules: two profiles with the same phases in a different order are
	// different profiles.
	Phases() []Phase
}

// BlockNames is a profile that can resolve a block's name to the handle it
// minted for it.
//
// It is separate from Profile because resolving a name is not something a tick
// ever does: handles are opaque by design, and a rule that looked one up by name
// would be naming a version fact from a version-neutral package. What needs it
// is everything that arrives from outside the simulation — a fixture, a world
// loader, a test — because a handle means nothing outside the profile that
// minted it, and a name survives the table being renumbered.
//
// A profile that does not implement this cannot be handed a world described by
// name. That is a reportable condition rather than a silent default: the caller
// is holding a description nothing can resolve.
type BlockNames interface {
	// Ref returns the handle a name resolves to, or false when the profile does
	// not know the name.
	Ref(name string) (world.BlockRef, bool)
}

// Phase is one stage of a tick.
//
// A phase reads and writes only through the TickState it is handed. Phases run
// in order on one goroutine, so a phase may rely on every earlier phase in the
// same tick having finished.
type Phase interface {
	// ID names the phase. Identifiers are unique within a profile.
	ID() string
	// Run performs the phase. Returning an error aborts the tick, and the
	// result carries no applicable change set.
	Run(ctx context.Context, tick *TickState) error
}

// TickState is the only thing a phase touches.
//
// Reads record what they consulted, writes append operations against their
// budgets, and emitters count against the event budget. A recorder that runs out
// of budget stores ErrLimitExhausted and the kernel reports it, so a phase that
// ignores a failed write cannot smuggle work past a limit.
//
// TickState is not safe for concurrent use. Phases run on one goroutine.
type TickState struct {
	profile    Profile
	tick       Tick
	limits     Limits
	scope      Scope
	blocks     world.View
	entities   entity.View
	locomotion movement.LocomotionView
	commands   []Command

	ops          []Op
	domain       []DomainEvent
	presentation []PresentationEvent
	outcomes     []CommandOutcome
	read         []Dependency
	missing      []Dependency
	random       RandomState

	entitySteps  int
	blockUpdates int
	events       int

	err error
}

// Profile returns the rules this tick runs under.
func (t *TickState) Profile() Profile { return t.profile }

// Tick returns the tick number being simulated.
func (t *TickState) Tick() Tick { return t.tick }

// Limits returns the effective budgets, with defaults filled in.
func (t *TickState) Limits() Limits { return t.limits }

// Scope returns what this tick simulates.
func (t *TickState) Scope() Scope { return t.scope }

// Random returns the random state the tick draws from.
func (t *TickState) Random() RandomState { return t.random }

// SetRandom records the random state the tick leaves behind.
func (t *TickState) SetRandom(state RandomState) { t.random = state }

// Blocks returns the block view for handing straight to collision.Resolve.
//
// A phase that uses this must forward collision.Result.Unknown to MissingBlocks.
// The collision package reports incompleteness in its own return value, and the
// kernel cannot see inside it: a phase that swept unknown cells and said nothing
// would produce a complete result built on state nobody described.
func (t *TickState) Blocks() world.View { return t.blocks }

// Locomotion returns a body's movement state and records the dependency.
//
// It reports false both when the view holds no state for the body and when the
// tick was given no locomotion view at all. A phase that needs one treats the
// absence as a body it does not simulate, not as a zero state.
func (t *TickState) Locomotion(id entity.ID) (movement.Locomotion, bool) {
	t.read = append(t.read, Dependency{Kind: DependencyEntity, Entity: id})
	if t.locomotion == nil {
		return movement.Locomotion{}, false
	}

	return t.locomotion.Locomotion(id)
}

// SetLocomotion writes a body's movement state.
//
// It spends the same budget a body write does. A tick that writes both for one
// entity spends two, which is what the budget is for: it bounds the work, not
// the entities.
func (t *TickState) SetLocomotion(id entity.ID, state movement.Locomotion) {
	if !t.spend(&t.entitySteps, t.limits.EntitySteps) {
		return
	}
	t.ops = append(t.ops, Op{Kind: OpSetLocomotion, Entity: id, Locomotion: state})
}

// Entity returns a body and records the dependency.
//
// A body the view does not hold is reported as absent rather than as missing
// data: "there is no such entity" is an answer, and a phase that wanted one is
// the thing that has to decide what to do about it.
func (t *TickState) Entity(id entity.ID) (entity.State, bool) {
	t.read = append(t.read, Dependency{Kind: DependencyEntity, Entity: id})

	return t.entities.Entity(id)
}

// Commands returns the intents this tick was asked to apply, in order.
//
// A phase answers each one it handles with RecordOutcome, whose Index is the
// command's position in this slice.
func (t *TickState) Commands() []Command { return t.commands }

// BlockShape returns a cell's collision shape, records the dependency, and marks
// the tick incomplete when nobody has described the cell.
func (t *TickState) BlockShape(pos geom.BlockPos) (geom.Shape, world.Lookup) {
	t.readBlock(pos)
	shape, lookup := t.blocks.CollisionShape(pos)
	if lookup == world.LookupUnknown {
		t.missing = append(t.missing, Dependency{Kind: DependencyBlock, Block: pos})
	}

	return shape, lookup
}

// BlockState returns a cell's block handle, records the dependency, and marks
// the tick incomplete when nobody has described the cell.
func (t *TickState) BlockState(pos geom.BlockPos) (world.BlockRef, world.Lookup) {
	t.readBlock(pos)
	ref, lookup := t.blocks.BlockState(pos)
	if lookup == world.LookupUnknown {
		t.missing = append(t.missing, Dependency{Kind: DependencyBlock, Block: pos})
	}

	return ref, lookup
}

// MissingBlocks declares cells the tick needed and could not read. A phase that
// resolves collision itself reports its unknown cells this way.
func (t *TickState) MissingBlocks(positions []geom.BlockPos) {
	for _, pos := range positions {
		t.missing = append(t.missing, Dependency{Kind: DependencyBlock, Block: pos})
	}
}

// SetEntity writes a body.
func (t *TickState) SetEntity(id entity.ID, state entity.State) {
	if !t.spend(&t.entitySteps, t.limits.EntitySteps) {
		return
	}
	t.ops = append(t.ops, Op{Kind: OpSetEntity, Entity: id, State: state})
}

// RemoveEntity drops a body.
func (t *TickState) RemoveEntity(id entity.ID) {
	if !t.spend(&t.entitySteps, t.limits.EntitySteps) {
		return
	}
	t.ops = append(t.ops, Op{Kind: OpRemoveEntity, Entity: id})
}

// SetBlock writes a block state.
func (t *TickState) SetBlock(pos geom.BlockPos, ref world.BlockRef) {
	if !t.spend(&t.blockUpdates, t.limits.BlockUpdates) {
		return
	}
	t.ops = append(t.ops, Op{Kind: OpSetBlock, Block: pos, Ref: ref})
}

// EmitDomain reports a simulation fact.
func (t *TickState) EmitDomain(event DomainEvent) {
	if !t.spend(&t.events, t.limits.Events) {
		return
	}
	t.domain = append(t.domain, event)
}

// EmitPresentation asks for a particle, a sound, or an animation.
func (t *TickState) EmitPresentation(event PresentationEvent) {
	if !t.spend(&t.events, t.limits.Events) {
		return
	}
	t.presentation = append(t.presentation, event)
}

// RecordOutcome says what the tick did with one command.
func (t *TickState) RecordOutcome(outcome CommandOutcome) {
	t.outcomes = append(t.outcomes, outcome)
}

// readBlock records a block dependency.
func (t *TickState) readBlock(pos geom.BlockPos) {
	t.read = append(t.read, Dependency{Kind: DependencyBlock, Block: pos})
}

// spend charges one unit against a budget, and stores ErrLimitExhausted when
// there is none left.
func (t *TickState) spend(counter *int, budget int) bool {
	if *counter >= budget {
		if t.err == nil {
			t.err = ErrLimitExhausted
		}

		return false
	}
	*counter++

	return true
}
