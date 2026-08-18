package v1_8

import (
	"context"
	"fmt"

	"github.com/go-theft-craft/minecraft-simulation/entity"
	"github.com/go-theft-craft/minecraft-simulation/geom"
	"github.com/go-theft-craft/minecraft-simulation/movement"
	"github.com/go-theft-craft/minecraft-simulation/sim"
	"github.com/go-theft-craft/minecraft-simulation/world"
)

// sneakScale multiplies both input axes while a body is sneaking.
//
// The scale belongs to the client's movement input rather than to the entity
// tick — the 1.8.9 client scales what it sends, and a server never sees an
// unscaled axis — so it is folded into the input-decay phase here and applied
// before the per-tick decay, which is the order the two happen in.
//
// It is a double literal in the game, multiplied into a float axis and narrowed
// back, so the scaling is written that way below rather than as a float product.
const sneakScale = 0.3

// motionThreshold is the magnitude below which the game discards a component of
// a body's motion. It is a double literal.
const motionThreshold = 0.005

// body is the per-tick working state for one entity.
//
// The phases carry their intermediate values here rather than writing them
// through the tick, so a tick that turns out to be incomplete leaves nothing
// behind: the kernel drops operations, and the commit phase is the only one that
// produces any.
type body struct {
	id    entity.ID
	state entity.State
	loco  movement.Locomotion

	strafe   float32
	forward  float32
	friction float32
	speed    float32

	collided bool
	landed   bool
	// present is false for an entity in scope that the store does not hold, or
	// holds without locomotion state. Later phases skip it.
	present bool
}

// scratch is one tick's working state, shared by the phases of one phase list.
//
// A phase list is stateful within a tick and belongs to one kernel: NewKernel
// takes the list once, and a kernel steps one tick at a time, so nothing here is
// ever touched by two ticks at once. Phases() returns a fresh list precisely so
// that two kernels built from one profile do not share this.
//
// The first phase resets it, so a tick that failed halfway cannot leak a value
// into the next one.
type scratch struct {
	bodies []body
}

// buildPhases returns the eleven phases of the 1.8.9 land tick, in order.
//
// The order is data. A custom profile may reorder or replace a phase, and M8.7
// builds a different order for 26.1.2 without touching a rule. That is also why
// friction and acceleration are separate phases even though one feeds the other:
// a profile that wants different friction should not have to reimplement
// acceleration to get it.
func (p *profile) buildPhases() []sim.Phase {
	shared := &scratch{}

	return []sim.Phase{
		phase{id: "v1_8.jump-countdown", run: func(tick *sim.TickState) error {
			return p.adoptInput(tick, shared)
		}},
		phase{id: "v1_8.motion-threshold", run: func(*sim.TickState) error {
			return p.motionThreshold(shared)
		}},
		phase{id: "v1_8.jump", run: func(*sim.TickState) error {
			return p.jump(shared)
		}},
		phase{id: "v1_8.input-decay", run: func(*sim.TickState) error {
			return p.decayInput(shared)
		}},
		phase{id: "v1_8.friction", run: func(tick *sim.TickState) error {
			return p.friction(tick, shared)
		}},
		phase{id: "v1_8.acceleration", run: func(*sim.TickState) error {
			return p.acceleration(shared)
		}},
		phase{id: "v1_8.apply-input", run: func(*sim.TickState) error {
			return p.applyInput(shared)
		}},
		// A dropped item falls before it moves, where a player falls after.
		// One ordered list holds both because each phase skips the families it
		// is not about.
		phase{id: "v1_8.item-gravity", run: func(*sim.TickState) error {
			return p.itemGravity(shared)
		}},
		phase{id: "v1_8.move", run: func(tick *sim.TickState) error {
			return p.move(tick, shared)
		}},
		phase{id: "v1_8.gravity", run: func(*sim.TickState) error {
			return p.gravity(shared)
		}},
		phase{id: "v1_8.vertical-drag", run: func(*sim.TickState) error {
			return p.verticalDrag(shared)
		}},
		phase{id: "v1_8.horizontal-drag", run: func(*sim.TickState) error {
			return p.horizontalDrag(shared)
		}},
		phase{id: "v1_8.item-drag", run: func(tick *sim.TickState) error {
			return p.itemDrag(tick, shared)
		}},
		phase{id: "v1_8.item-bounce", run: func(*sim.TickState) error {
			return p.itemBounce(shared)
		}},
		phase{id: "v1_8.arrow-stick", run: func(*sim.TickState) error {
			return p.arrowStick(shared)
		}},
		phase{id: "v1_8.arrow-inertia", run: func(*sim.TickState) error {
			return p.arrowInertia(shared)
		}},
		phase{id: "v1_8.arrow-gravity", run: func(*sim.TickState) error {
			return p.arrowGravity(shared)
		}},
		phase{id: "v1_8.commit", run: func(tick *sim.TickState) error {
			return p.commit(tick, shared)
		}},
	}
}

