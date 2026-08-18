package placement

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-theft-craft/minecraft-protocol/data"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/mining"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// Place asks the kernel to place the held item against a face.
//
// Several fields are the caller's claim rather than the kernel's knowledge, the
// way mining.Dig's are: the tick models no inventory, so what is held is said;
// and it holds a body's box but not the version's eye height, so where the
// player is looking from is said too. A field the kernel could derive is not
// asked for — the face, the cursor, and the clicked cell are the click itself.
type Place struct {
	Entity entity.ID
	// Clicked is the cell the cursor was on.
	Clicked geom.BlockPos
	// Face is the side of that cell the click landed on.
	Face mining.Face
	// Cursor is where within the clicked face the click landed, in the range
	// [0,1] per axis. Slabs and stairs read it; most blocks do not.
	Cursor geom.Vec3
	// Held is the item in the hand. The tick models no inventory — that is
	// M9.7's — so the caller says what it is holding.
	Held data.ItemID
	// Yaw is where the player is looking, which is what an orientable block
	// takes its facing from.
	Yaw float32
	// Eye is the point the reach is measured from, and Reach is how far it
	// stretches. A Reach of zero or less is a caller not asking for the check
	// rather than a caller with no reach: the distance a server allows is a
	// server setting, and the kernel does not hold one.
	Eye   geom.Vec3
	Reach float64
}

// CommandKind implements sim.Command.
func (Place) CommandKind() string { return "placement.place" }

// EventPlaced is the domain event an accepted placement emits.
const EventPlaced = "placement.placed"

// phaseID names the placement phase.
const phaseID = "placement.place"

// ErrNotPlaceable reports a profile that cannot say what a placement produces.
var ErrNotPlaceable = errors.New("placement: this profile does not place blocks")

// Replaceables is a profile that can say which blocks a placement replaces.
//
// It is optional and it is separate from Placer because no version's generated
// data publishes the property today. Air is replaceable in both editions and
// the tri-state view answers that much on its own; water, lava, tall grass,
// snow layers, and fire are replaceable too, and nothing in the dataset says
// so — the flag is a fact about the block's material that upstream does not
// carry.
//
// A profile that cannot answer therefore places against tall grass rather than
// into it, which is one cell too high. Closing it means measuring the property
// out of the pinned jars into the dataset, the way falling and climbable were
// measured in minecraft-protocol on 2026-08-18, rather than typing a list of
// block names into a profile.
type Replaceables interface {
	// Replaceable reports whether the block a handle names is placed into.
	Replaceable(ref world.BlockRef) bool
}

// Phase returns the kernel phase that applies placement commands.
//
// It holds no state. Whether a placement is legal is a function of the world,
// the bodies, and the command, so two kernels stepped with the same input agree
// — which is what the digest is for.
func Phase() sim.Phase { return placePhase{} }

type placePhase struct{}

// ID implements sim.Phase.
func (placePhase) ID() string { return phaseID }

// Run implements sim.Phase.
func (placePhase) Run(_ context.Context, tick *sim.TickState) error {
	for index, command := range tick.Commands() {
		place, ok := command.(Place)
		if !ok {
			continue
		}

		outcome, placed, ref := apply(tick, place)
		outcome.Index, outcome.Kind = index, place.CommandKind()
		tick.RecordOutcome(outcome)

		if !outcome.Accepted {
			continue
		}

		// The write happens here rather than in apply, so that a refusal
		// cannot reach it: a refused placement that still wrote a block would
		// put a block in the world the server does not have, and the client
		// would draw it until the next chunk update took it away.
		tick.SetBlock(placed, ref)
		tick.EmitDomain(sim.DomainEvent{
			Kind: EventPlaced, Entity: place.Entity, Block: placed,
		})
	}

	return nil
}

// apply answers one placement, and reports where it landed and as what.
func apply(tick *sim.TickState, place Place) (sim.CommandOutcome, geom.BlockPos, world.BlockRef) {
	placer, ok := tick.Profile().(Placer)
	if !ok {
		return rejected(ErrNotPlaceable.Error()), geom.BlockPos{}, 0
	}

	replaceable := replaceableOf(tick.Profile())

	clicked, lookup := tick.BlockState(place.Clicked)
	if lookup == world.LookupUnknown {
		// Not a rejection, for the reason a dig against an unknown block is
		// not one: the tick could not be computed, BlockState has already told
		// the result what was missing, and rejecting would tell the caller to
		// stop rather than to load the chunk.
		return rejected("the block at " + position(place.Clicked) +
			" has not been described"), geom.BlockPos{}, 0
	}

	target := Resolve(place.Clicked, place.Face,
		lookup == world.LookupAir || replaces(replaceable, clicked))

	ref, err := placer.PlacedState(place.Held, target, place.Face, place.Yaw, place.Cursor)
	if err != nil {
		return rejected(err.Error()), geom.BlockPos{}, 0
	}

	shape, ok := tick.Profile().Shape(ref)
	if !ok {
		return rejected(fmt.Sprintf("the profile minted handle %d and cannot shape it", ref)),
			geom.BlockPos{}, 0
	}

	legality, complete := Check(
		tick.Blocks(), bodies{tick}, target, replaceable, shape, place.Eye, place.Reach,
	)
	if !complete.Complete {
		tick.MissingBlocks(missingBlocks(complete))

		return rejected("the block at " + position(target.Placed) +
			" has not been described"), geom.BlockPos{}, 0
	}
	if !legality.Allowed {
		return rejected(legality.Reason), geom.BlockPos{}, 0
	}

	return sim.CommandOutcome{Accepted: true}, target.Placed, ref
}

// bodies adapts a tick to the entity view the legality check reads.
//
// The tick holds no list of every body — a phase asks for one at a time — and
// the check walks them, so the adapter is where the tick's own scope decides
// which bodies exist. What it is not is a second entity model: every answer
// comes from the tick.
type bodies struct{ tick *sim.TickState }

// Entity implements entity.View.
func (b bodies) Entity(id entity.ID) (entity.State, bool) { return b.tick.Entity(id) }

// IDs implements entity.View.
func (b bodies) IDs() []entity.ID { return b.tick.Scope().Entities }

// replaceableOf returns the profile's replaceability predicate, or nil.
func replaceableOf(profile sim.Profile) Replaceable {
	owner, ok := profile.(Replaceables)
	if !ok {
		return nil
	}

	return owner.Replaceable
}

// missingBlocks names the cells an incomplete decision needed.
func missingBlocks(complete sim.Completeness) []geom.BlockPos {
	positions := make([]geom.BlockPos, 0, len(complete.Missing))
	for _, dependency := range complete.Missing {
		if dependency.Kind == sim.DependencyBlock {
			positions = append(positions, dependency.Block)
		}
	}

	return positions
}

// rejected builds a refusal that names its reason.
func rejected(reason string) sim.CommandOutcome {
	return sim.CommandOutcome{Accepted: false, Reason: reason}
}

// position renders a block position for a reason string.
func position(pos geom.BlockPos) string {
	return fmt.Sprintf("(%d, %d, %d)", pos.X, pos.Y, pos.Z)
}
