package mining

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Face names the side of a block being hit.
//
// It changes nothing about the time and everything about what a server accepts,
// so it travels with the command rather than being reconstructed at the wire.
// The numbering is the wire's, which both protocols have shared since 1.8.
type Face uint8

const (
	// FaceBottom is the -Y side.
	FaceBottom Face = 0
	// FaceTop is the +Y side.
	FaceTop Face = 1
	// FaceNorth is the -Z side.
	FaceNorth Face = 2
	// FaceSouth is the +Z side.
	FaceSouth Face = 3
	// FaceWest is the -X side.
	FaceWest Face = 4
	// FaceEast is the +X side.
	FaceEast Face = 5
)

// Dig asks the kernel to make progress on breaking one block.
//
// It is a command rather than a state so that a dig which is interrupted — by a
// correction, by the block changing under the player, by the player looking
// away — stops making progress without anything having to cancel it.
//
// Elapsed is the count of consecutive earlier ticks spent on this block, and it
// is the caller's rather than the kernel's. That is where the game keeps it: a
// vanilla client holds curBlockDamageMP on the controller doing the digging and
// zeroes it the moment the button comes up, and the world knows nothing about a
// break in progress. A kernel that held it would be a kernel with state, and
// interrupting a dig would then need a command to cancel it rather than simply
// not sending one.
type Dig struct {
	Entity entity.ID
	Block  geom.BlockPos
	Face   Face
	// Held is the item in the hand and its efficiency level. The tick models no
	// inventory — that is M9.7's — so the caller says what it is holding.
	Held Held
	// Effects are the haste and mining fatigue on the digger.
	Effects Effects
	// Underwater is the digger's head being in water without aqua affinity.
	// The tick models no fluids, so this too is the caller's claim.
	//
	// Airborne is not here: it is the body's own OnGround, which the tick does
	// hold, and asking the caller for something the kernel already knows is how
	// the two come to disagree.
	Underwater bool
	// Elapsed is how many consecutive ticks this dig has already run for. A
	// resumed dig starts again from zero, because vanilla resets rather than
	// pausing — a client that paused would break a block in several sittings,
	// which is faster than the game allows and is the first thing an
	// anti-cheat flags.
	Elapsed int
}

// CommandKind implements sim.Command.
func (Dig) CommandKind() string { return "mining.dig" }

// EventBroke is the domain event a completed dig emits.
const EventBroke = "mining.broke"

// EventProgressed is the domain event a dig that did not finish emits. It says
// the tick counted, which is what tells a consumer apart from a dig that was
// rejected and one that was never seen.
const EventProgressed = "mining.progressed"

// airBlock is the name every version gives the block a broken one leaves behind.
const airBlock = "air"

// phaseID names the dig phase.
const phaseID = "mining.dig"

// ErrNotClassified reports a profile that cannot answer how a block breaks.
var ErrNotClassified = errors.New("mining: this profile does not classify blocks")

// Phase returns the kernel phase that applies dig commands.
//
// It computes rather than accumulates. Whether a block breaks on this tick is a
// function of the conditions and the elapsed count alone, so the phase needs no
// memory and two kernels stepped with the same input agree — which is what the
// digest is for.
func Phase() sim.Phase { return digPhase{} }

type digPhase struct{}

// ID implements sim.Phase.
func (digPhase) ID() string { return phaseID }

// Run implements sim.Phase.
func (digPhase) Run(_ context.Context, tick *sim.TickState) error {
	for index, command := range tick.Commands() {
		dig, ok := command.(Dig)
		if !ok {
			continue
		}

		outcome, broke := apply(tick, dig)
		outcome.Index, outcome.Kind = index, dig.CommandKind()
		tick.RecordOutcome(outcome)

		if !broke {
			continue
		}

		tick.EmitDomain(sim.DomainEvent{Kind: EventBroke, Entity: dig.Entity, Block: dig.Block})

		// The block goes to air, when the profile can name air. A profile that
		// cannot is reported rather than left to write a handle it did not
		// mint, and the event above still says the block broke.
		names, ok := tick.Profile().(sim.BlockNames)
		if !ok {
			continue
		}
		if air, ok := names.Ref(airBlock); ok {
			tick.SetBlock(dig.Block, air)
		}
	}

	return nil
}

// apply answers one dig, and reports whether the block broke on this tick.
func apply(tick *sim.TickState, dig Dig) (sim.CommandOutcome, bool) {
	classifier, ok := tick.Profile().(Classifier)
	if !ok {
		return rejected(fmt.Sprintf("%v", ErrNotClassified)), false
	}

	ref, lookup := tick.BlockState(dig.Block)
	if lookup == world.LookupUnknown {
		// Not a rejection. A block the caller has not received is not a block
		// that cannot be dug — it is a tick that could not be computed, and
		// BlockState has already told the result what was missing. Rejecting
		// would tell the caller to stop rather than to load the chunk.
		return rejected("the block at " + position(dig.Block) + " has not been described"), false
	}

	conditions, err := classifier.Conditions(
		ref, dig.Held, dig.Effects, dig.Underwater, airborne(tick, dig.Entity))
	if err != nil {
		return rejected(err.Error()), false
	}

	ticks, err := BreakTicks(classifier.Hardness(ref), conditions)
	if err != nil {
		// The outcome has to say why. A dig that silently makes no progress is
		// indistinguishable from a dig that is merely slow, and a caller
		// waiting on it waits forever.
		return rejected(err.Error()), false
	}

	// The elapsed count is of ticks already spent, so this one is the next.
	broke := dig.Elapsed+1 >= ticks
	if !broke {
		tick.EmitDomain(sim.DomainEvent{
			Kind: EventProgressed, Entity: dig.Entity, Block: dig.Block,
		})
	}

	return sim.CommandOutcome{Accepted: true}, broke
}

// airborne reports the digger not standing on anything.
//
// A body the tick does not hold is treated as standing. The alternative is to
// apply a fivefold penalty to a body nobody described, which is a wrong break
// time computed confidently.
func airborne(tick *sim.TickState, id entity.ID) bool {
	state, ok := tick.Entity(id)

	return ok && !state.OnGround
}

// rejected builds a refusal that names its reason.
func rejected(reason string) sim.CommandOutcome {
	return sim.CommandOutcome{Accepted: false, Reason: reason}
}

// position renders a block position for a reason string.
func position(pos geom.BlockPos) string {
	return fmt.Sprintf("(%d, %d, %d)", pos.X, pos.Y, pos.Z)
}