// phase adapts a function to sim.Phase.
type phase struct {
	id  string
	run func(*sim.TickState) error
}

// ID implements sim.Phase.
func (p phase) ID() string { return p.id }

// Run implements sim.Phase.
func (p phase) Run(_ context.Context, tick *sim.TickState) error { return p.run(tick) }

// adoptInput starts the tick: it gathers the bodies in scope, decrements each
// jump counter, and adopts this tick's input flags and facing.
//
// The flags are adopted at the top of the tick because everything after this
// reads them: the game sets them on the entity before its movement runs, and a
// phase that adopted them later would be applying last tick's facing to this
// tick's motion.
func (p *profile) adoptInput(tick *sim.TickState, shared *scratch) error {
	shared.bodies = shared.bodies[:0]

	inputs := latestInputs(tick.Commands())
	for _, id := range tick.Scope().Entities {
		working := body{id: id}

		state, ok := tick.Entity(id)
		if !ok {
			shared.bodies = append(shared.bodies, working)

			continue
		}
		loco, ok := tick.Locomotion(id)
		if !ok && state.Family == entity.FamilyPlayer {
			// A player with no locomotion state is a body this tick cannot
			// move: the jump counter, the facing, and the speed all live
			// there. An item or an arrow has none of those by nature, so it is
			// ticked with the zero value rather than skipped.
			shared.bodies = append(shared.bodies, working)

			continue
		}

		// A family with no constants is refused here rather than at the phase
		// that first needs a number, because the phases are guarded by family
		// and an unknown one would otherwise fall through all of them and
		// commit unmoved.
		if _, err := p.constantsFor(&body{id: id, state: state}); err != nil {
			return err
		}

		working.state = state
		working.loco = movement.Countdown(loco)
		working.present = true

		if input, ok := inputs[id]; ok {
			working.loco.Jumping = input.Jump
			working.loco.Sprinting = input.Sprint
			working.loco.Sneaking = input.Sneak
			working.loco.Yaw = input.Yaw
			working.loco.Pitch = input.Pitch
			working.strafe = input.Strafe
			working.forward = input.Forward
		}

		shared.bodies = append(shared.bodies, working)
	}

	return nil
}

// latestInputs indexes the tick's movement intents by body, keeping the last one
// for each. A controller that sent two intents for one tick meant the second.
func latestInputs(commands []sim.Command) map[entity.ID]movement.Input {
	inputs := make(map[entity.ID]movement.Input, len(commands))
	for _, command := range commands {
		if input, ok := command.(movement.Input); ok {
			inputs[input.Entity] = input
		}
	}

	return inputs
}

// motionThreshold discards each component of a body's motion that is too small
// to matter.
//
// It runs after the countdown and before the jump, which is where the game runs
// it. The oracle found it: the tick this milestone was planned from did not
// describe it, and without it a body walking at any angle other than square on
// disagrees with the game within a handful of ticks.
func (p *profile) motionThreshold(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		working.state.Motion = movement.ClampSmallMotion(working.state.Motion, motionThreshold)
	}

	return nil
}

// jump applies the jump impulse to every body whose state permits one.
func (p *profile) jump(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		loco, motion, _ := movement.Jump(
			p.table, working.loco, working.state.Motion, working.state.OnGround, jumpUpwards,
		)
		working.loco = loco
		working.state.Motion = motion
	}

	return nil
}

// decayInput scales the input axes: the sneak factor first, then the per-tick
// decay the game applies to both axes before they are used.
func (p *profile) decayInput(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		if working.loco.Sneaking {
			working.strafe = float32(float64(working.strafe) * sneakScale)
			working.forward = float32(float64(working.forward) * sneakScale)
		}
		working.strafe *= inputDecay
		working.forward *= inputDecay
	}

	return nil
}

// friction computes each body's horizontal multiplier from the block beneath it,
// before the body moves.
//
// Reading it here rather than in the drag phase is what makes a player who walks
// off ice keep ice friction for the tick that leaves it. The move phase runs
// between the two and cannot change what this recorded.
func (p *profile) friction(tick *sim.TickState, shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		slipperiness := p.slipperiness(0)
		if working.state.OnGround {
			below := movement.GroundFrictionBlock(working.state.Box, position(working.state.Box))
			ref, lookup := tick.BlockState(below)
			if lookup == world.LookupUnknown {
				// The tick is already incomplete; the kernel drops its work.
				continue
			}
			slipperiness = p.slipperiness(ref)
		}
		working.friction = movement.Friction(slipperiness, working.state.OnGround)
	}

	return nil
}

// acceleration turns the friction into this tick's input scale.
func (p *profile) acceleration(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		working.speed = movement.Speed(
			working.friction, working.state.OnGround, working.loco.MoveSpeed, working.loco.JumpFactor,
		)
	}

	return nil
}

// applyInput adds the scaled input to each body's motion.
func (p *profile) applyInput(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		working.state.Motion = movement.ApplyHeading(
			p.table, working.state.Motion,
			working.strafe, working.forward, working.speed, working.loco.Yaw,
		)
	}

	return nil
}

// move resolves each body's motion against the world.
//
// A clamped axis zeroes that component of the motion, which is what stops a body
// pressed against a wall from accumulating speed into it. The vertical clamp is
// also where standing comes from: a body whose downward motion was stopped is on
// the ground.
func (p *profile) move(tick *sim.TickState, shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		result, err := movement.Step(tick.Blocks(), working.state, tick.Limits().CollisionCandidates)
		if err != nil {
			return fmt.Errorf("resolving entity %d: %w", working.id, err)
		}
		if len(result.Unknown) != 0 {
			// collision reports incompleteness in its own return value, so the
			// phase is what tells the kernel. Without this the tick would report
			// itself complete over a region nobody described.
			tick.MissingBlocks(result.Unknown)

			continue
		}

		wasAirborne := !working.state.OnGround
		working.state.Box = result.Body
		working.state.OnGround = result.OnGround
		working.collided = result.CollidedHorizontally()
		working.landed = wasAirborne && result.OnGround

		if result.CollidedX {
			working.state.Motion.X = 0
		}
		if result.CollidedY {
			working.state.Motion.Y = 0
		}
		if result.CollidedZ {
			working.state.Motion.Z = 0
		}
	}

	return nil
}

// gravity applies one tick of fall to every body, at its own family's rate.
func (p *profile) gravity(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		working.state.Motion = movement.ApplyGravity(working.state.Motion, constants.Gravity)
	}

	return nil
}

// verticalDrag applies the vertical multiplier.
func (p *profile) verticalDrag(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		working.state.Motion = movement.ApplyVerticalDrag(
			working.state.Motion, float32(constants.VerticalDrag),
		)
	}

	return nil
}

// horizontalDrag applies the friction this tick's friction phase recorded.
func (p *profile) horizontalDrag(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		if working.state.Family != entity.FamilyPlayer {
			continue
		}

		working.state.Motion = movement.ApplyHorizontalDrag(working.state.Motion, working.friction)
	}

	return nil
}

// commit writes the tick's results and reports what happened.
//
// It is the only phase that produces operations. Everything before it worked on
// the scratch, so an incomplete tick had nothing to drop.
func (p *profile) commit(tick *sim.TickState, shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present {
			continue
		}

		tick.SetEntity(working.id, working.state)
		tick.SetLocomotion(working.id, working.loco)

		if working.collided {
			tick.EmitDomain(sim.DomainEvent{Kind: "movement.collided", Entity: working.id})
		}
		if working.landed {
			tick.EmitDomain(sim.DomainEvent{Kind: "movement.landed", Entity: working.id})
		}
	}

	return nil
}

// position returns the point the game treats as a body's location: the centre of
// its box horizontally, and its feet vertically.
func position(box geom.AABB) geom.Vec3 {
	return geom.Vec3{
		X: (box.MinX + box.MaxX) / 2,
		Y: box.MinY,
		Z: (box.MinZ + box.MaxZ) / 2,
	}
}

// constantsFor returns the body's own motion constants.
//
// Every phase that needs a number reads it through here rather than naming a
// family. Until M9.2 they all named entity.FamilyPlayer outright, so a dropped
// item in the world fell at the player's gravity and nothing said so.
func (p *profile) constantsFor(working *body) (sim.MotionConstants, error) {
	constants := p.Motion(working.state.Family)
	if constants == (sim.MotionConstants{}) {
		return sim.MotionConstants{}, fmt.Errorf("%w: entity %d is a %s",
			sim.ErrUnknownFamily, working.id, working.state.Family)
	}

	return constants, nil
}

// itemGravity applies a dropped item's fall, before it moves.
//
// EntityItem.onUpdate subtracts its gravity at the top of the tick and a player
// subtracts after the move, so the two families cannot share a phase however
// alike the arithmetic looks.
func (p *profile) itemGravity(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present || working.state.Family != entity.FamilyItem {
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		working.state.Motion = movement.ApplyGravity(working.state.Motion, constants.Gravity)
	}

	return nil
}

// itemDrag applies an item's two multipliers after it has moved.
//
// The block below is read here rather than before the move, which is the
// opposite of the player's friction phase and is what the game does: an item
// takes the friction of the block it ended the tick on. Both multipliers are
// 0.98F in this version, and the horizontal one is the block's slipperiness
// times it, formed at single width.
func (p *profile) itemDrag(tick *sim.TickState, shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present || working.state.Family != entity.FamilyItem {
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		slipperiness := p.slipperiness(0)
		if working.state.OnGround {
			below := movement.GroundFrictionBlock(working.state.Box, position(working.state.Box))
			ref, lookup := tick.BlockState(below)
			if lookup == world.LookupUnknown {
				// The tick is already incomplete; the kernel drops its work.
				continue
			}
			slipperiness = p.slipperiness(ref)
		}

		friction := movement.FrictionWith(
			slipperiness, working.state.OnGround, float32(constants.HorizontalDrag),
		)
		working.state.Motion = movement.ApplyHorizontalDrag(working.state.Motion, friction)
		working.state.Motion = movement.ApplyVerticalDrag(
			working.state.Motion, float32(constants.VerticalDrag),
		)
	}

	return nil
}

// itemBounce is the half-height hop an item takes off the ground.
//
// 1.8.9 applies it whenever the item is on the ground, with no test of the
// direction, so an item that is on the ground with upward motion has that
// motion halved and inverted too. 26.1.2 added the test; see that profile.
func (p *profile) itemBounce(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present || working.state.Family != entity.FamilyItem {
			continue
		}
		if !working.state.OnGround {
			continue
		}

		working.state.Motion.Y *= itemBounceFactor
	}

	return nil
}

// itemBounceFactor is the multiplier an item's vertical motion takes on the
// ground. It is a double literal in EntityItem.onUpdate.
const itemBounceFactor = -0.5

// arrowInertia applies an arrow's single multiplier to all three axes.
//
// An arrow has no friction from the block below it: the constant is the same
// whatever it is standing on, and it is applied after the move, which is where
// this phase sits.
func (p *profile) arrowInertia(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present || working.state.Family != entity.FamilyArrow {
			continue
		}
		if working.state.OnGround {
			// An arrow in the ground does no motion at all: the tick takes the
			// branch that ticks despawn and nothing else. See arrowStick.
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		inertia := float32(constants.HorizontalDrag)
		working.state.Motion = movement.ApplyHorizontalDrag(working.state.Motion, inertia)
		working.state.Motion = movement.ApplyVerticalDrag(working.state.Motion, inertia)
	}

	return nil
}

// arrowGravity applies an arrow's fall, after its inertia.
func (p *profile) arrowGravity(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present || working.state.Family != entity.FamilyArrow {
			continue
		}
		if working.state.OnGround {
			// An arrow in the ground does no motion at all: the tick takes the
			// branch that ticks despawn and nothing else. See arrowStick.
			continue
		}

		constants, err := p.constantsFor(working)
		if err != nil {
			return err
		}

		working.state.Motion = movement.ApplyGravity(working.state.Motion, constants.Gravity)
	}

	return nil
}

// arrowStick stops an arrow that has hit the ground.
//
// The game's arrow does not move at all once it is in the ground: its tick
// takes a branch that counts down a despawn and does nothing else, which is why
// a capture of a landed arrow is a long run of zero deltas. What this cannot
// reproduce is sticking into a wall: the game stops an arrow at the point a ray
// cast hit, and this module moves a box by sweeping it, so an arrow that hits a
// vertical face here slides to rest against it over a tick or two instead of
// stopping in it. Both agree once the arrow is down.
func (p *profile) arrowStick(shared *scratch) error {
	for index := range shared.bodies {
		working := &shared.bodies[index]
		if !working.present || working.state.Family != entity.FamilyArrow {
			continue
		}
		if !working.state.OnGround {
			continue
		}

		working.state.Motion = geom.Vec3{}
	}

	return nil
}
